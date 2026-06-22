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
	transferv1 "qback/internal/pb/qmeta/transfer/v1"
	"qback/utils"

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
	Debug         bool
}

func (c *ClientBasic) logDebug(format string, v ...any) {
	if c.Debug {
		utils.LogDebug(format, v...)
	}
}

func (c *ClientBasic) shouldLogChunk(chunk, total int64) bool {
	if total <= 10 {
		return true
	}
	return chunk == 1 || chunk == total || chunk%100 == 0
}

func (c *ClientBasic) defaultTimeout() {
	if c.ChunkTimeout == 0 {
		c.ChunkTimeout = 30
	}
}

func (c *ClientBasic) connect() (transferv1.FileTransferServiceClient, error) {
	log.Printf("Connecting on %s\n", c.ServerAddress)
	c.defaultTimeout()
	c.logDebug("connect config: address=%s secure=%t chunk_timeout=%ds chunksize=%d", c.ServerAddress, c.Secure, c.ChunkTimeout, c.Chunksize)

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

	c.logDebug("timeout=%ds", c.ChunkTimeout)
	c.logDebug("call options: %+v", callOpt)
	c.logDebug("dial options: %+v", serverOpt)

	conn, err := grpc.NewClient(c.ServerAddress, opts...)
	if err != nil {
		c.logDebug("connect failed: %v", err)
		return nil, err
	}

	if c.Secure {
		go common.ProbeTLSConnection(c.ServerAddress, tlsConfig)
		c.logDebug("started tls probe for %s", c.ServerAddress)
	}

	c.conn = conn
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.logDebug("grpc client connection ready")
	client := transferv1.NewFileTransferServiceClient(c.conn)

	return client, nil
}

