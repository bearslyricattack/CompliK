#!/bin/bash

# Build script for kubectl-block CLI tool

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 项目信息
PROJECT_NAME="kubectl-block"
VERSION=${VERSION:-"v0.2.0-alpha"}
BUILD_DIR="build"
BINARY_NAME="kubectl-block"

# 函数
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 清理函数
cleanup() {
    print_info "Cleaning up..."
    rm -rf $BUILD_DIR
}

# 检查依赖
check_dependencies() {
    print_info "Checking dependencies..."

    if ! command -v go &> /dev/null; then
        print_error "Go is not installed or not in PATH"
        exit 1
    fi

    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    print_info "Go version: $GO_VERSION"

    if [[ $(echo "$GO_VERSION" | cut -d. -f1) -lt "1" || \
          $(echo "$GO_VERSION" | cut -d. -f2) -lt "24" ]]; then
        print_error "Go 1.24+ is required"
        exit 1
    fi
}

# 构建函数
build_binary() {
    print_info "Building $PROJECT_NAME..."

    # 创建构建目录
    mkdir -p $BUILD_DIR

    # 设置构建变量
    export CGO_ENABLED=0
    export GOOS=${GOOS:-$(go env GOOS)}
    export GOARCH=${GOARCH:-$(go env GOARCH)}

    # 构建信息
    BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    GIT_BRANCH=$(git branch --show-current 2>/dev/null || echo "unknown")

    # 构建 ldflags
    LDFLAGS="-w -s"
    LDFLAGS="$LDFLAGS -X main.version=$VERSION"
    LDFLAGS="$LDFLAGS -X main.buildTime=$BUILD_TIME"
    LDFLAGS="$LDFLAGS -X main.gitCommit=$GIT_COMMIT"
    LDFLAGS="$LDFLAGS -X main.gitBranch=$GIT_BRANCH"

    # 构建二进制
    go build -o $BUILD_DIR/$BINARY_NAME \
        -ldflags "$LDFLAGS" \
        ./cmd/kubectl-block

    print_info "Binary built: $BUILD_DIR/$BINARY_NAME"
}

# 跨平台构建
build_multi_platform() {
    print_info "Building for multiple platforms..."

    platforms=(
        "linux/amd64"
        "linux/arm64"
        "darwin/amd64"
        "darwin/arm64"
        "windows/amd64"
    )

    for platform in "${platforms[@]}"; do
        GOOS=$(echo $platform | cut -d'/' -f1)
        GOARCH=$(echo $platform | cut -d'/' -f2)

        OUTPUT_NAME="$BINARY_NAME-$GOOS-$GOARCH"
        if [ "$GOOS" = "windows" ]; then
            OUTPUT_NAME="$OUTPUT_NAME.exe"
        fi

        print_info "Building for $GOOS/$GOARCH..."

        CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH \
        go build -o $BUILD_DIR/$OUTPUT_NAME \
            -ldflags "-w -s -X main.version=$VERSION" \
            ./cmd/kubectl-block

        print_info "Built: $BUILD_DIR/$OUTPUT_NAME"
    done
}

