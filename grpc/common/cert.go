package common

import (
	"crypto/tls"
	"crypto/x509"
	"os"

	"qback/configs"
)

func GenTLSInfo(debug bool, certType string) (*tls.Config, *x509.CertPool, error) {
	// 读取指定证书
	cert, key, err := configs.ReadCertsCfg(debug, certType)
	if err != nil {
		return nil, nil, err
	}
	// 读取 CA 证书
	caCert, _, err := configs.ReadCertsCfg(debug, "ca")
	if err != nil {
		return nil, nil, err
	}
	// 设置证书信息
	tlsInfo, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, nil, err
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsInfo},
	}
	// 设置 CA 信息
	certPool := x509.NewCertPool()
	ca, err := os.ReadFile(caCert)
	if err != nil {
		return nil, nil, err
	}
	certPool.AppendCertsFromPEM(ca)
	return tlsConfig, certPool, nil
}
