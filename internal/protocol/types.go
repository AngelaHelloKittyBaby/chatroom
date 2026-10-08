// Package protocol 定义聊天室通信协议
//
// 面向对象设计原则:
//   - Message 是核心领域对象，封装了所有消息相关的行为
//   - Conn 接口抽象网络连接，支持测试替身
//   - Codec 负责序列化/反序列化，遵循单一职责
//   - MessageType 使用常量避免魔法字符串
package protocol

import (
	"net"
	"time"
)

// ============================================================
// 网络抽象接口 (面向接口编程，支持测试)
// ============================================================

// Conn 抽象 TCP 连接，便于单元测试时 mock
// net.Conn 自动满足此接口
type Conn interface {
	Read(b []byte) (n int, err error)
	Write(b []byte) (n int, err error)
	Close() error
	RemoteAddr() net.Addr
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

// ============================================================
// 消息类型常量
// ============================================================

// MessageType 消息类型 (类型安全常量)
type MessageType string

const (
	TypeJoin    MessageType = "join"
	TypeWelcome MessageType = "welcome"
	TypeChat    MessageType = "chat"
	TypePrivate MessageType = "private" // 私聊消息
	TypeSystem  MessageType = "system"
	TypePing    MessageType = "ping"
	TypePong    MessageType = "pong"
	TypeUsers   MessageType = "users"
	TypeError   MessageType = "error"
)

// IsValid 验证消息类型是否合法
func (t MessageType) IsValid() bool {
	switch t {
	case TypeJoin, TypeWelcome, TypeChat, TypePrivate, TypeSystem,
		TypePing, TypePong, TypeUsers, TypeError:
		return true
	}
	return false
}

// ============================================================
// 核心领域对象: Message
// ============================================================

// Message 聊天室通信的核心领域对象
//
// 职责:
//   - 承载所有通信数据
//   - 提供工厂方法创建不同类型的消息
//   - 封装消息验证逻辑
type Message struct {
	Type        MessageType `json:"type"`                   // 消息类型
	From        string      `json:"from,omitempty"`         // 发送者
	To          string      `json:"to,omitempty"`           // 私聊目标
	Password    string      `json:"password,omitempty"`     // 登录/注册密码，仅用于 join
	Content     string      `json:"content,omitempty"`      // 消息体
	Timestamp   string      `json:"timestamp,omitempty"`    // HH:MM:SS
	OnlineUsers []string    `json:"online_users,omitempty"` // 在线用户
	UserCount   int         `json:"user_count,omitempty"`   // 在线人数
}

// --- 工厂方法 (Factory Pattern) ---

// NewChatMessage 创建聊天消息
func NewChatMessage(from, content string) *Message {
	return &Message{
		Type:      TypeChat,
		From:      from,
		Content:   content,
		Timestamp: time.Now().Format("15:04:05"),
	}
}

// NewPrivateMessage 创建私聊消息
func NewPrivateMessage(from, to, content string) *Message {
	return &Message{
		Type:      TypePrivate,
		From:      from,
		To:        to,
		Content:   content,
		Timestamp: time.Now().Format("15:04:05"),
	}
}

// NewSystemMessage 创建系统通知
func NewSystemMessage(content string) *Message {
	return &Message{
		Type:      TypeSystem,
		Content:   content,
		Timestamp: time.Now().Format("15:04:05"),
	}
}

// NewErrorMessage 创建错误消息
func NewErrorMessage(content string) *Message {
	return &Message{
		Type:    TypeError,
		Content: content,
	}
}

// NewWelcomeMessage 创建欢迎消息
func NewWelcomeMessage(content string) *Message {
	return &Message{
		Type:    TypeWelcome,
		Content: content,
	}
}

// NewPingMessage 创建心跳请求
func NewPingMessage() *Message {
	return &Message{Type: TypePing}
}

// NewPongMessage 创建心跳响应
func NewPongMessage() *Message {
	return &Message{Type: TypePong}
}

// NewUsersMessage 创建在线用户列表消息
func NewUsersMessage(users []string) *Message {
	return &Message{
		Type:        TypeUsers,
		OnlineUsers: users,
		UserCount:   len(users),
	}
}

// NewJoinMessage 创建加入请求
func NewJoinMessage(name, password string) *Message {
	return &Message{
		Type:     TypeJoin,
		From:     name,
		Password: password,
	}
}

// --- 行为方法 ---

// IsCommand 判断是否为命令消息 (以 / 开头)
func (m *Message) IsCommand() bool {
	return len(m.Content) > 0 && m.Content[0] == '/'
}

// IsEmpty 判断消息内容是否为空
func (m *Message) IsEmpty() bool {
	return len(m.Content) == 0
}

// WithTimestamp 设置时间戳 (Builder 模式)
func (m *Message) WithTimestamp() *Message {
	m.Timestamp = time.Now().Format("15:04:05")
	return m
}
