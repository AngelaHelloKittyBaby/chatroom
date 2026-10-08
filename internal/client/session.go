// Package client — 聊天室客户端会话
//
// 面向对象设计:
//   - Session 封装服务器连接和状态
//   - SessionLifecycle 接口定义连接生命周期
//   - MessageDisplay 接口解耦显示逻辑
//   - 组合 HeartbeatResponder 实现心跳
package client

import (
	"chatroom/internal/protocol"
	"chatroom/internal/validator"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// ============================================================
// 接口定义
// ============================================================

// SessionLifecycle 会话生命周期接口
type SessionLifecycle interface {
	Connect(addr string) error
	Join(name, password string) error
	Disconnect() error
	IsConnected() bool
}

// MessageDisplay 消息显示接口 (UI 层解耦)
type MessageDisplay interface {
	ShowChat(msg *protocol.Message)
	ShowSystem(msg *protocol.Message)
	ShowPrivate(msg *protocol.Message)
	ShowError(err string)
	ShowUsers(users []string, count int)
	ShowWelcome(msg string)
}

// ============================================================
// Session 实现
// ============================================================

// Session 客户端会话对象
//
// 职责:
//   - 管理 TCP 连接
//   - 处理加入/离开流程
//   - 提供消息收发接口
//   - 组合心跳响应器
type Session struct {
	conn      net.Conn
	name      string
	addr      string
	connected bool
	mu        sync.RWMutex
	writeMu   sync.Mutex
	done      chan struct{}
	closeOnce sync.Once
}

// NewSession 创建会话 (构造函数)
func NewSession() *Session {
	return &Session{
		done: make(chan struct{}),
	}
}

// --- Lifecycle 接口实现 ---

// Connect 连接到服务器
func (s *Session) Connect(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("连接服务器失败: %w", err)
	}

	// 启用 TCP KeepAlive
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	s.conn = conn
	s.addr = addr
	s.connected = true
	s.done = make(chan struct{})
	return nil
}

// Join 发送加入请求并等待确认
func (s *Session) Join(name, password string) error {
	cleanName := validator.TrimName(name)
	password = strings.TrimSpace(password)

	// 本地验证
	v := validator.NewStandardValidator(validator.DefaultConfig())
	if err := v.ValidateUsername(cleanName); err != nil {
		return fmt.Errorf("用户名不合法: %w", err)
	}
	if password == "" {
		return fmt.Errorf("密码不能为空")
	}

	// 发送加入请求
	if err := s.writeMessage(protocol.NewJoinMessage(cleanName, password)); err != nil {
		return fmt.Errorf("发送加入请求失败: %w", err)
	}

	// 读取响应
	msg, err := s.readMessageWithTimeout(30 * time.Second)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	switch msg.Type {
	case protocol.TypeWelcome:
		s.name = cleanName
		return nil
	case protocol.TypeError:
		return fmt.Errorf("%s", msg.Content)
	default:
		return fmt.Errorf("收到未知响应: %s", msg.Type)
	}
}

// Disconnect 断开连接
func (s *Session) Disconnect() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.connected = false
		close(s.done)
		if s.conn != nil {
			s.conn.Close()
		}
	})
	return nil
}

// IsConnected 是否已连接
func (s *Session) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected
}

// --- 属性访问 ---

// Name 返回网名
func (s *Session) Name() string { return s.name }

// Addr 返回服务器地址
func (s *Session) Addr() string { return s.addr }

// Done 返回关闭信号
func (s *Session) Done() <-chan struct{} { return s.done }

// Conn 返回底层连接 (供心跳等内部组件使用)
func (s *Session) Conn() net.Conn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.conn
}

// --- 消息收发 ---

// SendMessage 发送聊天消息 (带本地验证)
func (s *Session) SendMessage(content string) error {
	content = validator.TrimContent(content)
	v := validator.NewStandardValidator(validator.DefaultConfig())
	if err := v.ValidateContent(content); err != nil {
		return err
	}
	return s.writeMessage(protocol.NewChatMessage(s.name, content))
}

// SendCommand 发送命令 (以 / 开头)
func (s *Session) SendCommand(cmd string) error {
	msg := &protocol.Message{
		Type:    protocol.TypeChat,
		From:    s.name,
		Content: cmd,
	}
	return s.writeMessage(msg)
}

// ReadMessage 读取一条消息 (阻塞)
func (s *Session) ReadMessage() (*protocol.Message, error) {
	return s.readMessageWithTimeout(90 * time.Second)
}

// --- 内部方法 ---

func (s *Session) writeMessage(msg *protocol.Message) error {
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("未连接到服务器")
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err := protocol.WriteMessage(conn, msg)
	return err
}

func (s *Session) readMessageWithTimeout(timeout time.Duration) (*protocol.Message, error) {
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("未连接到服务器")
	}
	conn.SetReadDeadline(time.Now().Add(timeout))
	return protocol.ReadMessage(conn)
}

// SendPong 发送心跳响应
func (s *Session) SendPong() error {
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("未连接")
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := protocol.WriteMessage(conn, protocol.NewPongMessage())
	return err
}

// ============================================================
// 默认终端显示实现
// ============================================================

// TerminalDisplay 终端消息显示器
type TerminalDisplay struct{}

func (d *TerminalDisplay) ShowChat(msg *protocol.Message) {
	fmt.Printf("[%s] %s: %s\n", msg.Timestamp, msg.From, msg.Content)
}

func (d *TerminalDisplay) ShowSystem(msg *protocol.Message) {
	if msg.Timestamp != "" {
		fmt.Printf("[%s] *** %s ***\n", msg.Timestamp, msg.Content)
	} else {
		fmt.Printf("*** %s ***\n", msg.Content)
	}
}

func (d *TerminalDisplay) ShowPrivate(msg *protocol.Message) {
	fmt.Printf("[%s] 🔒 %s → %s: %s\n", msg.Timestamp, msg.From, msg.To, msg.Content)
}

func (d *TerminalDisplay) ShowError(err string) {
	fmt.Printf("❌ %s\n", err)
}

func (d *TerminalDisplay) ShowUsers(users []string, count int) {
	if count > 0 {
		fmt.Printf("📋 当前在线 (%d人): ", count)
		for i, name := range users {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(name)
		}
		fmt.Println()
	} else {
		fmt.Println("📋 你是第一个在线的用户！")
	}
}

func (d *TerminalDisplay) ShowWelcome(msg string) {
	fmt.Println(msg)
}

// ============================================================
// 消息分发器
// ============================================================

// DispatchMessage 根据消息类型分发到显示接口
func DispatchMessage(msg *protocol.Message, display MessageDisplay) {
	switch msg.Type {
	case protocol.TypeChat:
		display.ShowChat(msg)
	case protocol.TypeSystem:
		display.ShowSystem(msg)
	case protocol.TypePrivate:
		display.ShowPrivate(msg)
	case protocol.TypeError:
		display.ShowError(msg.Content)
	case protocol.TypeUsers:
		display.ShowUsers(msg.OnlineUsers, msg.UserCount)
	case protocol.TypeWelcome:
		display.ShowWelcome(msg.Content)
	default:
		log.Printf("⚠️ 未处理的消息类型: %s", msg.Type)
	}
}
