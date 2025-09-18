package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"docker-tool/internal/config"
	"docker-tool/internal/watcher"
)

// initLogger 初始化日志系统
func initLogger() {
	// 确保logs目录存在
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		log.Fatalf("创建日志目录失败: %v", err)
	}

	// 生成日志文件名（按日期）
	logFileName := filepath.Join(logsDir, fmt.Sprintf("docker-tool-%s.log", time.Now().Format("2006-01-02")))
	
	// 打开日志文件
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("打开日志文件失败: %v", err)
	}

	// 设置日志输出到文件和控制台
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	
	// 设置日志格式
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	
	log.Printf("日志系统已初始化，日志文件: %s", logFileName)
}

func main() {
	// 命令行参数
	var configFile = flag.String("config", "conf/config.yaml", "配置文件路径")
	flag.Parse()

	// 初始化日志系统
	initLogger()

	// 加载配置
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	// 创建容器监听器
	containerWatcher, err := watcher.New(cfg)
	if err != nil {
		log.Fatalf("创建容器监听器失败: %v", err)
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动监听器
	if err := containerWatcher.Start(ctx); err != nil {
		log.Fatalf("启动容器监听器失败: %v", err)
	}

	log.Println("Docker Tool 已启动，开始监听容器事件...")

	// 等待信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		log.Printf("收到信号 %v，正在关闭...", sig)
		cancel()
	case <-ctx.Done():
		log.Println("上下文已取消")
	}

	// 清理资源
	if err := containerWatcher.Stop(); err != nil {
		log.Printf("停止容器监听器时出错: %v", err)
	}

	log.Println("Docker Tool 已退出")
}
