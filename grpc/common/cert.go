package common

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"time"

	"qback/configs"
)

func getCertInfo(certType string) (*tls.Certificate, error) {
	cert, key, err := configs.ReadCertsCfg(certType)
	if err != nil {
		return nil, err
	}
	tlsInfo, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}
	return &tlsInfo, nil
}

func getCAPool() (*x509.CertPool, error) {
	certPool := x509.NewCertPool()

	caInfo, _, err := configs.ReadCertsCfg("ca")
	if err != nil {
		return nil, err
	}

	ca, err := os.ReadFile(caInfo)
	if err != nil {
		return nil, err
	}

	ok := certPool.AppendCertsFromPEM(ca)
	if !ok {
		return nil, fmt.Errorf("failed to append CA certs")
	}

	return certPool, nil
}

func tlsVersionString(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS1.0"
	case tls.VersionTLS11:
		return "TLS1.1"
	case tls.VersionTLS12:
		return "TLS1.2"
	case tls.VersionTLS13:
		return "TLS1.3"
	default:
		return fmt.Sprintf("0x%x", v)
	}
}

func cipherSuiteName(id uint16) string {
	switch id {
	case tls.TLS_AES_128_GCM_SHA256:
		return "TLS_AES_128_GCM_SHA256"
	case tls.TLS_AES_256_GCM_SHA384:
		return "TLS_AES_256_GCM_SHA384"
	case tls.TLS_CHACHA20_POLY1305_SHA256:
		return "TLS_CHACHA20_POLY1305_SHA256"
	case tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256:
		return "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"
	case tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:
		return "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
	case tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384:
		return "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"
	case tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:
		return "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
	default:
		return fmt.Sprintf("0x%x", id)
	}
}

func GenTLSInfo(certType string, mTLS bool) (*tls.Config, error) {
	// 设置证书信息
	certInfo, err := getCertInfo(certType)
	if err != nil {
		return nil, err
	}

	// 设置 CA 信息
	caPool, err := getCAPool()
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*certInfo},
		MinVersion:   tls.VersionTLS13,
	}

	switch certType {
	case "server":
		if mTLS {
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}
		tlsConfig.ClientCAs = caPool
	case "client":
		tlsConfig.RootCAs = caPool
	default:
		return nil, fmt.Errorf("unknown cert type: %s", certType)
	}

	return tlsConfig, nil
}

func ProbeTLSConnection(address string, tlsConfig *tls.Config) {
	probe, err := tls.Dial("tcp", address, tlsConfig)
	if err != nil {
		log.Printf("TLS probe failed: %v", err)
		return
	}
	defer probe.Close()
	state := probe.ConnectionState()

	ver := tlsVersionString(state.Version)
	cipher := cipherSuiteName(state.CipherSuite)
	curve := state.CurveID.String()

	var notAfter time.Time
	if len(state.PeerCertificates) > 0 {
		notAfter = state.PeerCertificates[0].NotAfter
	}
	var daysLeft string
	if !notAfter.IsZero() {
		d := time.Until(notAfter).Hours() / 24.0
		daysLeft = fmt.Sprintf("%.1f days", d)
	} else {
		daysLeft = "unknown"
	}

	log.Printf("TLS Connected: ServerName=%s, Version=%s, CipherSuite=%s, Curve=%s", state.ServerName, ver, cipher, curve)

	if !notAfter.IsZero() {
		log.Printf("TLS Cert: Subject=%s, Issuer=%s, NotBefore=%s, NotAfter=%s (left: %s)",
			state.PeerCertificates[0].Subject.CommonName,
			state.PeerCertificates[0].Issuer.CommonName,
			state.PeerCertificates[0].NotBefore.Format(time.RFC3339),
			notAfter.Format(time.RFC3339),
			daysLeft,
		)
	}
}
