#!/bin/bash

# Docker Tool 停止脚本
# 用于停止docker-tool程序

set -e

# 获取脚本所在目录的父目录（项目根目录）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=========================================="
echo "Docker Tool 停止脚本"
echo "=========================================="
echo "项目目录: $PROJECT_DIR"
echo "停止时间: $(date)"
echo ""

# 进入项目目录
cd "$PROJECT_DIR"

# 检查PID文件是否存在
PID_FILE="docker-tool.pid"
if [ ! -f "$PID_FILE" ]; then
    echo "信息: 未找到PID文件 $PID_FILE"
    echo "程序可能没有在运行"
    
    # 尝试通过进程名查找
    PIDS=$(pgrep -f "docker-tool" || true)
    if [ -n "$PIDS" ]; then
        echo "发现运行中的docker-tool进程: $PIDS"
        echo "是否要停止这些进程? (y/N)"
        read -r response
        if [[ "$response" =~ ^[Yy]$ ]]; then
            echo "停止进程..."
            echo "$PIDS" | xargs kill -TERM
            sleep 2
            echo "✓ 进程已停止"
        else
            echo "取消停止操作"
        fi
    else
        echo "✓ 没有发现运行中的docker-tool进程"
    fi
    exit 0
fi

# 读取PID
PID=$(cat "$PID_FILE")

# 检查进程是否存在
if ! ps -p "$PID" > /dev/null 2>&1; then
    echo "信息: 进程 $PID 不存在，可能已经停止"
    echo "清理PID文件..."
    rm -f "$PID_FILE"
    echo "✓ PID文件已清理"
    exit 0
fi

echo "发现运行中的Docker Tool进程 (PID: $PID)"

# 发送TERM信号
echo "发送停止信号..."
kill -TERM "$PID"

# 等待进程停止
echo "等待进程停止..."
for i in {1..10}; do
    if ! ps -p "$PID" > /dev/null 2>&1; then
        echo "✓ 进程已正常停止"
        break
    fi
    echo "等待中... ($i/10)"
    sleep 1
done

# 如果进程仍然存在，强制杀死
if ps -p "$PID" > /dev/null 2>&1; then
    echo "进程未响应TERM信号，强制停止..."
    kill -KILL "$PID"
    sleep 1
    if ps -p "$PID" > /dev/null 2>&1; then
        echo "错误: 无法停止进程 $PID"
        exit 1
    else
        echo "✓ 进程已强制停止"
    fi
fi

# 清理PID文件
rm -f "$PID_FILE"
echo "✓ PID文件已清理"

echo ""
echo "Docker Tool 已停止"
echo ""

# 显示最近的日志
LATEST_LOG=$(ls -t logs/docker-tool-*.log 2>/dev/null | head -1)
if [ -n "$LATEST_LOG" ]; then
    echo "最近的日志文件: $LATEST_LOG"
    echo "最后几行日志:"
    echo "----------------------------------------"
    tail -5 "$LATEST_LOG" 2>/dev/null || echo "无法读取日志文件"
    echo "----------------------------------------"
fi

echo "=========================================="
echo "停止完成！"
echo "=========================================="
