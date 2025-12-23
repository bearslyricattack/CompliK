# Makefile for ProcScan Aggregator

.PHONY: build clean run docker-build docker-push test help

# 变量定义
APP_NAME := procscan-aggregator
DOCKER_IMAGE := $(APP_NAME):latest
DOCKER_REGISTRY := your-registry.com
BINARY := bin/$(APP_NAME)

# 构建二进制文件
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p bin
	go build -o $(BINARY) ./cmd/aggregator

# 清理构建文件
clean:
	@echo "Cleaning..."
	@rm -rf bin/

# 本地运行
run: build
	@echo "Running $(APP_NAME)..."
	$(BINARY) -config ./config.yaml

# 构建 Docker 镜像
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .

# 推送 Docker 镜像
docker-push: docker-build
	@echo "Pushing Docker image..."
	docker tag $(DOCKER_IMAGE) $(DOCKER_REGISTRY)/$(DOCKER_IMAGE)
	docker push $(DOCKER_REGISTRY)/$(DOCKER_IMAGE)

# 运行测试
test:
	@echo "Running tests..."
	go test -v ./...

# 格式化代码
fmt:
	@echo "Formatting code..."
	go fmt ./...

# 代码检查
lint:
	@echo "Running linter..."
	golangci-lint run

# 帮助信息
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  clean        - Clean build artifacts"
	@echo "  run          - Build and run locally"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-push  - Build and push Docker image"
	@echo "  test         - Run tests"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linter"
	@echo "  help         - Show this help message"
