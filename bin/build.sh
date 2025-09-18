#!/bin/bash

# Docker Tool 构建脚本
# 用于编译docker-tool程序

set -e

# 获取脚本所在目录的父目录（项目根目录）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=========================================="
echo "Docker Tool 构建脚本"
echo "=========================================="
echo "项目目录: $PROJECT_DIR"
echo "构建时间: $(date)"
echo ""

# 进入项目目录
cd "$PROJECT_DIR"

# 检查Go环境
if ! command -v go &> /dev/null; then
    echo "错误: 未找到Go环境，请先安装Go"
    exit 1
fi

echo "Go版本: $(go version)"
echo ""

# 清理旧的构建文件
echo "清理旧的构建文件..."
rm -f docker-tool
echo "✓ 清理完成"
echo ""

# 下载依赖
echo "下载Go模块依赖..."
go mod tidy
echo "✓ 依赖下载完成"
echo ""

# 构建程序
echo "开始构建程序..."
go build -o docker-tool -ldflags="-s -w" .
echo "✓ 构建完成"
echo ""

# 检查构建结果
if [ -f "docker-tool" ]; then
    echo "构建成功！"
    echo "可执行文件: $(pwd)/docker-tool"
    echo "文件大小: $(du -h docker-tool | cut -f1)"
    echo ""
    echo "使用方法:"
    echo "  ./docker-tool -config config.yaml"
    echo "  ./bin/start.sh"
else
    echo "错误: 构建失败，未找到可执行文件"
    exit 1
fi

echo "=========================================="
echo "构建完成！"
echo "=========================================="
