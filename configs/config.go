package configs

import (
	"errors"
	"log"
	"path/filepath"
	"sync"

	"qback/utils"
)

var MustGetCertPath = sync.OnceValue(func() string {
	p, err := GetCertPath()
	if err != nil {
		log.Fatal(err)
	}
	return p
})

func GetCertPath() (string, error) {
	root, err := utils.FileSuite.RootPath("certs")
	if err != nil {
		return "", err
	}

	if !utils.FileSuite.Exists(root) {
		return "", errors.New("cert path not found")
	}

	return root, nil
}

// 读取证书信息
func ReadCertsCfg(debug bool, certType string) (string, string, error) {
	root := MustGetCertPath()

	switch certType {
	case "server":
		certFile := filepath.Join(root, "server.pem")
		keyFile := filepath.Join(root, "server.key")
		return certFile, keyFile, nil
	case "client":
		certFile := filepath.Join(root, "client.pem")
		keyFile := filepath.Join(root, "client.key")
		return certFile, keyFile, nil
	case "ca":
		certFile := filepath.Join(root, "ca.pem")
		keyFile := ""
		return certFile, keyFile, nil
	}
	return "", "", errors.New("cert type error")
}
