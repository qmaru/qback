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

// CheckFile 检查文件存在
func CheckFile(dirPath, filetag, filename, filehash string) bool {
	savefile := SetSavePath(dirPath, filetag, filename)
	oldFilehash := CalcMD5(savefile)
	if oldFilehash != filehash {
		os.Remove(savefile)
		return false
	}
	_, err := os.Stat(savefile)
	return err == nil
}

// CreateSaveFile 设置文件保存信息
func CreateSaveFile(dirPath, filetag, filename string) *os.File {
	savefile := SetSavePath(dirPath, filetag, filename)
	recFile, err := os.OpenFile(savefile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		PrintError("Create File Error", err)
	}
	return recFile
}
