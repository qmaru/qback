/*
服务端接收文件流程：

	1.接收客户端发送的 meta 数据，包含文件名(name)，文件大小(size)，文件分片数(chunks)，文件哈希值(hash)
	2.接收客户端的流数据
	3.校验哈希值

服务端预处理的事项：

	1.根据文件名和标签创建文件夹
	2.接收文件并计算进度
*/
package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"qback/grpc/common"
	pb "qback/grpc/libs"
	"qback/utils"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/peer"
)

type ServerBasic struct {
	ListenAddress string
	SavePath      string
	Secure        bool
	MemoryMode    bool
}

type FileService struct {
	savePath   string
	memoryMode bool
	pb.UnimplementedFileTransferServiceServer
}

// unaryLoggerMid
func unaryLoggerMid(ctx context.Context, info *grpc.UnaryServerInfo) error {
	var clientIP string
	pr, ok := peer.FromContext(ctx)
	if ok {
		clientIP = pr.Addr.String()
	} else {
		clientIP = ""
	}

	log.Printf("%s %s\n", clientIP, info.FullMethod)
	return nil
}

// streamLoggerMid
func streamLoggerMid(ctx context.Context, info *grpc.StreamServerInfo) error {
	var clientIP string
	pr, ok := peer.FromContext(ctx)
	if ok {
		clientIP = pr.Addr.String()
	} else {
		clientIP = ""
	}

	log.Printf("%s %s (stream)\n", clientIP, info.FullMethod)
	return nil
}

// unaryLogInterceptor
func unaryLogInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	// 输出日志
	err := unaryLoggerMid(ctx, info)
	if err != nil {
		return nil, err
	}
	// 继续处理请求
	return handler(ctx, req)
}

func streamLogInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	err := streamLoggerMid(ss.Context(), info)
	if err != nil {
		return err
	}
	return handler(srv, ss)
}

func (s *ServerBasic) Run() error {
	log.Printf("Listen on %s\n", s.ListenAddress)

	listener, err := net.Listen("tcp", s.ListenAddress)
	if err != nil {
		return err
	}

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(unaryLogInterceptor),
		grpc.StreamInterceptor(streamLogInterceptor),
		grpc.MaxRecvMsgSize(common.MaxMsgSize),
		grpc.MaxSendMsgSize(common.MaxMsgSize),
		grpc.ConnectionTimeout(10 * time.Second),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	if s.Secure {
		log.Println("TLS ON")
		tlsConfig, certPool, err := common.GenTLSInfo("server")
		if err != nil {
			return err
		}

		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		tlsConfig.ClientCAs = certPool

		tlsConfig.MinVersion = tls.VersionTLS12
		tlsConfig.MaxVersion = tls.VersionTLS13

		tlsConfig.Time = func() time.Time { return time.Now() }

		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.Creds(creds))
	}
	server := grpc.NewServer(opts...)
	pb.RegisterFileTransferServiceServer(server, &FileService{savePath: s.SavePath, memoryMode: s.MemoryMode})

	log.Println("Server is ready")
	if s.MemoryMode {
		log.Println("Memory Mode: ON")
	}
	err = server.Serve(listener)
	if err != nil {
		return err
	}
	return nil
}

func (s *FileService) sendUploadError(stream pb.FileTransferService_UploadFileServer, message string) error {
	return stream.Send(&pb.UploadResponse{
		Payload: &pb.UploadResponse_Result{
			Result: &pb.TransferResult{
				Status:  false,
				Message: message,
			},
		},
	})
}

func (s *FileService) sendUploadSuccess(stream pb.FileTransferService_UploadFileServer, message string) error {
	return stream.Send(&pb.UploadResponse{
		Payload: &pb.UploadResponse_Result{
			Result: &pb.TransferResult{
				Status:  true,
				Message: message,
			},
		},
	})
}

func (s *FileService) sendDownloadError(stream pb.FileTransferService_DownloadFileServer, message string) error {
	return stream.Send(&pb.DownloadResponse{
		Payload: &pb.DownloadResponse_Result{
			Result: &pb.TransferResult{
				Status:  false,
				Message: message,
			},
		},
	})
}

func (s *FileService) sendDownloadSuccess(stream pb.FileTransferService_DownloadFileServer, message string) error {
	return stream.Send(&pb.DownloadResponse{
		Payload: &pb.DownloadResponse_Result{
			Result: &pb.TransferResult{
				Status:  true,
				Message: message,
			},
		},
	})
}

func (s *FileService) ServerCheck(ctx context.Context, in *pb.Ping) (*pb.Pong, error) {
	log.Printf("[Ping] Received")
	if in.GetStatus() {
		return &pb.Pong{Status: true}, nil
	}
	return &pb.Pong{Status: false}, nil
}

