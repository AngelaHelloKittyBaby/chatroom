// Package server — 客户端状态机
//
// 面向对象设计:
//   - Client 封装了连接、身份、状态和行为
//   - 状态机模式管理客户端生命周期
//   - 通过 channel 与 Room 通信 (Actor 模式)
//   - ClientIdentity 接口暴露只读身份信息
package server

import (
	"chatroom/internal/protocol"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// 客户端状态机 (State Pattern)
// ============================================================

// ClientState 客户端状态
type ClientState int

const (
	StateConnected      ClientState = iota // TCP 已连接
	StateAuthenticating                    // 正在验证用户名
	StateActive                            // 已加入房间，正常聊天
	StateDisconnecting                     // 正在断开
	StateDisconnected                      // 已断开
)

func (s ClientState) String() string {
	switch s {
	case StateConnected:
		return "Connected"
	case StateAuthenticating:
		return "Authenticating"
	case StateActive:
		return "Active"
	case StateDisconnecting:
		return "Disconnecting"
	case StateDisconnected:
		return "Disconnected"
	default:
		return "Unknown"
	}
}

// ============================================================
// Client 对象
// ============================================================

// Client 表示服务端管理的一个客户端连接
//
// 职责:
//   - 封装 TCP 连接
//   - 管理状态转换
//   - 提供非阻塞消息发送
//   - 统计连接数据
type Client struct {
	id       string                 // 唯一标识 (UUID)
	name     string                 // 显示名称
	conn     net.Conn               // TCP 连接
	sendCh   chan *protocol.Message // 待发送消息队列
	room     *Room                  // 所属房间
	joinedAt time.Time              // 加入时间

	state   ClientState  // 当前状态
	stateMu sync.RWMutex // 保护状态

	pongCh chan struct{} // 收到 pong 信号

	done      chan struct{} // 关闭信号
	closeOnce sync.Once     // 确保只关闭一次

	// 统计
	msgSent int64 // 已发送消息数
	msgRecv int64 // 已接收消息数
}

// NewClient 创建客户端 (构造函数)
func NewClient(id, name string, conn net.Conn, room *Room) *Client {
	return &Client{
		id:       id,
		name:     name,
		conn:     conn,
		sendCh:   make(chan *protocol.Message, 256),
		room:     room,
		joinedAt: time.Now(),
		state:    StateConnected,
		pongCh:   make(chan struct{}, 3),
		done:     make(chan struct{}),
	}
}

// --- 身份访问 (只读) ---

// ID 返回唯一标识
func (c *Client) ID() string { return c.id }

// Name 返回显示名称
func (c *Client) Name() string { return c.name }

// Room 返回所属房间
func (c *Client) Room() *Room { return c.room }

// JoinedAt 返回加入时间
func (c *Client) JoinedAt() time.Time { return c.joinedAt }

// RemoteAddr 返回客户端地址
func (c *Client) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }

// --- 统计 ---

// MsgSent 返回已发送消息数
func (c *Client) MsgSent() int64 { return atomic.LoadInt64(&c.msgSent) }

// MsgRecv 返回已接收消息数
func (c *Client) MsgRecv() int64 { return atomic.LoadInt64(&c.msgRecv) }

// IncrSent 增加发送计数
func (c *Client) IncrSent() { atomic.AddInt64(&c.msgSent, 1) }

// IncrRecv 增加接收计数
func (c *Client) IncrRecv() { atomic.AddInt64(&c.msgRecv, 1) }

// --- 状态机 ---

// State 返回当前状态
func (c *Client) State() ClientState {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.state
}

// SetState 转换状态
func (c *Client) SetState(s ClientState) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.state = s
}

// IsActive 是否处于活跃状态
func (c *Client) IsActive() bool {
	return c.State() == StateActive
}

// --- 消息发送 ---

// Send 非阻塞发送消息给客户端
//
// 如果发送通道满，返回 false (调用方可选择断开连接)
func (c *Client) Send(msg *protocol.Message) bool {
	select {
	case c.sendCh <- msg:
		c.IncrSent()
		return true
	default:
		return false
	}
}

// SendCh 返回发送通道 (供 Room 广播使用)
func (c *Client) SendCh() <-chan *protocol.Message {
	return c.sendCh
}

// --- 心跳 ---

// PongCh 返回心跳信号通道
func (c *Client) PongCh() chan<- struct{} {
	return c.pongCh
}

// SignalPong 发送 pong 信号
func (c *Client) SignalPong() {
	select {
	case c.pongCh <- struct{}{}:
	default:
	}
}

// --- 连接管理 ---

// Conn 返回底层连接 (供读写循环使用)
func (c *Client) Conn() net.Conn {
	return c.conn
}

// SetReadDeadline 设置读超时
func (c *Client) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// SetWriteDeadline 设置写超时
func (c *Client) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

// Close 安全关闭客户端 (幂等)
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.SetState(StateDisconnecting)
		close(c.done)
		c.conn.Close()
		c.SetState(StateDisconnected)
	})
}

// Done 返回关闭信号
func (c *Client) Done() <-chan struct{} {
	return c.done
}

// String 实现 Stringer 接口
func (c *Client) String() string {
	return fmt.Sprintf("Client{id=%s, name=%s, state=%s, addr=%s}",
		c.id, c.name, c.State(), c.RemoteAddr())
}
