#!/bin/bash
# 开发环境初始化脚本

set -e

echo "🚀 初始化 migra-go 开发环境..."

# 检查 Go 版本
if ! command -v go &> /dev/null; then
    echo "❌ 未找到 Go，请先安装 Go 1.21+"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}')
echo "✅ Go 版本: $GO_VERSION"

# 下载依赖
echo "📦 下载 Go 依赖..."
go mod download

# 安装开发工具
echo "🔧 安装开发工具..."

# golangci-lint
if ! command -v golangci-lint &> /dev/null; then
    echo "  安装 golangci-lint..."
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
else
    echo "  ✅ golangci-lint 已安装"
fi

# 构建项目
echo "🔨 构建项目..."
make build

echo ""
echo "✅ 开发环境初始化完成！"
echo ""
echo "常用命令："
echo "  make build    - 构建项目"
echo "  make test     - 运行测试"
echo "  make lint     - 代码检查"
echo "  make fmt      - 格式化代码"
echo "  make ci       - 运行 CI 检查"
