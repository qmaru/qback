/*
客户端发送文件流程：

	1.向服务端发送 meta 数据，包含文件名(name)，文件大小(size)，文件分片数(chunks)，文件哈希值(hash)
	2.服务端验证通过后，客户端向服务端推送文件流，包含数据(data)，当前分片(chunk)，文件标签(tag)
	3.推送完成后客户端关闭连接

客户端预处理的事项：

	1.计算待发送文件的哈希值(hash)
	2.根据分片大小计算分片数量(chunks)
	3.分片大小默认为 1MB
*/
package client

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"qback/grpc/common"
	pb "qback/grpc/libs"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type ClientBasic struct {
	conn          *grpc.ClientConn
	ctx           context.Context
	cancel        context.CancelFunc
	ChunkTimeout  int
	Chunksize     int
	ServerAddress string
	Secure        bool
}

func (c *ClientBasic) defaultTimeout() {
	if c.ChunkTimeout == 0 {
		c.ChunkTimeout = 30
	}
}

func (c *ClientBasic) connect() (pb.FileTransferServiceClient, error) {
	log.Printf("Connecting on %s\n", c.ServerAddress)
	c.defaultTimeout()

	var cred credentials.TransportCredentials
	var tlsConfig *tls.Config
	if c.Secure {
		log.Println("TLS ON")
		tlsCfg, err := common.GenTLSInfo("client", true)
		if err != nil {
			return nil, err
		}
		tlsCfg.ServerName = "127.0.0.1"
		cred = credentials.NewTLS(tlsCfg)
		tlsConfig = tlsCfg
	} else {
		log.Println("TLS OFF")
		cred = insecure.NewCredentials()
	}

	callOpt := grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(common.MaxMsgSize),
		grpc.MaxCallSendMsgSize(common.MaxMsgSize),
	)

	serverOpt := grpc.WithDefaultServiceConfig(common.RetryPolicy)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(cred),
		callOpt,
		serverOpt,
	}

	conn, err := grpc.NewClient(c.ServerAddress, opts...)
	if err != nil {
		return nil, err
	}

	if c.Secure {
		go common.ProbeTLSConnection(c.ServerAddress, tlsConfig)
	}

	c.conn = conn
	c.ctx, c.cancel = context.WithCancel(context.Background())
	client := pb.NewFileTransferServiceClient(c.conn)

	return client, nil
}

func (c *ClientBasic) close() {
	c.cancel()
	c.conn.Close()
}

func (c *ClientBasic) ServerCheck() error {
	client, err := c.connect()
	if err != nil {
		return err
	}
	defer c.close()

	checkCtx, checkCancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer checkCancel()

	_, err = client.ServerCheck(checkCtx, &pb.Ping{Status: true})
	if err != nil {
		return err
	}
	return nil
}

