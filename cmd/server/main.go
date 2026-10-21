// 聊天室服务端入口 — 薄层，所有逻辑在 internal/server 中
//
// 在 cmd/server 目录启动: go run .
// 在项目根目录启动: go run ./cmd/server -config config.yaml
// 构建: go build -o chat-server ./cmd/server
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"chatroom/internal/config"
	"chatroom/internal/server"
)

func main() {
	// 命令行参数 (优先级最高)
	port := flag.Int("port", 0, "监听端口 (覆盖配置文件)")
	host := flag.String("host", "", "监听地址 (覆盖配置文件)")
	configPath := flag.String("config", "../../config.yaml", "配置文件路径（相对于启动目录，默认从 cmd/server 启动）")
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
	if *configPath != "" {
		absolutePath, _ := filepath.Abs(*configPath)
		log.Printf("配置文件: %s", absolutePath)
	} else {
		log.Print("配置来源: 默认值 + 环境变量（未指定文件）")
	}
	log.Printf("Redis: enable=%t addr=%s db=%d", cfg.Redis.Enable, cfg.Redis.Addr, cfg.Redis.DB)
	if override, ok := os.LookupEnv("CHATROOM_REDIS_ENABLE"); ok {
		log.Printf("Redis 开关被环境变量 CHATROOM_REDIS_ENABLE=%q 覆盖", override)
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
