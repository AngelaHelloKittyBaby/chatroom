// 聊天室客户端入口 — 薄层，所有逻辑在 internal/client 中
//
// 启动: go run ./cmd/client
// 构建: go build -o chat-client ./cmd/client
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"chatroom/internal/client"
	"chatroom/internal/protocol"
)

func main() {
	addr := flag.String("addr", "localhost:8888", "服务器地址")
	flag.Parse()

	fmt.Println("    💬 命令行聊天室 - 客户端   ")
	fmt.Println()

	// --- 连接 ---
	stdin := bufio.NewReader(os.Stdin)

	fmt.Printf("服务器地址 (默认 %s): ", *addr)
	input, _ := stdin.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		*addr = input
	}

	fmt.Printf("正在连接 %s ...\n", *addr)
	session := client.NewSession()
	if err := session.Connect(*addr); err != nil {
		log.Fatalf("❌ %v\n   请确保服务器已启动: go run ./cmd/server", err)
	}
	defer session.Disconnect()

	// --- 加入 ---
	var display client.TerminalDisplay
	for {
		fmt.Print("请输入你的网名: ")
		name, _ := stdin.ReadString('\n')
		name = strings.TrimSpace(name)

		fmt.Print("请输入密码: ")
		password, _ := stdin.ReadString('\n')
		password = strings.TrimSpace(password)

		if err := session.Join(name, password); err != nil {
			fmt.Printf("❌ %v\n", err)
			if strings.Contains(err.Error(), "已被占用") ||
				strings.Contains(err.Error(), "不合法") ||
				strings.Contains(err.Error(), "保留名称") ||
				strings.Contains(err.Error(), "密码") {
				continue
			}
			return
		}
		break
	}

	fmt.Printf("\n✅ 已进入聊天室，网名: %s\n", session.Name())
	fmt.Println("💡 /msg 用户名 消息 → 私聊 | /users → 在线用户 | /quit → 退出 | /rank → 查看前 10 名 | /rank [1-100] ")
	fmt.Println("──────────────────────────────────────────────────")

	// --- 启动接收协程 ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msgCh := make(chan *protocol.Message, 50)
	go client.ReadLoopWithHeartbeat(session, msgCh, ctx)

	// 显示协程
	go func() {
		for msg := range msgCh {
			client.DispatchMessage(msg, &display)

		}
		fmt.Println("\n❌ 与服务器的连接已断开")
		cancel()
		os.Exit(0)
	}()

	// --- 主循环: 读取用户输入 ---
	for {
		msg, err := stdin.ReadString('\n')
		if err != nil {
			break
		}
		msg = strings.TrimSpace(msg)
		if msg == "" {
			continue
		}

		switch {
		case msg == "/quit":
			fmt.Println("👋 正在退出...")
			cancel()
			session.Disconnect()
			fmt.Println("👋 已退出聊天室")
			return

		case msg == "/help":
			printHelp()

		case strings.HasPrefix(msg, "/"):
			if err := session.SendCommand(msg); err != nil {
				fmt.Printf("❌ %v\n", err)
			}

		default:
			if err := session.SendMessage(msg); err != nil {
				fmt.Printf("❌ 发送失败: %v\n", err)
			}
		}
	}
}

func printHelp() {
	fmt.Println("━━━━━━━━━ 命令列表 ━━━━━━━━━")
	fmt.Println("  /msg <用户名> <消息>  发送私聊消息")
	fmt.Println("  /users                查看在线用户")
	fmt.Println("  /profile              查看当前用户资料（优先读取 Redis 缓存）")
	fmt.Println("  /quit                 退出聊天室")
	fmt.Println("  /rank [1-100]          Activity ranking")
	fmt.Println("  /help                 显示此帮助")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━")
}
