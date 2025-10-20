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
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"qback/grpc/common"
	pb "qback/grpc/libs"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

type ServerBasic struct {
	ListenAddress string
	SavePath      string
	Secure        bool
}

type FileService struct {
	savePath string
	pb.UnimplementedFileTransferServiceServer
}

type fileTransferContext struct {
	fileTag    string
	fileName   string
	fileSize   int64
	fileChunks int64
	fileHash   string
}

func loggerMid(ctx context.Context, info *grpc.UnaryServerInfo) error {
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

// logInterceptor 日志拦截器
func logInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	// 输出日志
	err := loggerMid(ctx, info)
	if err != nil {
		return nil, err
	}
	// 继续处理请求
	return handler(ctx, req)
}

func streamLogInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
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
		grpc.UnaryInterceptor(logInterceptor),
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
	pb.RegisterFileTransferServiceServer(server, &FileService{savePath: s.SavePath})

	log.Println("Server is ready")
	err = server.Serve(listener)
	if err != nil {
		return err
	}
	return nil
}

func (s *FileService) ServerCheck(ctx context.Context, in *pb.Ping) (*pb.Pong, error) {
	if in.GetStatus() {
		return &pb.Pong{Status: true}, nil
	}
	return &pb.Pong{Status: false}, nil
}

func (s *FileService) SendMeta(ctx context.Context, in *pb.MetaRequest) (*pb.MetaResponse, error) {
	fileTag := in.GetTag()
	fileName := in.GetName()
	fileHash := in.GetHash()

	log.Printf("[SendMeta] Received: tag=%s, name=%s, hash=%s\n", fileTag, fileName, fileHash)

	ok, err := FileIsExist(s.savePath, fileTag, fileName, fileHash)
	if err != nil {
		log.Printf("[SendMeta] Check file error: %v\n", err)
		return &pb.MetaResponse{Status: false, Message: "Check file error: " + err.Error()}, nil
	}

	log.Printf("[SendMeta] FileIsExist returned: %v\n", ok)

	if ok {
		log.Printf("[SendMeta] File already exists, returning false status\n")
		return &pb.MetaResponse{Status: false, Message: "File already exists"}, nil
	}

	log.Printf("[SendMeta] File does not exist, allowing upload\n")

	fileSize := in.GetSize()
	fileChunks := in.GetChunks()

	header := metadata.Pairs(
		"file-tag", fileTag,
		"file-name", fileName,
		"file-hash", fileHash,
		"file-size", strconv.FormatInt(fileSize, 10),
		"file-chunks", strconv.FormatInt(fileChunks, 10),
	)
	grpc.SendHeader(ctx, header)

	log.Printf("Meta received: tag=%s, name=%s, size=%d, chunks=%d, hash=%s\n",
		fileTag, fileName, fileSize, fileChunks, fileHash)

	return &pb.MetaResponse{Status: true, Message: "The server allows receiving"}, nil
}

func (s *FileService) SendFile(stream pb.FileTransferService_SendFileServer) error {
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Failed to get metadata"})
	}

	transferCtx := &fileTransferContext{}

	if tags := md.Get("file-tag"); len(tags) > 0 {
		transferCtx.fileTag = tags[0]
	} else {
		return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Missing file-tag in metadata"})
	}

	if names := md.Get("file-name"); len(names) > 0 {
		transferCtx.fileName = names[0]
	} else {
		return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Missing file-name in metadata"})
	}

	if hashes := md.Get("file-hash"); len(hashes) > 0 {
		transferCtx.fileHash = hashes[0]
	} else {
		return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Missing file-hash in metadata"})
	}

	if sizes := md.Get("file-size"); len(sizes) > 0 {
		size, err := strconv.ParseInt(sizes[0], 10, 64)
		if err != nil {
			return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Invalid file-size in metadata"})
		}
		transferCtx.fileSize = size
	} else {
		return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Missing file-size in metadata"})
	}

	if chunks := md.Get("file-chunks"); len(chunks) > 0 {
		chunk, err := strconv.ParseInt(chunks[0], 10, 64)
		if err != nil {
			return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Invalid file-chunks in metadata"})
		}
		transferCtx.fileChunks = chunk
	} else {
		return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Missing file-chunks in metadata"})
	}

	dstFilePath, err := SetDstFilePath(s.savePath, transferCtx.fileTag, transferCtx.fileName)
	if err != nil {
		log.Printf("Get save filepath error: %v\n", err)
		return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Server receive failed: " + err.Error()})
	}

	recFile, err := CreateSaveFile(dstFilePath)
	if err != nil {
		log.Printf("Create save file error: %v\n", err)
		return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Server receive failed: " + err.Error()})
	}

	recFileName := recFile.Name()
	defer recFile.Close()

	bufWriter := bufio.NewWriterSize(recFile, 64*1024)

	for {
		in, err := stream.Recv()

		if err == io.EOF {
			if err := bufWriter.Flush(); err != nil {
				log.Printf("Flush buffer error: %v\n", err)
				os.Remove(recFileName)
				return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Flush failed: " + err.Error()})
			}

			if err := recFile.Sync(); err != nil {
				log.Printf("Sync file error: %v\n", err)
				os.Remove(recFileName)
				return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Sync failed: " + err.Error()})
			}

			fileInfo, err := os.Stat(recFileName)
			if err != nil {
				log.Printf("Stat file error: %v\n", err)
				os.Remove(recFileName)
				return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "File stat error: " + err.Error()})
			}

			if fileInfo.Size() != transferCtx.fileSize {
				log.Printf("File size mismatch: expected %d, got %d\n", transferCtx.fileSize, fileInfo.Size())
				os.Remove(recFileName)
				return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "File size mismatch"})
			}

			recHash, err := common.CalcBlake3(recFileName)
			if err != nil {
				log.Printf("Calculate hash error: %v\n", err)
				os.Remove(recFileName)
				return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "File hash error: " + err.Error()})
			}

			if recHash != transferCtx.fileHash {
				log.Printf("Hash mismatch: expected %s, got %s (file: %s, size: %d)\n", transferCtx.fileHash, recHash, recFileName, fileInfo.Size())
				os.Remove(recFileName)
				return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "File hash mismatch"})
			}

			log.Printf("Receive succeed: %s\n", recFileName)

			return stream.SendAndClose(&pb.FileResponse{Status: true, Message: "Server Receive Succeed"})
		}

		if err != nil {
			log.Printf("Receive error: %v\n", err)
			return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Server receive failed: " + err.Error()})
		}

		// 保存文件
		fileData := in.GetData()
		if len(fileData) == 0 {
			continue
		}

		_, err = bufWriter.Write(fileData)
		if err != nil {
			log.Printf("Write file error: %v\n", err)
			return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Write file failed: " + err.Error()})
		}

		// 进度条
		chunk := in.GetChunk()
		if transferCtx.fileChunks > 0 {
			common.ShowProgress(chunk, transferCtx.fileChunks)
		}
	}
}
