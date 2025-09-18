#!/bin/bash

# Docker Tool 重启脚本
# 用于重启docker-tool程序

set -e

# 获取脚本所在目录的父目录（项目根目录）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=========================================="
echo "Docker Tool 重启脚本"
echo "=========================================="
echo "项目目录: $PROJECT_DIR"
echo "重启时间: $(date)"
echo ""

# 进入项目目录
cd "$PROJECT_DIR"

# 停止程序
echo "正在停止 Docker Tool..."
"$SCRIPT_DIR/stop.sh"

# 等待一下确保程序完全停止
echo ""
echo "等待程序完全停止..."
sleep 3

# 启动程序
echo ""
echo "正在启动 Docker Tool..."
"$SCRIPT_DIR/start.sh"

echo ""
echo "=========================================="
echo "重启完成！"
echo "=========================================="