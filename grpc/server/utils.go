package server

import (
	"fmt"
	"os"

	"qback/grpc/common"
	pb "qback/grpc/libs"
	"qback/utils"
)

func GetFileList(savePath, fileTag string) ([]*pb.FileItem, error) {
	if savePath == "" || fileTag == "" {
		return nil, fmt.Errorf("savePath or fileTag is empty")
	}

	dstFolder := utils.FileSuite.JoinPath(savePath, fileTag)

	if !utils.FileSuite.Exists(dstFolder) {
		return nil, fmt.Errorf("folder not exists")
	}

	files, err := os.ReadDir(dstFolder)
	if err != nil {
		return nil, fmt.Errorf("read dir failed: %w", err)
	}

	var fileList []*pb.FileItem
	for _, file := range files {
		if !file.IsDir() {
			name := file.Name()
			info, err := file.Info()
			if err != nil {
				return nil, fmt.Errorf("get file info failed: %w", err)
			}
			size := info.Size()

			filePath := utils.FileSuite.JoinPath(dstFolder, name)
			hash, err := common.CalcBlake3(filePath)
			if err != nil {
				return nil, fmt.Errorf("calc file hash failed: %w", err)
			}

			fileList = append(fileList, &pb.FileItem{
				Name:   name,
				Size:   size,
				Hash:   hash,
				Chunks: 0,
			})
		}
	}

	return fileList, nil
}

// SetDstFilePath 设置文件路径
func SetDstFilePath(savePath, fileTag, fileName string) (string, error) {
	if savePath == "" || fileTag == "" || fileName == "" {
		return "", fmt.Errorf("savePath, fileTag, fileName is empty")
	}

	dstFolder := utils.FileSuite.JoinPath(savePath, fileTag)

	if !utils.FileSuite.Exists(dstFolder) {
		if _, err := utils.FileSuite.Mkdir(dstFolder); err != nil {
			return "", fmt.Errorf("create folder failed: %w", err)
		}
	}

	return utils.FileSuite.JoinPath(dstFolder, fileName), nil
}

// FileIsExist 检查发送的文件存在
func FileIsExist(savePath, fileTag, fileName, fileHash string) (bool, error) {
	dstFile, err := SetDstFilePath(savePath, fileTag, fileName)
	if err != nil {
		return false, fmt.Errorf("get filepath failed")
	}

	if !utils.FileSuite.Exists(dstFile) {
		return false, nil
	}

	// 检查哈希值
	currentHash, err := common.CalcBlake3(dstFile)
	if err != nil {
		return false, err
	}

	// 哈希值为空则文件不存在
	if currentHash == "" {
		return false, nil
	}

	// 文件存在但哈希值不同则重传
	if currentHash != fileHash {
		if err := os.Remove(dstFile); err != nil {
			return false, fmt.Errorf("remove file failed: %w", err)
		}
		return false, nil
	}

	return true, nil
}

// CreateSaveFile 设置文件保存信息
func CreateSaveFile(dstFilePath string) (*os.File, error) {
	recFile, err := os.OpenFile(dstFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("create file failed: %w", err)
	}

	return recFile, nil
}
