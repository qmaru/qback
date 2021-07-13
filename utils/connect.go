package utils

import (
	"log"
)

// ListenOrConnect 设置服务端/客户端地址
func ListenOrConnect(address string, isServer bool) string {
	if address == "" {
		address = "127.0.0.1:20000"
	}

	if isServer {
		log.Printf("Listening on: %s\n", address)
	} else {
		log.Printf("Connecting to: %s\n", address)
	}
	return address
}