func (c *ClientBasic) close() {
	c.logDebug("closing client resources")
	if c.cancel != nil {
		c.cancel()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

func (c *ClientBasic) ServerCheck() error {
	client, err := c.connect()
	if err != nil {
		return err
	}
	defer c.close()

	checkCtx, checkCancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer checkCancel()

	checkReq := &transferv1.ServerCheckRequest{}
	checkReq.SetStatus(true)
	c.logDebug("sending server check request")
	_, err = client.ServerCheck(checkCtx, checkReq)
	if err != nil {
		c.logDebug("server check failed: %v", err)
		return err
	}
	c.logDebug("server check succeeded")
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
		c.logDebug("upload source resolved: benchmark path=%s generated_name=%s", filePath, fileName)
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
		c.logDebug("upload source resolved: path=%s name=%s size=%d", filePath, fileName, fileSize)
	}

	fileChunks := int64(math.Ceil(float64(fileSize) / float64(c.Chunksize)))
	c.logDebug("upload metadata prepared: tag=%s name=%s chunks=%d hash=%s", fileTag, fileName, fileChunks, fileHash)

	log.Printf("[Upload] Metadata tag=%s, name=%s, size=%d, chunks=%d x %d Byte, hash=%s\n",
		fileTag, fileName, fileSize, fileChunks, c.Chunksize, fileHash)

	stream, err := client.UploadFile(c.ctx)
	if err != nil {
		c.logDebug("failed to create upload stream: %v", err)
		return "", err
	}
	c.logDebug("upload stream created")

	// 1. send metadata
	fileMetadata := &transferv1.FileMetadata{}
	fileMetadata.SetTag(fileTag)
	fileMetadata.SetName(fileName)
	fileMetadata.SetSize(fileSize)
	fileMetadata.SetChunks(fileChunks)
	fileMetadata.SetChunksize(int64(c.Chunksize))
	fileMetadata.SetHash(fileHash)

	uploadReq := &transferv1.UploadFileRequest{}
	uploadReq.SetMetadata(fileMetadata)

	c.logDebug("sending upload metadata to server")
	if err := stream.Send(uploadReq); err != nil {
		stream.CloseSend()
		c.logDebug("send metadata failed: %v", err)
		return "", fmt.Errorf("failed to send metadata: %w", err)
	}

	ack, err := stream.Recv()
	if err != nil {
		stream.CloseSend()
		c.logDebug("receive metadata ack failed: %v", err)
		return "", fmt.Errorf("failed to receive ack: %w", err)
	}

	// 2. check ack
	metaAck := ack.GetMetaAck()
	if metaAck == nil || !metaAck.GetAllowUpload() {
		message := "server rejected upload"
		if metaAck != nil {
			message = metaAck.GetMessage()
		} else if result := ack.GetResult(); result != nil {
			message = result.GetMessage()
		}
		stream.CloseSend()
		c.logDebug("server rejected upload: %s", message)
		return message, nil
	}

	log.Printf("[Upload] Server ack: %s\n", metaAck.GetMessage())
	log.Printf("[Upload] Server allowed, starting transfer\n")
	c.logDebug("server ack details: allow=%t message=%s", metaAck.GetAllowUpload(), metaAck.GetMessage())

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
			if c.shouldLogChunk(chunk, fileChunks) {
				c.logDebug("sending benchmark chunk=%d/%d bytes=%d", chunk, fileChunks, chunkSize)
			}

			sendErr := make(chan error, 1)
			go func(data []byte, chunkNum int64) {
				chunk := &transferv1.ChunkData{}
				chunk.SetChunk(chunkNum)
				chunk.SetData(data)
				uploadReq := &transferv1.UploadFileRequest{}
				uploadReq.SetChunk(chunk)
				sendErr <- stream.Send(uploadReq)
			}(buffer[:chunkSize], chunk)

			select {
			case <-chunkCtx.Done():
				chunkCancel()
				stream.CloseSend()
				c.logDebug("benchmark chunk timeout: chunk=%d/%d timeout=%ds", chunk, fileChunks, c.ChunkTimeout)
				return "", fmt.Errorf("chunk %d/%d send timeout after %ds", chunk, fileChunks, c.ChunkTimeout)
			case err := <-sendErr:
				chunkCancel()
				if err != nil {
					stream.CloseSend()
					c.logDebug("benchmark chunk send failed: chunk=%d/%d err=%v", chunk, fileChunks, err)
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
				c.logDebug("read file chunk failed: chunk=%d err=%v", chunk, err)
				return "", fmt.Errorf("failed to read file at chunk %d: %w", chunk, err)
			}
			if c.shouldLogChunk(chunk, fileChunks) {
				c.logDebug("sending file chunk=%d/%d offset=%d bytes=%d", chunk, fileChunks, fileChunkOffset, n)
			}

			sendErr := make(chan error, 1)
			go func() {
				chunkData := &transferv1.ChunkData{}
				chunkData.SetChunk(chunk)
				chunkData.SetData(readBuffer[:n])

				uploadReq := &transferv1.UploadFileRequest{}
				uploadReq.SetChunk(chunkData)

				sendErr <- stream.Send(uploadReq)
			}()

			select {
			case <-chunkCtx.Done():
				chunkCancel()
				stream.CloseSend()
				c.logDebug("file chunk timeout: chunk=%d/%d timeout=%ds", chunk, fileChunks, c.ChunkTimeout)
				return "", fmt.Errorf("chunk %d/%d send timeout after %ds", chunk, fileChunks, c.ChunkTimeout)
			case err := <-sendErr:
				chunkCancel()
				if err != nil {
					stream.CloseSend()
					c.logDebug("file chunk send failed: chunk=%d/%d err=%v", chunk, fileChunks, err)
					return "", fmt.Errorf("failed to send chunk %d/%d: %w", chunk, fileChunks, err)
				}
				totalSent += int64(n)
			}

			// 显示进度
			common.ShowProgress(chunk, fileChunks)
		}
	}

	if err := stream.CloseSend(); err != nil {
		c.logDebug("close upload stream failed: %v", err)
		return "", fmt.Errorf("failed to close send: %w", err)
	}
	c.logDebug("upload stream closed, waiting for final response")

	resp, err := stream.Recv()
	if err != nil {
		c.logDebug("receive upload final response failed: %v", err)
		return "", fmt.Errorf("failed to receive final response: %w", err)
	}

	result := resp.GetResult()
	if result == nil {
		return "", fmt.Errorf("unexpected response type: result is nil")
	}

	if !result.GetStatus() {
		c.logDebug("upload finished with server failure: %s", result.GetMessage())
		return "", fmt.Errorf("upload failed: %s", result.GetMessage())
	}

	elapsed := time.Since(startTime)
	speed := float64(totalSent) / elapsed.Seconds()
	speedStr := common.FormatSpeed(speed)

	log.Printf("[Upload] Complete: %s, elapsed=%d, speed=%s (sent: %d bytes)\n", fileName, elapsed.Milliseconds(), speedStr, totalSent)
	log.Printf("[Upload] Success: %s\n", result.GetMessage())
	c.logDebug("upload completed successfully: file=%s elapsed_ms=%d sent=%d", fileName, elapsed.Milliseconds(), totalSent)
	return result.GetMessage(), nil
}

