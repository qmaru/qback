package utils

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"log"
	"os"
	"path/filepath"
)

// PrintError 统一输出格式
func PrintError(msg string, err error) {
	log.Fatalf("[message]: %s [error]: %v", msg, err)
}

// CalcMD5 计算文件 MD5
func CalcMD5(filepath string) string {
	fData, err := os.Open(filepath)
	if err != nil {
		PrintError("Open File Error", err)
	}
	defer fData.Close()
	md5hash := md5.New()
	io.Copy(md5hash, fData)
	return hex.EncodeToString(md5hash.Sum(nil))
}

// 读取证书信息
func ReadCertsCfg(t string) (certFile, keyFile string) {
	var root string
	if Debug {
		root, _ = os.Getwd()
	} else {
		root, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	}
	certRoot := filepath.Join(root, "certs")
	_, err := os.Stat(certRoot)
	if err != nil {
		PrintError(certRoot+" Not Found", err)
	}
	switch t {
	case "server":
		certFile = filepath.Join(certRoot, "server.pem")
		keyFile = filepath.Join(certRoot, "server.key")
		return
	case "client":
		certFile = filepath.Join(certRoot, "client.pem")
		keyFile = filepath.Join(certRoot, "client.key")
		return
	case "ca":
		certFile = filepath.Join(certRoot, "ca.pem")
		keyFile = ""
		return
	}
	return "", ""
}
