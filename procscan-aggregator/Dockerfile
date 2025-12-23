# 多阶段构建 Dockerfile for ProcScan Aggregator

# 第一阶段：构建
FROM golang:1.24-alpine AS builder

WORKDIR /build

# 安装构建依赖
RUN apk add --no-cache git make

# 复制 go mod 文件
COPY go.mod go.sum* ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建二进制文件
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o procscan-aggregator ./cmd/aggregator

# 第二阶段：运行
FROM alpine:latest

# 安装 CA 证书（用于 HTTPS 请求）
RUN apk --no-cache add ca-certificates

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/procscan-aggregator .

# 复制配置文件
COPY config.yaml .

# 暴露端口
EXPOSE 8090

# 运行
ENTRYPOINT ["/app/procscan-aggregator"]
CMD ["-config", "/app/config.yaml"]
