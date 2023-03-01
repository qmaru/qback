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
	"time"

	"qBack/grpc/common"
	pb "qBack/grpc/libs"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type ClientBasic struct {
	conn          *grpc.ClientConn
	ctx           context.Context
	cancel        context.CancelFunc
	Timeout       int
	Chunksize     int
	ServerAddress string
	Secure        bool
}

func (c *ClientBasic) defaultTimeout() {
	if c.Timeout == 0 {
		c.Timeout = 30
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
		cred = insecure.NewCredentials()
	}

	callOpt := grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(common.MaxMsgSize),
		grpc.MaxCallSendMsgSize(common.MaxMsgSize),
	)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(cred),
		grpc.WithBlock(),
		callOpt,
	}

	c.ctx, c.cancel = context.WithTimeout(context.Background(), time.Duration(c.Timeout)*time.Second)
	conn, err := grpc.DialContext(c.ctx, c.ServerAddress, opts...)
	if err != nil {
		return nil, err
	}
	c.conn = conn
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

	_, err = client.ServerCheck(c.ctx, &pb.Ping{Status: true})
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
	fileHash, err := common.CalcMD5(filePath)
	if err != nil {
		return "", err
	}
	// 发送 meta 数据
	log.Println("Sending metadata")
	log.Printf("  -- Name: %s", fileName)
	log.Printf("  -- Size: %d Byte", fileSize)
	log.Printf("  -- Chunk: %d Byte ", c.Chunksize)
	log.Printf("  -- Count: %d", fileChunks)
	res, err := client.SendMeta(c.ctx, &pb.MetaRequest{
		Name:   fileName,
		Size:   fileSize,
		Chunks: fileChunks,
		Hash:   fileHash,
		Tag:    fileTag,
	})
	if res.GetStatus() {
		message := res.GetMessage()
		if res.GetStatus() {
			log.Println(message)
			// 初始化客户端
			stream, err := client.SendFile(context.Background())
			if err != nil {
				return "", err
			}
			// 获取文件内容
			fileBody, err := os.Open(filePath)
			if err != nil {
				return "", err
			}
			defer fileBody.Close()
			// 发送文件
			buffer := make([]byte, c.Chunksize)
			for chunk := int64(1); chunk <= fileChunks; chunk++ {
				// 计算分片大小
				fileChunksize := (chunk - 1) * int64(c.Chunksize)
				// 设置偏移量
				fileBody.Seek(fileChunksize, 0)
				// 设置最后的分片的偏移量
				if len(buffer) > int((fileSize - fileChunksize)) {
					buffer = make([]byte, fileSize-fileChunksize)
				}
				// 读取内容
				offset, err := fileBody.Read(buffer)
				if err != nil {
					return "", err
				}

				err = stream.Send(&pb.FileRequest{
					Chunk: chunk,
					Data:  buffer[:offset],
				})
				common.ShowProgress(chunk, fileChunks)
				if err != nil {
					return "", err
				}
			}
			res, err := stream.CloseAndRecv()
			if err != nil {
				return "", err
			}
			message := res.GetMessage()
			if err != nil {
				return "", err
			}
			return message, nil
		}
		return message, nil
	}
	return res.GetMessage(), nil
}
