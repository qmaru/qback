package server

import (
	"os"
	"path/filepath"

	"qback/grpc/common"
)

// SetDstPath 设置文件路径
func SetDstPath(savePath, fileTag, fileName string) (string, error) {
	dstRoot := filepath.Join(savePath, fileTag)
	_, err := os.Stat(savePath)
	if err != nil {
		return "", err
	}
	if os.IsNotExist(err) {
		err = os.Mkdir(savePath, os.ModePerm)
		if err != nil {
			return "", err
		}
	}
	dstFile := filepath.Join(dstRoot, fileName)
	return dstFile, nil
}

// FileIsExist 检查发送的文件存在
func FileIsExist(savePath, fileTag, fileName, fileHash string) (bool, error) {
	dstFile, err := SetDstPath(savePath, fileTag, fileName)
	if err != nil {
		return false, err
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
		os.Remove(dstFile)
		return false, nil
	}

	_, err = os.Stat(dstFile)
	if err != nil {
		return false, err
	}
	return true, nil
}

// CreateSaveFile 设置文件保存信息
func CreateSaveFile(savePath, fileTag, fileName string) (*os.File, error) {
	dstRoot := filepath.Join(savePath, fileTag)
	err := os.MkdirAll(dstRoot, 0755)
	if err != nil {
		return nil, err
	}

	dstFile, err := SetDstPath(savePath, fileTag, fileName)
	if err != nil {
		return nil, err
	}
	recFile, err := os.OpenFile(dstFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return recFile, nil
}
