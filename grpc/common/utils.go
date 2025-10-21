package common

import (
	"fmt"
	"os"

	pb "qback/grpc/libs"
	"qback/utils"
)

type FileValidationInfo struct {
	Data         []byte
	FilePath     string
	ExpectedSize int64
	ExpectedHash string
	IsMemory     bool
}

type FileMode int

const (
	FileRead FileMode = iota
	FileWrite
	FileReadWrite
)

func GetFileList(savePath, fileTag string) ([]*pb.ListFileItem, error) {
	if savePath == "" || fileTag == "" {
		return nil, fmt.Errorf("savePath or fileTag is empty")
	}

	targetFolder := utils.FileSuite.JoinPath(savePath, fileTag)

	if !utils.FileSuite.Exists(targetFolder) {
		return nil, fmt.Errorf("folder not exists")
	}

	files, err := os.ReadDir(targetFolder)
	if err != nil {
		return nil, fmt.Errorf("read dir failed: %w", err)
	}

	var fileList []*pb.ListFileItem
	for _, file := range files {
		if !file.IsDir() {
			name := file.Name()
			info, err := file.Info()
			if err != nil {
				return nil, fmt.Errorf("get file info failed: %w", err)
			}
			size := info.Size()

			filePath := utils.FileSuite.JoinPath(targetFolder, name)
			hash, err := CalcBlake3(filePath)
			if err != nil {
				return nil, fmt.Errorf("calc file hash failed: %w", err)
			}

			mt := info.ModTime()

			fileList = append(fileList, &pb.ListFileItem{
				Name:         name,
				Size:         size,
				Hash:         hash,
				ModifiedTime: mt.Unix(),
			})
		}
	}

	return fileList, nil
}

// SetTargetFilePath 设置文件路径
func SetTargetFilePath(savePath, fileTag, fileName string) (string, error) {
	if savePath == "" || fileTag == "" || fileName == "" {
		return "", fmt.Errorf("savePath, fileTag, fileName is empty")
	}

	targetFolder := utils.FileSuite.JoinPath(savePath, fileTag)

	if !utils.FileSuite.Exists(targetFolder) {
		if _, err := utils.FileSuite.Mkdir(targetFolder); err != nil {
			return "", fmt.Errorf("create folder failed: %w", err)
		}
	}

	targetFilePath := utils.FileSuite.JoinPath(targetFolder, fileName)
	return targetFilePath, nil
}

// FileIsExist 检查发送的文件存在
func FileIsExist(savePath, fileTag, fileName, fileHash string) (bool, error) {
	targetFile := utils.FileSuite.JoinPath(savePath, fileTag, fileName)
	if !utils.FileSuite.Exists(targetFile) {
		return false, nil
	}

	if fileHash != "" {
		// 检查哈希值
		currentHash, err := CalcBlake3(targetFile)
		if err != nil {
			return false, err
		}

		// 哈希值为空则文件不存在
		if currentHash == "" {
			return false, nil
		}

		// 文件存在但哈希值不同则重传
		if currentHash != fileHash {
			if err := os.Remove(targetFile); err != nil {
				return false, fmt.Errorf("remove file failed: %w", err)
			}
			return false, nil
		}
	}

	return true, nil
}

// OpenTargetFile 设置文件保存信息
func OpenTargetFile(targetFilePath string, mode FileMode) (*os.File, error) {
	var fileMode int
	switch mode {
	case FileRead:
		fileMode = os.O_RDONLY
	case FileWrite:
		fileMode = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	case FileReadWrite:
		fileMode = os.O_RDWR | os.O_CREATE
	}

	recFile, err := os.OpenFile(targetFilePath, fileMode, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", targetFilePath, err)
	}

	return recFile, nil
}

func ValidateFileIntegrity(info FileValidationInfo) error {
	var recHash string
	var actualSize int64
	var err error

	if info.IsMemory {
		recHash, err = CalcBlake3FromBytes(info.Data)
		actualSize = int64(len(info.Data))
	} else {
		recHash, err = CalcBlake3(info.FilePath)
		if err == nil {
			fileInfo, err := os.Stat(info.FilePath)
			if err != nil {
				return fmt.Errorf("stat file error: %v", err)
			}
			actualSize = fileInfo.Size()
		}
	}

	if err != nil {
		return fmt.Errorf("hash calculation error: %v", err)
	}

	if actualSize != info.ExpectedSize {
		return fmt.Errorf("size mismatch: expected=%d got=%d", info.ExpectedSize, actualSize)
	}

	if recHash != info.ExpectedHash {
		return fmt.Errorf("hash mismatch: expected=%s got=%s", info.ExpectedHash, recHash)
	}

	return nil
}