# 打包函数
package_release() {
    print_info "Packaging release..."

    RELEASE_DIR="$BUILD_DIR/release"
    mkdir -p $RELEASE_DIR

    # 复制二进制文件
    cp $BUILD_DIR/$BINARY_NAME $RELEASE_DIR/

    # 创建版本信息文件
    cat > $RELEASE_DIR/VERSION << EOF
$PROJECT_NAME
Version: $VERSION
Build Time: $(date -u '+%Y-%m-%dT%H:%M:%SZ')
Git Commit: $(git rev-parse HEAD 2>/dev/null || echo "unknown")
Platform: $(go env GOOS)/$(go env GOARCH)
EOF

    # 创建安装脚本
    cat > $RELEASE_DIR/install.sh << 'EOF'
#!/bin/bash

# kubectl-block 安装脚本

set -e

INSTALL_DIR="${HOME}/.local/bin"
BINARY_NAME="kubectl-block"

# 创建安装目录
mkdir -p "$INSTALL_DIR"

# 复制二进制文件
cp "$BINARY_NAME" "$INSTALL_DIR/"

# 检查 PATH
if echo "$PATH" | grep -q "$INSTALL_DIR"; then
    echo "✅ $BINARY_NAME installed successfully!"
    echo "Run: kubectl block --help"
else
    echo "⚠️  Please add $INSTALL_DIR to your PATH:"
    echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
    echo ""
    echo "Or run: sudo cp $BINARY_NAME /usr/local/bin/"
fi
EOF
    chmod +x $RELEASE_DIR/install.sh

    # 创建 README
    cat > $RELEASE_DIR/README.md << EOF
# kubectl-block v$VERSION

Block Controller CLI tool for managing Kubernetes namespace lifecycle.

## Quick Install

1. Extract the archive
2. Run the install script:
   \`\`\`
   ./install.sh
   \`\`\`

## Usage

\`\`\`
kubectl block lock my-namespace --duration=24h --reason="Maintenance"
kubectl block unlock my-namespace
kubectl block status --all
kubectl block report
\`\`\`

## Documentation

For more information, see the [project documentation](https://github.com/gitlayzer/block-controller).
EOF

    # 创建压缩包
    cd $BUILD_DIR
    tar -czf "kubectl-block-${VERSION}.tar.gz" -C release .

    print_info "Release packaged: $BUILD_DIR/kubectl-block-${VERSION}.tar.gz"
}

# 测试函数
run_tests() {
    print_info "Running tests..."

    # 运行单元测试
    if [ -f "./cmd/kubectl-block/main_test.go" ] || [ -d "./cmd/kubectl-block/test" ]; then
        go test ./cmd/kubectl-block/...
    else
        print_warn "No tests found for CLI"
    fi

    # 基本功能测试
    if [ -f "$BUILD_DIR/$BINARY_NAME" ]; then
        print_info "Testing binary..."
        $BUILD_DIR/$BINARY_NAME --help > /dev/null
        print_info "Binary works correctly!"
    fi
}

# 清理函数
clean_build() {
    print_info "Cleaning build artifacts..."
    rm -rf $BUILD_DIR
}

# 显示帮助
show_help() {
    cat << EOF
kubectl-block Build Script

Usage: $0 [command] [options]

Commands:
    build           Build the CLI binary (default)
    multi-platform  Build for multiple platforms
    package         Package release files
    test            Run tests
    clean           Clean build artifacts
    help            Show this help message

Environment Variables:
    VERSION         Version string (default: v0.2.0-alpha)
    GOOS            Target OS (default: current OS)
    GOARCH          Target Architecture (default: current ARCH)

Examples:
    $0                          # Build for current platform
    $0 build                   # Same as above
    $0 multi-platform          # Build for all platforms
    $0 package                  # Package release files
    VERSION=v0.2.0 $0 build      # Build with custom version

    # Cross-platform builds
    GOOS=linux GOARCH=amd64 $0  # Build for Linux AMD64
    GOOS=darwin GOARCH=arm64 $0 # Build for macOS ARM64
EOF
}

# 主逻辑
main() {
    local command=${1:-"build"}

    case $command in
        "build")
            cleanup
            check_dependencies
            build_binary
            run_tests
            ;;
        "multi-platform")
            cleanup
            check_dependencies
            build_multi_platform
            ;;
        "package")
            if [ ! -f "$BUILD_DIR/$BINARY_NAME" ]; then
                print_warn "Binary not found, building first..."
                cleanup
                check_dependencies
                build_binary
            fi
            package_release
            ;;
        "test")
            run_tests
            ;;
        "clean")
            clean_build
            ;;
        "help"|"-h"|"--help")
            show_help
            ;;
        *)
            print_error "Unknown command: $command"
            show_help
            exit 1
            ;;
    esac

    print_info "Build completed successfully!"
}

# 信号处理
trap cleanup EXIT

# 运行主函数
main "$@"