func (s *FileService) UploadFile(stream pb.FileTransferService_UploadFileServer) error {
	req, err := stream.Recv()
	if err != nil {
		return err
	}

	metadata := req.GetMetadata()
	if metadata == nil {
		return s.sendUploadError(stream, "Missing metadata")
	}

	fileTag := metadata.GetTag()
	fileName := metadata.GetName()
	fileSize := metadata.GetSize()
	fileChunks := metadata.GetChunks()
	fileChunksize := metadata.GetChunksize()
	fileHash := metadata.GetHash()

	log.Printf("[Upload] Metadata: tag=%s, name=%s, size=%d, chunks=%d x %d Byte, hash=%s\n",
		fileTag, fileName, fileSize, fileChunks, fileChunksize, fileHash)

	if !s.memoryMode {
		ok, err := common.FileIsExist(s.savePath, fileTag, fileName, fileHash)
		if err != nil {
			log.Printf("[Upload] File error: %s \n", err.Error())
			return s.sendUploadError(stream, "File not found")
		}
		if ok {
			return s.sendUploadError(stream, "File already exists")
		}
	}

	if err := stream.Send(&pb.UploadResponse{
		Payload: &pb.UploadResponse_MetaAck{
			MetaAck: &pb.MetaAck{
				AllowUpload: true,
				Message:     "Ready to receive",
			},
		},
	}); err != nil {
		return err
	}

	var bufWriter *bufio.Writer
	var recFile *os.File
	var recFilePath string
	var memBuf []byte
	startTime := time.Now()

	if s.memoryMode {
		memBuf = make([]byte, 0, fileSize)
	} else {
		dstFilePath, err := common.SetTargetFilePath(s.savePath, fileTag, fileName)
		if err != nil {
			log.Printf("[Upload] Create file path error: %s \n", err.Error())
			return s.sendUploadError(stream, "File upload path unavailable")
		}

		recFile, err = common.OpenTargetFile(dstFilePath, common.FileWrite)
		if err != nil {
			log.Printf("[Upload] Create file error: %s \n", err.Error())
			return s.sendUploadError(stream, "Failed to access destination file")
		}
		defer recFile.Close()

		recFilePath = dstFilePath
		bufWriter = bufio.NewWriterSize(recFile, 64*1024)
	}

	log.Println("[Upload] Start receiving data")
	for {
		req, err := stream.Recv()

		if err == io.EOF {
			log.Println("[Upload] Reached EOF, processing final validation")
			break
		}

		if err != nil {
			log.Printf("[Upload] Receive error: %v\n", err)
			if !s.memoryMode {
				os.Remove(recFilePath)
			}
			return s.sendUploadError(stream, "Receive error: file transfer incomplete")
		}

		chunk := req.GetChunk()
		if chunk == nil {
			continue
		}

		fileData := chunk.GetData()
		if len(fileData) == 0 {
			continue
		}

		if s.memoryMode {
			memBuf = append(memBuf, fileData...)
		} else {
			if _, err := bufWriter.Write(fileData); err != nil {
				log.Printf("[Upload] Write error: %v\n", err)
				os.Remove(recFilePath)
				return s.sendUploadError(stream, "Receive error: write file error")
			}
		}

		if fileChunks > 0 {
			common.ShowProgress(chunk.GetChunk(), fileChunks)
		}
	}

	if !s.memoryMode {
		if err := bufWriter.Flush(); err != nil {
			os.Remove(recFilePath)
			log.Printf("[Upload] Flush error: %v\n", err)
			return s.sendUploadError(stream, "Receive error: save file")
		}
		if err := recFile.Sync(); err != nil {
			os.Remove(recFilePath)
			log.Printf("[Upload] Sync error: %v\n", err)
			return s.sendUploadError(stream, "Receive error: sync file")
		}
	}

	if err := common.ValidateFileIntegrity(common.FileValidationInfo{
		Data:         memBuf,
		FilePath:     recFilePath,
		ExpectedSize: fileSize,
		ExpectedHash: fileHash,
		IsMemory:     s.memoryMode,
	}); err != nil {
		if !s.memoryMode {
			os.Remove(recFilePath)
		}
		log.Printf("[Upload] Validation error: %v\n", err)
		return s.sendUploadError(stream, fmt.Sprintf("Receive error: %v", err))
	}

	elapsed := time.Since(startTime)
	speed := float64(fileSize) / elapsed.Seconds()
	speedStr := common.FormatSpeed(speed)

	if s.memoryMode {
		log.Printf("[Upload] Success (memory): size=%d, speed=%s\n", len(memBuf), speedStr)
	} else {
		log.Printf("[Upload] Success: %s, speed=%s\n", fileName, speedStr)
	}

	return s.sendUploadSuccess(stream, "Receive complete")
}