func (c *ClientBasic) DownloadFile(fileTag, fileName, savePath string) (string, error) {
	ok, err := common.FileIsExist(savePath, fileTag, fileName, "")

	if err != nil {
		return "", fmt.Errorf("check file exist failed: %w", err)
	} else if ok {
		log.Printf("[Download] File already exists: %s\n", fileName)
		c.logDebug("download skipped because target exists: tag=%s name=%s save_path=%s", fileTag, fileName, savePath)
		return "", fmt.Errorf("file already exists: %s", fileName)
	}

	client, err := c.connect()
	if err != nil {
		return "", err
	}
	defer c.close()

	log.Printf("[Download] Request: tag=%s, name=%s, chunksize=%d\n", fileTag, fileName, c.Chunksize)
	c.logDebug("download request prepared: tag=%s name=%s save_path=%s", fileTag, fileName, savePath)

	downloadReq := &transferv1.DownloadFileRequest{}
	downloadReq.SetTag(fileTag)
	downloadReq.SetName(fileName)
	downloadReq.SetChunksize(int64(c.Chunksize))

	stream, err := client.DownloadFile(c.ctx, downloadReq)
	if err != nil {
		c.logDebug("failed to create download stream: %v", err)
		return "", fmt.Errorf("failed to create download stream: %w", err)
	}
	c.logDebug("download stream created")

	// 1. 接收元数据
	resp, err := stream.Recv()
	if err != nil {
		c.logDebug("failed to receive download metadata: %v", err)
		return "", fmt.Errorf("failed to receive metadata: %w", err)
	}

	// 处理响应
	if !resp.HasPayload() {
		return "", fmt.Errorf("unexpected response type: payload is nil")
	}

	metadata := resp.GetMetadata()
	if metadata == nil {
		if result := resp.GetResult(); result != nil {
			return "", fmt.Errorf("server error: %s", result.GetMessage())
		}
		return "", fmt.Errorf("missing metadata")
	}

	fileSize := metadata.GetSize()
	fileChunks := metadata.GetChunks()
	fileHash := metadata.GetHash()

	log.Printf("[Download] Metadata: size=%d, chunks=%d x %d Byte, hash=%s\n",
		fileSize, fileChunks, metadata.GetChunksize(), fileHash)
	c.logDebug("download metadata received: chunks=%d chunksize=%d hash=%s", fileChunks, metadata.GetChunksize(), fileHash)

	dstFilePath, err := common.SetTargetFilePath(savePath, fileTag, fileName)
	if err != nil {
		return "", fmt.Errorf("create target file path failed: %w", err)
	}
	c.logDebug("download target path resolved: %s", dstFilePath)

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

		respChan := make(chan *transferv1.DownloadFileResponse, 1)
		errChan := make(chan error, 1)

		go func() {
			resp, err := stream.Recv()
			if err != nil {
				errChan <- err
				return
			}
			respChan <- resp
		}()

		var resp *transferv1.DownloadFileResponse
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
			c.logDebug("download chunk receive timeout after %ds, removed partial file=%s", c.ChunkTimeout, recFilePath)
			return "", fmt.Errorf("chunk receive timeout after %ds", c.ChunkTimeout)
		case err = <-errChan:
			chunkCancel()
			if err == io.EOF {
				c.logDebug("download stream reached EOF")
				break
			}
			if bufWriter != nil {
				_ = bufWriter.Flush()
			}
			if recFile != nil {
				_ = recFile.Close()
			}
			_ = os.Remove(recFilePath)
			c.logDebug("failed to receive download chunk: %v, removed partial file=%s", err, recFilePath)
			return "", fmt.Errorf("failed to receive chunk: %w", err)
		case resp = <-respChan:
			chunkCancel()
		}

		if err == io.EOF {
			break
		}

		// 检查是否是结果消息
		if result := resp.GetResult(); result != nil {
			if !result.GetStatus() {
				if bufWriter != nil {
					_ = bufWriter.Flush()
				}
				if recFile != nil {
					_ = recFile.Close()
				}
				os.Remove(recFilePath)
				c.logDebug("download failed by server result: %s, removed partial file=%s", result.GetMessage(), recFilePath)
				return "", fmt.Errorf("download failed: %s", result.GetMessage())
			}
			log.Printf("[Download] Server confirmed: %s\n", result.GetMessage())
			c.logDebug("download server result received: %s", result.GetMessage())
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
		if c.shouldLogChunk(chunk.GetChunk(), fileChunks) {
			c.logDebug("received file chunk=%d/%d bytes=%d", chunk.GetChunk(), fileChunks, len(data))
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
			c.logDebug("failed to write download chunk=%d: %v, removed partial file=%s", chunk.GetChunk(), err, recFilePath)
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
		c.logDebug("flush download file failed: %v, removed partial file=%s", err, recFilePath)
		return "", fmt.Errorf("failed to flush: %w", err)
	}

	if err := recFile.Sync(); err != nil {
		if recFile != nil {
			_ = recFile.Close()
		}
		_ = os.Remove(recFilePath)
		c.logDebug("sync download file failed: %v, removed partial file=%s", err, recFilePath)
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
		c.logDebug("download validation failed: %v, removed partial file=%s", err, recFilePath)
		return "", fmt.Errorf("validation error: %w", err)
	}

	elapsed := time.Since(startTime)
	speed := float64(totalReceived) / elapsed.Seconds()
	speedStr := common.FormatSpeed(speed)

	log.Printf("[Download] Complete: %s, elapsed: %d, received=%d bytes, speed=%s\n", fileName, elapsed.Milliseconds(), totalReceived, speedStr)
	log.Printf("[Download] Saved to: %s\n", recFilePath)
	c.logDebug("download completed successfully: file=%s elapsed_ms=%d received=%d", fileName, elapsed.Milliseconds(), totalReceived)

	return recFilePath, nil
}

func (c *ClientBasic) ListFiles(fileTag string) ([]*transferv1.ListFileItem, error) {
	client, err := c.connect()
	if err != nil {
		return nil, err
	}
	defer c.close()

	checkCtx, checkCancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer checkCancel()

	listReq := &transferv1.ListFilesRequest{}
	listReq.SetTag(fileTag)
	c.logDebug("listing files for tag=%s", fileTag)
	response, err := client.ListFiles(checkCtx, listReq)
	if err != nil {
		c.logDebug("list files request failed: %v", err)
		return nil, err
	}

	if response.GetStatus() {
		c.logDebug("list files succeeded: count=%d", len(response.GetFiles()))
		return response.GetFiles(), nil

	}

	c.logDebug("list files rejected: %s", response.GetMessage())
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

	c.logDebug("virtual hash input generated: size=%d buffer=%d", fileSize, c.Chunksize)
	return common.CalcBlake3FromBytes(data)
}
