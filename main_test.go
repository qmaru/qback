package main

import (
	"context"
	"testing"
	"time"

	"qback/grpc/client"
	"qback/grpc/server"
)

const listenAddr = "127.0.0.1:50051"

func runServer() func() {
	qServer := server.ServerBasic{
		ListenAddress: listenAddr,
		MemoryMode:    true,
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		qServer.Run(ctx)
	}()

	return cancel
}

func TestServer(t *testing.T) {
	stopServer := runServer()
	defer stopServer()

	qClient := client.ClientBasic{
		ServerAddress: listenAddr,
	}

	timeout := time.After(5 * time.Second)
	tick := time.Tick(100 * time.Millisecond)
	for {
		select {
		case <-timeout:
			t.Fatal("server did not start in time")
		case <-tick:
			if err := qClient.ServerCheck(); err == nil {
				t.Logf("Server is healthy")
				return
			}
		}
	}
}
