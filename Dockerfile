FROM golang:1.23-alpine AS builder

RUN apk add upx ca-certificates tzdata

WORKDIR /usr/src

COPY . /usr/src

RUN go mod download
RUN gover=`go version | awk '{print $3,$4}'` \
    && sed -i "s#COMMIT_GOVER#$gover#g" cmd/version.go \
    &&CGO_ENABLED=0 go build -ldflags="-s -w -extldflags='static'" -trimpath -o app \
    && upx --best --lzma app

FROM scratch AS prod

COPY --from=builder /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/src/app /qBack

ENTRYPOINT ["/qBack"]