func (c *ClientBasic) UploadFile(fileTag, filePath string) (string, error) {
	client, err := c.connect()
	if err != nil {
		return "", err
	}
	defer c.close()

	var fileName string
	var fileSize int64
	var fileHash string
	var isBenchmark bool

	if strings.HasPrefix(filePath, "benchmark://") {
		isBenchmark = true
		parts := strings.Split(strings.TrimPrefix(filePath, "benchmark://"), "/")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid benchmark file format, use: benchmark://filename/size")
		}
		baseFileName := parts[0]
		fileName = fmt.Sprintf("%s_%d", baseFileName, time.Now().UnixNano())
		fileSize, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid file size: %w", err)
		}
		fileHash, err = c.calcVirtualHash(fileSize)
		if err != nil {
			return "", fmt.Errorf("failed to calc virtual hash: %w", err)
		}
		log.Printf("[Upload] Benchmark file: name=%s, size=%d\n", fileName, fileSize)
	} else {
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return "", err
		}
		fileName = fileInfo.Name()
		fileSize = fileInfo.Size()
		fileHash, err = common.CalcBlake3(filePath)
		if err != nil {
			return "", err
		}

		log.Printf("[Upload] Real file: name=%s, size=%d\n", fileName, fileSize)
	}

	fileChunks := int64(math.Ceil(float64(fileSize) / float64(c.Chunksize)))

	log.Printf("[Upload] Metadata tag=%s, name=%s, size=%d, chunks=%d x %d Byte, hash=%s\n",
		fileTag, fileName, fileSize, fileChunks, c.Chunksize, fileHash)

	stream, err := client.UploadFile(c.ctx)
	if err != nil {
		return "", err
	}

	// 1. send metadata
	if err := stream.Send(&pb.UploadRequest{
		Payload: &pb.UploadRequest_Metadata{
			Metadata: &pb.FileMetadata{
				Tag:       fileTag,
				Name:      fileName,
				Size:      fileSize,
				Chunks:    fileChunks,
				Chunksize: int64(c.Chunksize),
				Hash:      fileHash,
			},
		},
	}); err != nil {
		stream.CloseSend()
		return "", fmt.Errorf("failed to send metadata: %w", err)
	}

	ack, err := stream.Recv()
	if err != nil {
		stream.CloseSend()
		return "", fmt.Errorf("failed to receive ack: %w", err)
	}

	// 2. check ack
	metaAck := ack.GetMetaAck()
	if metaAck == nil || !metaAck.AllowUpload {
		message := "server rejected upload"
		if metaAck != nil {
			message = metaAck.Message
		} else if result := ack.GetResult(); result != nil {
			message = result.Message
		}
		stream.CloseSend()
		return message, nil
	}

	log.Printf("[Upload] Server ack: %s\n", metaAck.Message)
	log.Printf("[Upload] Server allowed, starting transfer\n")

	startTime := time.Now()
	buffer := make([]byte, c.Chunksize)
	var totalSent int64 = 0

	if isBenchmark {
		for chunk := int64(1); chunk <= fileChunks; chunk++ {
			chunkCtx, chunkCancel := context.WithTimeout(c.ctx, time.Duration(c.ChunkTimeout)*time.Second)

			remainingSize := fileSize - (chunk-1)*int64(c.Chunksize)
			chunkSize := int64(c.Chunksize)
			if remainingSize < chunkSize {
				chunkSize = remainingSize
			}

			sendErr := make(chan error, 1)
			go func(data []byte, chunkNum int64) {
				sendErr <- stream.Send(&pb.UploadRequest{
					Payload: &pb.UploadRequest_Chunk{
						Chunk: &pb.ChunkData{
							Chunk: chunkNum,
							Data:  data,
						},
					},
				})
			}(buffer[:chunkSize], chunk)

			select {
			case <-chunkCtx.Done():
				chunkCancel()
				stream.CloseSend()
				return "", fmt.Errorf("chunk %d/%d send timeout after %ds", chunk, fileChunks, c.ChunkTimeout)
			case err := <-sendErr:
				chunkCancel()
				if err != nil {
					stream.CloseSend()
					return "", fmt.Errorf("failed to send chunk %d/%d: %w", chunk, fileChunks, err)
				}
				totalSent += chunkSize
			}

			common.ShowProgress(chunk, fileChunks)
		}
	} else {
		// 3. send file chunks
		fileBody, err := os.Open(filePath)
		if err != nil {
			stream.CloseSend()
			return "", err
		}
		defer fileBody.Close()

		for chunk := int64(1); chunk <= fileChunks; chunk++ {
			chunkCtx, chunkCancel := context.WithTimeout(c.ctx, time.Duration(c.ChunkTimeout)*time.Second)

			// 计算分片偏移量
			fileChunkOffset := (chunk - 1) * int64(c.Chunksize)
			if _, err := fileBody.Seek(fileChunkOffset, 0); err != nil {
				chunkCancel()
				stream.CloseSend()
				return "", fmt.Errorf("failed to seek file at chunk %d: %w", chunk, err)
			}

			// 调整最后一个分片的大小
			remainingSize := fileSize - fileChunkOffset
			readBuffer := buffer
			if int64(len(buffer)) > remainingSize {
				readBuffer = buffer[:remainingSize]
			}

			// 读取数据
			n, err := fileBody.Read(readBuffer)
			if err != nil {
				chunkCancel()
				stream.CloseSend()
				return "", fmt.Errorf("failed to read file at chunk %d: %w", chunk, err)
			}

			sendErr := make(chan error, 1)
			go func() {
				sendErr <- stream.Send(&pb.UploadRequest{
					Payload: &pb.UploadRequest_Chunk{
						Chunk: &pb.ChunkData{
							Chunk: chunk,
							Data:  readBuffer[:n],
						},
					},
				})
			}()

			select {
			case <-chunkCtx.Done():
				chunkCancel()
				stream.CloseSend()
				return "", fmt.Errorf("chunk %d/%d send timeout after %ds", chunk, fileChunks, c.ChunkTimeout)
			case err := <-sendErr:
				chunkCancel()
				if err != nil {
					stream.CloseSend()
					return "", fmt.Errorf("failed to send chunk %d/%d: %w", chunk, fileChunks, err)
				}
				totalSent += int64(n)
			}

			// 显示进度
			common.ShowProgress(chunk, fileChunks)
		}
	}

	if err := stream.CloseSend(); err != nil {
		return "", fmt.Errorf("failed to close send: %w", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return "", fmt.Errorf("failed to receive final response: %w", err)
	}

	result := resp.GetResult()
	if result == nil {
		return "", fmt.Errorf("unexpected response type: result is nil")
	}

	if !result.Status {
		return "", fmt.Errorf("upload failed: %s", result.Message)
	}

	elapsed := time.Since(startTime)
	speed := float64(totalSent) / elapsed.Seconds()
	speedStr := common.FormatSpeed(speed)

	log.Printf("[Upload] Complete: %s, elapsed=%d, speed=%s (sent: %d bytes)\n", fileName, elapsed.Milliseconds(), speedStr, totalSent)
	log.Printf("[Upload] Success: %s\n", result.Message)
	return result.Message, nil
}

