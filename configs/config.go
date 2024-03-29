package configs

import (
	"errors"
	"os"
	"path/filepath"
)

func LocalPath(debug bool) (path string, err error) {
	if debug {
		path, err = os.Getwd()
		if err != nil {
			return "", err
		}
	} else {
		path, err = filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			return "", err
		}
	}
	return path, nil
}

// 读取证书信息
func ReadCertsCfg(debug bool, certType string) (string, string, error) {
	root, err := LocalPath(debug)
	if err != nil {
		return "", "", err
	}
	certRoot := filepath.Join(root, "certs")
	_, err = os.Stat(certRoot)
	if err != nil {
		return "", "", err
	}

	switch certType {
	case "server":
		certFile := filepath.Join(certRoot, "server.pem")
		keyFile := filepath.Join(certRoot, "server.key")
		return certFile, keyFile, nil
	case "client":
		certFile := filepath.Join(certRoot, "client.pem")
		keyFile := filepath.Join(certRoot, "client.key")
		return certFile, keyFile, nil
	case "ca":
		certFile := filepath.Join(certRoot, "ca.pem")
		keyFile := ""
		return certFile, keyFile, nil
	}
	return "", "", errors.New("cert type error")
}
