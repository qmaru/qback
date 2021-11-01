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
	"crypto/x509"
	"io"
	"io/ioutil"
	"log"
	"net"

	pb "qBack/libs"
	"qBack/utils"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

var (
	DirPath = ""
)

type FileService struct {
	filetag    string
	filename   string
	filesize   int64
	filechunks int64
	filehash   string
	pb.UnimplementedFileTransferServiceServer
}

func Run(address, rootPath string) {
	BindAddress := utils.ListenOrConnect(address, true)
	DirPath = rootPath

	listenAddress, err := net.Listen("tcp", BindAddress)
	if err != nil {
		log.Fatalf("Listen Failed: %v", err)
	}

	var server *grpc.Server
	if utils.Debug {
		server = grpc.NewServer()
	} else {
		// 获取证书
		serverCert, serverKey := utils.ReadCertsCfg("server")
		caCert, _ := utils.ReadCertsCfg("ca")
		// 设置证书
		cert, _ := tls.LoadX509KeyPair(serverCert, serverKey)
		certPool := x509.NewCertPool()
		ca, _ := ioutil.ReadFile(caCert)
		certPool.AppendCertsFromPEM(ca)

		creds := credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.RequireAndVerifyClientCert,
			ClientCAs:    certPool,
		})
		server = grpc.NewServer(grpc.Creds(creds))
	}
	pb.RegisterFileTransferServiceServer(server, &FileService{})
	server.Serve(listenAddress)
}

func (s *FileService) CheckServer(ctx context.Context, in *pb.SayRequest) (*pb.SayResponse, error) {
	if in.GetStstus() {
		pr, _ := peer.FromContext(ctx)
		log.Printf("Client From: %s\n", pr.Addr.String())
		return &pb.SayResponse{Status: true}, nil
	}
	return &pb.SayResponse{Status: false}, nil
}

func (s *FileService) SendMeta(ctx context.Context, in *pb.MetaRequest) (*pb.MetaResponse, error) {
	filetag := in.GetTag()
	filename := in.GetName()
	filehash := in.GetHash()

	if utils.CheckFile(DirPath, filetag, filename, filehash) {
		return &pb.MetaResponse{Status: false, Message: "File Already Exists"}, nil
	}

	s.filetag = filetag
	s.filename = filename
	s.filesize = in.GetSize()
	s.filechunks = in.GetChunks()
	s.filehash = filehash
	log.Println("Waiting for the client to send the file...")
	return &pb.MetaResponse{Status: true, Message: "Please send the file"}, nil
}

func (s *FileService) SendFile(stream pb.FileTransferService_SendFileServer) error {
	recFile, err := utils.CreateSaveFile(DirPath, s.filetag, s.filename)
	if err != nil {
		stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Server Receive Failed"})
	}
	defer recFile.Close()
	for {
		res, err := stream.Recv()
		// 保存文件
		fileData := res.GetData()
		recFile.Write(fileData)
		// 进度条
		chunk := res.GetChunk()
		utils.SetProgress(chunk, s.filechunks)
		// 完成
		if err == io.EOF {
			savefile := utils.SetSavePath(DirPath, s.filetag, s.filename)
			recMd5 := utils.CalcMD5(savefile)
			if recMd5 == "" {
				return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Server Receive Failed"})
			}
			if recMd5 == s.filehash {
				log.Printf("Server Receive Succeed")
				return stream.SendAndClose(&pb.FileResponse{Status: true, Message: "Server Receive Succeed"})
			}
			return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "File Hash Error"})
		}
		if err != nil {
			return stream.SendAndClose(&pb.FileResponse{Status: false, Message: "Server Receive Failed"})
		}
	}
}
