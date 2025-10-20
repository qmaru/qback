# qback

基于 gRPC 的文件传输工具，服务端和客户端二合一，支持证书验证。

## command

```go
qback is a File Transfer Service

Usage:
  qback [flags]
  qback [command]

Available Commands:
  client      Run Client
  help        Help about any command
  server      Run Server

Flags:
  -a, --address string   Server Address (default "127.0.0.1:20000")
      --debug            Debug mode
  -h, --help             help for qback
  -s, --secure           With TLS
  -v, --version          version for qback

Use "qback [command] --help" for more information about a command.
```

## container

```shell
docker run --rm \
    -v /path/download:/download \
    -p 20000:20000 \
    ghcr.io/qmaru/qback server -a 0.0.0.0:20000 -d /download
```

```shell
docker run --rm \
    ghcr.io/qmaru/qback client ping -a 172.17.0.1:20000
```
