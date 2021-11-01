/*
客户端发送文件流程：
	1.向服务端发送 meta 数据，包含文件名(name)，文件大小(size)，文件分片数(chunks)，文件哈希值(hash)
	2.服务端验证通过后，客户端向服务端推送文件流，包含数据(data)，当前分片(chunk)，文件标签(tag)
	3.推送完成后客户端关闭连接

客户端预处理的事项：
	1.计算待发送文件的哈希值(hash)
	2.根据分片大小计算分片数量(chunks)
	3.分片大小为 1MB
*/
package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"math"
	"os"
	"time"

	pb "qBack/libs"
	"qBack/utils"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	chunksize     int64         = 1 << 20 // 1M
	clientTimeout time.Duration = 1800    // 超时时间30分钟
)

func PingServer(address string) {
	BindAddress := utils.ListenOrConnect(address, false)
	// 设置客户端超时时间
	ctx, cancel := context.WithTimeout(context.Background(), clientTimeout*time.Second)
	defer cancel()

	var conn *grpc.ClientConn
	var err error
	if utils.Debug {
		conn, err = grpc.DialContext(ctx, BindAddress, grpc.WithInsecure(), grpc.WithBlock())
	} else {
		// 获取证书
		clientCert, clientKey := utils.ReadCertsCfg("client")
		caCert, _ := utils.ReadCertsCfg("ca")
		// 设置证书
		cert, _ := tls.LoadX509KeyPair(clientCert, clientKey)
		certPool := x509.NewCertPool()
		ca, _ := ioutil.ReadFile(caCert)
		certPool.AppendCertsFromPEM(ca)

		creds := credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
			ServerName:   "127.0.0.1",
			RootCAs:      certPool,
		})

		conn, err = grpc.DialContext(ctx, BindAddress, grpc.WithTransportCredentials(creds), grpc.WithBlock())
	}

	if err != nil {
		utils.PrintError("Connect Failed", err)
	}

	defer conn.Close()
	// 设置 gRPC 客户端
	client := pb.NewFileTransferServiceClient(conn)
	result, _ := client.CheckServer(ctx, &pb.SayRequest{Ststus: true})
	if result.GetStatus() {
		log.Printf("[%s] is Up\n", address)
	} else {
		log.Printf("[%s] is Down\n", address)
	}
}

func Run(address, filetag, filepath string) {
	BindAddress := utils.ListenOrConnect(address, false)
	// 设置客户端超时时间
	ctx, cancel := context.WithTimeout(context.Background(), clientTimeout*time.Second)
	defer cancel()

	var conn *grpc.ClientConn
	var err error
	if utils.Debug {
		conn, err = grpc.DialContext(ctx, BindAddress, grpc.WithInsecure(), grpc.WithBlock())
	} else {
		// 获取证书
		clientCert, clientKey := utils.ReadCertsCfg("client")
		caCert, _ := utils.ReadCertsCfg("ca")
		// 设置证书
		cert, _ := tls.LoadX509KeyPair(clientCert, clientKey)
		certPool := x509.NewCertPool()
		ca, _ := ioutil.ReadFile(caCert)
		certPool.AppendCertsFromPEM(ca)

		creds := credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
			ServerName:   "127.0.0.1",
			RootCAs:      certPool,
		})

		conn, err = grpc.DialContext(ctx, BindAddress, grpc.WithTransportCredentials(creds), grpc.WithBlock())
	}

	if err != nil {
		utils.PrintError("Connect Failed", err)
	}

	defer conn.Close()
	// 设置 gRPC 客户端
	client := pb.NewFileTransferServiceClient(conn)
	FileStream(ctx, client, filetag, filepath)
}

func FileStream(ctx context.Context, client pb.FileTransferServiceClient, filetag, filepath string) {
	// 获取文件属性
	fileInfo, err := os.Stat(filepath)
	if err != nil {
		utils.PrintError("File Not Found", err)
	}
	filename := fileInfo.Name()
	filesize := fileInfo.Size()
	filechunks := int64(math.Ceil(float64(filesize) / float64(chunksize)))
	filehash := utils.CalcMD5(filepath)
	// 发送 meta 数据
	res, err := client.SendMeta(ctx, &pb.MetaRequest{
		Name:   filename,
		Size:   filesize,
		Chunks: filechunks,
		Hash:   filehash,
		Tag:    filetag,
	})
	if res.GetStatus() {
		msg := res.GetMessage()
		if res.GetStatus() {
			log.Println(msg)
			// 初始化客户端
			stream, err := client.SendFile(context.Background())
			if err != nil {
				utils.PrintError("Client Create Failed", err)
			}
			// 获取文件内容
			fileBody, err := os.Open(filepath)
			if err != nil {
				utils.PrintError("File Open Error", err)
			}
			defer fileBody.Close()
			// 发送文件
			buffer := make([]byte, chunksize)
			for chunk := int64(1); chunk <= filechunks; chunk++ {
				// 计算分片大小
				fileChunksize := (chunk - 1) * chunksize
				// 设置偏移量
				fileBody.Seek(fileChunksize, 0)
				// 设置最后的分片的偏移量
				if len(buffer) > int((filesize - fileChunksize)) {
					buffer = make([]byte, filesize-fileChunksize)
				}
				// 读取内容
				offset, err := fileBody.Read(buffer)
				if err != nil {
					utils.PrintError("Read File Error", err)
				}

				err = stream.Send(&pb.FileRequest{
					Chunk: chunk,
					Data:  buffer[:offset],
				})
				utils.SetProgress(chunk, filechunks)
				if err != nil {
					utils.PrintError("Client Send Error", err)
				}
			}

			res, err := stream.CloseAndRecv()
			serverMsg := res.GetMessage()
			if err != nil {
				utils.PrintError(serverMsg, err)
			}
			log.Println(serverMsg)
		} else {
			utils.PrintError(msg, nil)
		}
	} else {
		log.Println(res.GetMessage(), err)
	}
}
