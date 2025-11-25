# Makefile for kubectl-block CLI

# Variables
BINARY_NAME=kubectl-block
MAIN_FILE=main_simple.go
BUILD_DIR=build
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.2.0-alpha")
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "✅ Build completed: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for multiple platforms
.PHONY: build-all
build-all:
	@echo "🔨 Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)

	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_FILE)

	# Linux ARM64
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_FILE)

	# Darwin AMD64
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_FILE)

	# Darwin ARM64
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_FILE)

	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_FILE)

	@echo "✅ Multi-platform build completed in $(BUILD_DIR)/"

# Install to /usr/local/bin
.PHONY: install
install: build
	@echo "📦 Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "✅ Installation completed!"

# Test the CLI
.PHONY: test
test: build
	@echo "🧪 Testing $(BINARY_NAME)..."
	@echo "Testing help command..."
	./$(BUILD_DIR)/$(BINARY_NAME) --help
	@echo ""
	@echo "Testing lock command..."
	./$(BUILD_DIR)/$(BINARY_NAME) lock --help
	@echo ""
	@echo "✅ All tests passed!"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY_NAME)
	@echo "✅ Clean completed!"

# Run demo
.PHONY: demo
demo: build
	@echo "🎬 Running demo..."
	./demo.sh

# Format Go code
.PHONY: fmt
fmt:
	@echo "📝 Formatting Go code..."
	go fmt ./...
	@echo "✅ Code formatted!"

# Run Go vet
.PHONY: vet
vet:
	@echo "🔍 Running go vet..."
	go vet ./...
	@echo "✅ Vet completed!"

# Build release artifacts
.PHONY: release
release: clean fmt vet build-all
	@echo "📦 Creating release artifacts..."
	@mkdir -p $(BUILD_DIR)/release
	@cd $(BUILD_DIR) && \
		for binary in $(BINARY_NAME)-*; do \
			if [[ $$binary == *.exe ]]; then \
				zip "release/$${binary%.exe}.zip" $$binary; \
			else \
				tar -czf "release/$$binary.tar.gz" $$binary; \
			fi; \
		done
	@echo "✅ Release artifacts created in $(BUILD_DIR)/release/"

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build      - Build the binary for current platform"
	@echo "  build-all  - Build for all supported platforms"
	@echo "  install    - Install to /usr/local/bin"
	@echo "  test       - Run basic tests"
	@echo "  demo       - Run demo script"
	@echo "  clean      - Clean build artifacts"
	@echo "  fmt        - Format Go code"
	@echo "  vet        - Run go vet"
	@echo "  release    - Create release artifacts"
	@echo "  help       - Show this help"