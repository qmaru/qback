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
	"context"
	"fmt"
	"log"
	"math"
	"os"
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
	if c.Secure {
		log.Println("TLS ON")
		tlsConfig, certPool, err := common.GenTLSInfo("client")
		if err != nil {
			return nil, err
		}
		tlsConfig.ServerName = "127.0.0.1"
		tlsConfig.RootCAs = certPool
		cred = credentials.NewTLS(tlsConfig)
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

	// 获取文件属性
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}
	fileName := fileInfo.Name()
	fileSize := fileInfo.Size()
	fileChunks := int64(math.Ceil(float64(fileSize) / float64(c.Chunksize)))
	fileHash, err := common.CalcBlake3(filePath)
	if err != nil {
		return "", err
	}

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

	// 3. send file chunks
	fileBody, err := os.Open(filePath)
	if err != nil {
		stream.CloseSend()
		return "", err
	}
	defer fileBody.Close()

	startTime := time.Now()
	// 发送文件流
	buffer := make([]byte, c.Chunksize)
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
		}

		// 显示进度
		common.ShowProgress(chunk, fileChunks)
	}

	elapsed := time.Since(startTime)
	speed := float64(fileSize) / elapsed.Seconds()
	speedStr := common.FormatSpeed(speed)

	log.Printf("[Upload] Complete: %s, speed=%s\n", fileName, speedStr)

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

	log.Printf("[Upload] Success: %s\n", result.Message)
	return result.Message, nil
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
