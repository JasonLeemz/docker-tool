#!/bin/bash

# Docker Tool 启动脚本
# 用于启动docker-tool程序

set -e

# 获取脚本所在目录的父目录（项目根目录）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=========================================="
echo "Docker Tool 启动脚本"
echo "=========================================="
echo "项目目录: $PROJECT_DIR"
echo "启动时间: $(date)"
echo ""

# 进入项目目录
cd "$PROJECT_DIR"

# 检查可执行文件是否存在
if [ ! -f "docker-tool" ]; then
    echo "错误: 未找到可执行文件 docker-tool"
    echo "请先运行构建脚本: ./bin/build.sh"
    exit 1
fi

# 检查配置文件是否存在
CONFIG_FILE="config.yaml"
if [ ! -f "$CONFIG_FILE" ]; then
    echo "错误: 未找到配置文件 $CONFIG_FILE"
    echo "请确保配置文件存在"
    exit 1
fi

# 检查是否已经在运行
PID_FILE="docker-tool.pid"
if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if ps -p "$PID" > /dev/null 2>&1; then
        echo "警告: Docker Tool 已经在运行 (PID: $PID)"
        echo "如果要重启，请先运行: ./bin/stop.sh"
        exit 1
    else
        echo "清理旧的PID文件..."
        rm -f "$PID_FILE"
    fi
fi

# 创建必要的目录
echo "创建必要的目录..."
mkdir -p logs
echo "✓ 目录创建完成"
echo ""

# 启动程序
echo "启动 Docker Tool..."
echo "配置文件: $CONFIG_FILE"
echo "日志目录: logs/"
echo ""

# 后台启动程序
nohup ./docker-tool -config "$CONFIG_FILE" > /dev/null 2>&1 &
PID=$!

# 保存PID
echo "$PID" > "$PID_FILE"

# 等待一下确保程序启动
sleep 2

# 检查程序是否成功启动
if ps -p "$PID" > /dev/null 2>&1; then
    echo "✓ Docker Tool 启动成功！"
    echo "进程ID: $PID"
    echo "PID文件: $PID_FILE"
    echo ""
    echo "查看日志:"
    echo "  tail -f logs/docker-tool-$(date +%Y-%m-%d).log"
    echo ""
    echo "停止程序:"
    echo "  ./bin/stop.sh"
    echo ""
    echo "查看状态:"
    echo "  ps -p $PID"
else
    echo "错误: Docker Tool 启动失败"
    rm -f "$PID_FILE"
    exit 1
fi

echo "=========================================="
echo "启动完成！"
echo "=========================================="
