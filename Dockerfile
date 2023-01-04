FROM golang:alpine AS builder

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOPROXY="https://goproxy.cn,direct"

WORKDIR /build

COPY . .

RUN go build -o app server/server.go

FROM scratch

COPY ./etc /etc
COPY ./logs /logs

COPY --from=builder /build/app .

EXPOSE 7668

ENTRYPOINT ["/app"]
