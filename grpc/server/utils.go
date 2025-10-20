package server

import (
	"fmt"
	"os"

	"qback/grpc/common"
	"qback/utils"
)

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
