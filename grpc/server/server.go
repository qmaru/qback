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
	"context"
	"crypto/tls"
	"io"
	"log"
	"net"

	"qBack/grpc/common"
	pb "qBack/grpc/libs"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

var (
	savePath = ""
)

type ServerBasic struct {
	ListenAddress string
	SavePath      string
	Secure        bool
	Debug         bool
}

type FileService struct {
	fileTag    string
	fileName   string
	fileSize   int64
	fileChunks int64
	fileHash   string
	pb.UnimplementedFileTransferServiceServer
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

func (s *ServerBasic) Run() error {
	log.Printf("Listen on %s\n", s.ListenAddress)

	savePath = s.SavePath
	listener, err := net.Listen("tcp", s.ListenAddress)
	if err != nil {
		return err
	}

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(logInterceptor),
		grpc.MaxRecvMsgSize(common.MaxMsgSize),
		grpc.MaxSendMsgSize(common.MaxMsgSize),
	}

	if s.Secure {
		log.Println("TLS ON")
		tlsConfig, certPool, err := common.GenTLSInfo(s.Debug, "server")
		if err != nil {
			return err
		}
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		tlsConfig.ClientCAs = certPool
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.Creds(creds))
	}
	server := grpc.NewServer(opts...)
	pb.RegisterFileTransferServiceServer(server, &FileService{})
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

	ok, err := FileIsExist(savePath, fileTag, fileName, fileHash)
	if err != nil {
		return &pb.MetaResponse{Status: false, Message: "Check file error: " + err.Error()}, nil
	}

	if ok {
		return &pb.MetaResponse{Status: false, Message: "File already exists"}, nil
	}

	s.fileTag = fileTag
	s.fileName = fileName
	s.fileSize = in.GetSize()
	s.fileChunks = in.GetChunks()
	s.fileHash = fileHash
	return &pb.MetaResponse{Status: true, Message: "The server allows receiving"}, nil
}

func (s *FileService) SendFile(stream pb.FileTransferService_SendFileServer) error {
	recFile, err := CreateSaveFile(savePath, s.fileTag, s.fileName)
	if err != nil {
		stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Server receive failed: " + err.Error()})
	}
	defer recFile.Close()
	for {
		in, err := stream.Recv()
		// 保存文件
		fileData := in.GetData()
		recFile.Write(fileData)
		// 进度条
		chunk := in.GetChunk()
		common.ShowProgress(chunk, s.fileChunks)
		// 完成
		if err == io.EOF {
			dstFile, err := SetDstPath(savePath, s.fileTag, s.fileName)
			if err != nil {
				stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Set dst folder failed: " + err.Error()})
			}
			recMd5, err := common.CalcBlake3(dstFile)
			if err != nil {
				return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "File hash error: " + err.Error()})
			}
			if recMd5 == "" {
				return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "File hash is empty"})
			}
			if recMd5 == s.fileHash {
				log.Println("Receive Succeed")
				return stream.SendAndClose(&pb.FileResponse{Status: true, Message: "Server Receive Succeed"})
			}
		}
		if err != nil {
			return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Server receive failed: " + err.Error()})
		}
	}
}
