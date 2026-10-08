// 聊天室服务端入口 — 薄层，所有逻辑在 internal/server 中
//
// 启动: go run ./cmd/server
// 构建: go build -o chat-server ./cmd/server
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"chatroom/internal/config"
	"chatroom/internal/server"
)

func main() {
	// 命令行参数 (优先级最高)
	port := flag.Int("port", 0, "监听端口 (覆盖配置文件)")
	host := flag.String("host", "", "监听地址 (覆盖配置文件)")
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadFile(*configPath)
	if err != nil {
		log.Fatalf("❌ 加载配置失败: %v", err)
	}

	// 命令行覆盖
	if *port > 0 {
		cfg.Server.Port = *port
	}
	if *host != "" {
		cfg.Server.Host = *host
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("❌ 配置无效: %v", err)
	}

	// 打印横幅
	printBanner(cfg.Server.Port)

	// 启动服务器 (RunServer 内部处理信号和优雅关闭)
	if err := server.RunServer(cfg); err != nil {
		log.Fatalf("❌ 服务器运行失败: %v", err)
	}

	fmt.Println("\n👋 服务器已停止")
	os.Exit(0)
}

func printBanner(port int) {
	fmt.Println("    🚀 命令行聊天室 - 服务器    ")
	fmt.Println()
	fmt.Printf("🔧 监听端口: %d\n", port)
	fmt.Println("💡 等待客户端连接...")
	fmt.Println()
}
