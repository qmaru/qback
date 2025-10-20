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
	"log"
	"math"
	"os"
	"strconv"
	"time"

	"qback/grpc/common"
	pb "qback/grpc/libs"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type ClientBasic struct {
	conn           *grpc.ClientConn
	ctx            context.Context
	cancel         context.CancelFunc
	ConnectTimeout int
	MetaTimeout    int
	Chunksize      int
	ServerAddress  string
	Secure         bool
}

func (c *ClientBasic) defaultTimeout() {
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = 10
	}
	if c.MetaTimeout == 0 {
		c.MetaTimeout = 30
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

	log.Printf("Connection Configuration:")
	log.Printf("├─ Max Message Size: %d MB (Send/Recv)", common.MaxMsgSize/(1024*1024))
	log.Printf("├─ Connect Timeout: %d seconds (configurable)", c.ConnectTimeout)
	log.Printf("├─ Metadata Timeout: %d seconds (configurable)", c.MetaTimeout)
	log.Printf("├─ Stream Timeout: No limit (continuous transfer)")
	log.Printf("└─ Retry Policy: Max 4 attempts, 3s~30s backoff (UNAVAILABLE, UNKNOWN)")

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

func (c *ClientBasic) FileStream(fileTag, filePath string) (string, error) {
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

	// 发送 meta 数据
	log.Printf("File Metadata: %s (Size: %d Byte, Chunks: %d x %d Byte, Hash: %s)\n",
		fileName, fileSize, fileChunks, c.Chunksize, fileHash)

	metaCtx, metaCancel := context.WithTimeout(c.ctx, time.Duration(c.MetaTimeout)*time.Second)
	defer metaCancel()

	res, err := client.SendMeta(metaCtx, &pb.MetaRequest{
		Name:   fileName,
		Size:   fileSize,
		Chunks: fileChunks,
		Hash:   fileHash,
		Tag:    fileTag,
	})
	if err != nil {
		return "", err
	}

	metaStatus := res.GetStatus()
	metaMessage := res.GetMessage()

	if !metaStatus {
		return metaMessage, nil
	}

	md := metadata.Pairs(
		"file-tag", fileTag,
		"file-name", fileName,
		"file-hash", fileHash,
		"file-size", strconv.FormatInt(fileSize, 10),
		"file-chunks", strconv.FormatInt(fileChunks, 10),
	)

	ctx := metadata.NewOutgoingContext(c.ctx, md)

	stream, err := client.SendFile(ctx)
	if err != nil {
		return "", err
	}

	// 打开文件
	fileBody, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer fileBody.Close()

	// 发送文件流
	buffer := make([]byte, c.Chunksize)
	for chunk := int64(1); chunk <= fileChunks; chunk++ {
		// 计算分片偏移量
		fileChunkOffset := (chunk - 1) * int64(c.Chunksize)
		fileBody.Seek(fileChunkOffset, 0)

		// 调整最后一个分片的大小
		remainingSize := fileSize - fileChunkOffset
		if int64(len(buffer)) > remainingSize {
			buffer = make([]byte, remainingSize)
		}

		// 读取数据
		n, err := fileBody.Read(buffer)
		if err != nil {
			return "", err
		}

		// 发送数据
		err = stream.Send(&pb.FileRequest{
			Chunk: chunk,
			Data:  buffer[:n],
		})
		if err != nil {
			return "", err
		}

		// 显示进度
		common.ShowProgress(chunk, fileChunks)
	}

	// 关闭流并接收响应
	streamRes, err := stream.CloseAndRecv()
	if err != nil {
		return "", err
	}

	streamStatus := streamRes.GetStatus()
	streamMessage := streamRes.GetMessage()

	if !streamStatus {
		log.Printf("Upload failed: %s\n", streamMessage)
	}

	return streamMessage, nil
}