func (c *ClientBasic) DownloadFile(fileTag, fileName, savePath string) (string, error) {
	ok, err := common.FileIsExist(savePath, fileTag, fileName, "")

	if err != nil {
		return "", fmt.Errorf("check file exist failed: %w", err)
	} else if ok {
		log.Printf("[Download] File already exists: %s\n", fileName)
		return "", fmt.Errorf("file already exists: %s", fileName)
	}

	client, err := c.connect()
	if err != nil {
		return "", err
	}
	defer c.close()

	log.Printf("[Download] Request: tag=%s, name=%s, chunksize=%d\n", fileTag, fileName, c.Chunksize)

	stream, err := client.DownloadFile(c.ctx, &pb.DownloadRequest{
		Tag:       fileTag,
		Name:      fileName,
		Chunksize: int64(c.Chunksize),
	})

	if err != nil {
		return "", fmt.Errorf("failed to create download stream: %w", err)
	}

	// 1. 接收元数据
	resp, err := stream.Recv()
	if err != nil {
		return "", fmt.Errorf("failed to receive metadata: %w", err)
	}

	// 处理响应
	if resp.GetPayload() == nil {
		return "", fmt.Errorf("unexpected response type: payload is nil")
	}

	metadata := resp.GetMetadata()
	if metadata == nil {
		if result := resp.GetResult(); result != nil {
			return "", fmt.Errorf("server error: %s", result.Message)
		}
		return "", fmt.Errorf("missing metadata")
	}

	fileSize := metadata.GetSize()
	fileChunks := metadata.GetChunks()
	fileHash := metadata.GetHash()

	log.Printf("[Download] Metadata: size=%d, chunks=%d x %d Byte, hash=%s\n",
		fileSize, fileChunks, metadata.GetChunksize(), fileHash)

	dstFilePath, err := common.SetTargetFilePath(savePath, fileTag, fileName)
	if err != nil {
		return "", fmt.Errorf("create target file path failed: %w", err)
	}

	recFile, err := common.OpenTargetFile(dstFilePath, common.FileWrite)
	if err != nil {
		return "", fmt.Errorf("failed to open target file: %w", err)
	}
	defer recFile.Close()

	recFilePath := dstFilePath

	bufWriter := bufio.NewWriterSize(recFile, 64*1024)
	var receivedChunks int64
	var totalReceived int64 = 0
	startTime := time.Now()

	log.Println("[Download] Start receiving data")

	for {
		chunkCtx, chunkCancel := context.WithTimeout(c.ctx, time.Duration(c.ChunkTimeout)*time.Second)

		respChan := make(chan *pb.DownloadResponse, 1)
		errChan := make(chan error, 1)

		go func() {
			resp, err := stream.Recv()
			if err != nil {
				errChan <- err
				return
			}
			respChan <- resp
		}()

		var resp *pb.DownloadResponse
		var err error

		select {
		case <-chunkCtx.Done():
			chunkCancel()
			if bufWriter != nil {
				_ = bufWriter.Flush()
			}
			if recFile != nil {
				_ = recFile.Close()
			}
			_ = os.Remove(recFilePath)
			return "", fmt.Errorf("chunk receive timeout after %ds", c.ChunkTimeout)
		case err = <-errChan:
			chunkCancel()
			if err == io.EOF {
				break
			}
			if bufWriter != nil {
				_ = bufWriter.Flush()
			}
			if recFile != nil {
				_ = recFile.Close()
			}
			_ = os.Remove(recFilePath)
			return "", fmt.Errorf("failed to receive chunk: %w", err)
		case resp = <-respChan:
			chunkCancel()
		}

		if err == io.EOF {
			break
		}

		// 检查是否是结果消息
		if result := resp.GetResult(); result != nil {
			if !result.Status {
				if bufWriter != nil {
					_ = bufWriter.Flush()
				}
				if recFile != nil {
					_ = recFile.Close()
				}
				os.Remove(recFilePath)
				return "", fmt.Errorf("download failed: %s", result.Message)
			}
			log.Printf("[Download] Server confirmed: %s\n", result.Message)
			break
		}

		chunk := resp.GetChunk()
		if chunk == nil {
			continue
		}

		data := chunk.GetData()
		if len(data) == 0 {
			continue
		}

		totalReceived += int64(len(data))

		// 写入数据
		if _, err := bufWriter.Write(data); err != nil {
			if bufWriter != nil {
				_ = bufWriter.Flush()
			}
			if recFile != nil {
				_ = recFile.Close()
			}
			os.Remove(recFilePath)
			return "", fmt.Errorf("failed to write chunk: %w", err)
		}

		receivedChunks = chunk.GetChunk()
		common.ShowProgress(receivedChunks, fileChunks)
	}

	if err := bufWriter.Flush(); err != nil {
		if recFile != nil {
			_ = recFile.Close()
		}
		_ = os.Remove(recFilePath)
		return "", fmt.Errorf("failed to flush: %w", err)
	}

	if err := recFile.Sync(); err != nil {
		if recFile != nil {
			_ = recFile.Close()
		}
		_ = os.Remove(recFilePath)
		return "", fmt.Errorf("failed to sync: %w", err)
	}

	if err := common.ValidateFileIntegrity(common.FileValidationInfo{
		FilePath:     recFilePath,
		ExpectedSize: fileSize,
		ExpectedHash: fileHash,
	}); err != nil {
		if recFile != nil {
			_ = recFile.Close()
		}
		_ = os.Remove(recFilePath)
		return "", fmt.Errorf("validation error: %w", err)
	}

	elapsed := time.Since(startTime)
	speed := float64(totalReceived) / elapsed.Seconds()
	speedStr := common.FormatSpeed(speed)

	log.Printf("[Download] Complete: %s, elapsed: %d, received=%d bytes, speed=%s\n", fileName, elapsed.Milliseconds(), totalReceived, speedStr)
	log.Printf("[Download] Saved to: %s\n", recFilePath)

	return recFilePath, nil
}

func (c *ClientBasic) ListFiles(fileTag string) ([]*pb.ListFileItem, error) {
	client, err := c.connect()
	if err != nil {
		return nil, err
	}
	defer c.close()

	checkCtx, checkCancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer checkCancel()

	response, err := client.ListFiles(checkCtx, &pb.ListFilesRequest{Tag: fileTag})
	if err != nil {
		return nil, err
	}

	if response.GetStatus() {
		return response.GetFiles(), nil

	}

	return nil, fmt.Errorf("%s", response.GetMessage())
}

func (c *ClientBasic) calcVirtualHash(fileSize int64) (string, error) {
	buffer := make([]byte, c.Chunksize)

	var data []byte
	var written int64

	for written < fileSize {
		remainingSize := fileSize - written
		if remainingSize < int64(c.Chunksize) {
			data = append(data, buffer[:remainingSize]...)
			written += remainingSize
		} else {
			data = append(data, buffer...)
			written += int64(c.Chunksize)
		}
	}

	return common.CalcBlake3FromBytes(data)
}