func (s *FileService) DownloadFile(in *pb.DownloadRequest, stream pb.FileTransferService_DownloadFileServer) error {
	fileTag := in.GetTag()
	fileName := in.GetName()
	fileChunksize := in.GetChunksize()

	log.Printf("[Download] Request: tag=%s, name=%s, chunksize=%d\n", fileTag, fileName, fileChunksize)

	if s.memoryMode {
		return s.sendDownloadError(stream, "download not supported in Memory Mode")
	}

	ok, err := common.FileIsExist(s.savePath, fileTag, fileName, "")
	if err != nil {
		log.Printf("[Download] File error: %s \n", err.Error())
		return s.sendDownloadError(stream, "file not found")
	}

	if !ok {
		log.Printf("[Download] File %s/%s does not exist\n", fileTag, fileName)
		return s.sendDownloadError(stream, "file does not exist")
	}

	srcFilePath := utils.FileSuite.JoinPath(s.savePath, fileTag, fileName)

	srcFileInfo, err := os.Stat(srcFilePath)
	if err != nil {
		log.Printf("[Download] Stat error: %s \n", err.Error())
		return s.sendDownloadError(stream, "file access error")
	}

	srcFileSize := srcFileInfo.Size()
	chunkSize64 := fileChunksize
	if chunkSize64 <= 0 {
		log.Printf("[Download] Invalid chunksize: %d\n", chunkSize64)
		return s.sendDownloadError(stream, "invalid chunksize")
	}
	totalChunks := (srcFileSize + chunkSize64 - 1) / chunkSize64

	srcFileHash, err := common.CalcBlake3(srcFilePath)
	if err != nil {
		log.Printf("[Download] Hash calculation error: %s \n", err.Error())
		return s.sendDownloadError(stream, "hash calculation error")
	}

	// 1. send metadata
	if err := stream.Send(&pb.DownloadResponse{
		Payload: &pb.DownloadResponse_Metadata{
			Metadata: &pb.FileMetadata{
				Tag:       fileTag,
				Name:      fileName,
				Size:      srcFileSize,
				Chunks:    totalChunks,
				Chunksize: chunkSize64,
				Hash:      srcFileHash,
			},
		},
	}); err != nil {
		log.Printf("[Download] Send metadata error: %s \n", err.Error())
		return fmt.Errorf("failed to send metadata")
	}

	log.Printf("[Download] Metadata sent: size=%d, chunks=%d x %d Byte, hash=%s\n",
		srcFileSize, totalChunks, chunkSize64, srcFileHash)

	file, err := common.OpenTargetFile(srcFilePath, common.FileRead)
	if err != nil {
		log.Printf("[Download] Open error: %s \n", err.Error())
		return s.sendDownloadError(stream, "file open error")
	}
	defer file.Close()

	bufReader := bufio.NewReaderSize(file, 64*1024)

	log.Println("[Download] Start sending data")
	var sentChunks int64 = 0
	startTime := time.Now()
	chunkSize := int(chunkSize64)
	buffer := make([]byte, chunkSize)

	// 2. send file data
	for {
		n, err := bufReader.Read(buffer)
		if err != nil && err != io.EOF {
			log.Printf("[Download] Read error: %s \n", err.Error())
			return s.sendDownloadError(stream, "file read error")
		}

		if n == 0 {
			break
		}

		sentChunks++
		if err := stream.Send(&pb.DownloadResponse{
			Payload: &pb.DownloadResponse_Chunk{
				Chunk: &pb.ChunkData{
					Chunk: sentChunks,
					Data:  buffer[:n],
				},
			},
		}); err != nil {
			log.Printf("[Download] Send error: %s \n", err.Error())
			return s.sendDownloadError(stream, "file send error")
		}

		common.ShowProgress(sentChunks, totalChunks)
	}

	elapsed := time.Since(startTime)
	speed := float64(srcFileSize) / elapsed.Seconds()
	speedStr := common.FormatSpeed(speed)

	log.Printf("[Download] Complete: %s, speed=%s\n", fileName, speedStr)

	// 3. send result
	return s.sendDownloadSuccess(stream, "download complete")
}

func (s *FileService) ListFiles(ctx context.Context, in *pb.ListFilesRequest) (*pb.ListFilesResponse, error) {
	if s.memoryMode {
		return &pb.ListFilesResponse{
			Status:  false,
			Message: "ListFiles not supported in Memory Mode",
			Files:   nil,
		}, nil
	}

	tag := in.GetTag()
	files, err := common.GetFileList(s.savePath, tag)
	if err != nil {
		return &pb.ListFilesResponse{
			Status:  false,
			Message: "Get file list error: " + err.Error(),
			Files:   nil,
		}, nil
	}

	log.Printf("Found %d files under tag %s\n", len(files), tag)

	return &pb.ListFilesResponse{
		Status:  true,
		Message: "File list retrieved successfully",
		Files:   files,
	}, nil
}
