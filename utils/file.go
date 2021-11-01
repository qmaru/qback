package utils

import (
	"os"
	"path/filepath"
)

// SetSavePath 设置文件路径
func SetSavePath(dirPath, filetag, filename string) string {
	var root string
	if dirPath != "" {
		root = dirPath
	} else {
		root, _ = os.Getwd()
	}
	savePath := filepath.Join(root, filetag)
	savefile := filepath.Join(savePath, filename)
	_, err := os.Stat(savePath)
	if err == nil {
		return savefile
	}
	if os.IsNotExist(err) {
		_ = os.Mkdir(savePath, os.ModePerm)
		return savefile
	}

	return savefile
}

// CheckFile 检查发送的文件存在
func CheckFile(dirPath, filetag, filename, filehash string) bool {
	savefile := SetSavePath(dirPath, filetag, filename)
	// 检查哈希值
	oldFilehash := CalcMD5(savefile)
	// 哈希值为空则文件不存在
	if oldFilehash == "" {
		return false
	}
	// 文件存在但哈希值不同则重传
	if oldFilehash != filehash {
		os.Remove(savefile)
		return false
	}

	_, err := os.Stat(savefile)
	return err == nil
}

// CreateSaveFile 设置文件保存信息
func CreateSaveFile(dirPath, filetag, filename string) (*os.File, error) {
	folder := filepath.Join(dirPath, filetag)
	err := os.MkdirAll(folder, 0755)
	if err != nil {
		return nil, err
	}

	savefile := SetSavePath(dirPath, filetag, filename)
	recFile, err := os.OpenFile(savefile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return recFile, nil
}
