// Package client — 客户端心跳响应
//
// 面向对象设计:
//   - HeartbeatResponder 封装心跳响应逻辑
//   - 组合到 Session 中使用
//   - ReadLoopWithHeartbeat 函数式处理读取+心跳
package client

import (
	"context"
	"log"
	"time"

	"chatroom/internal/protocol"
)

const (
	// DefaultReadTimeout 默认读超时
	DefaultReadTimeout = 90 * time.Second
)

// HeartbeatResponder 心跳响应器
type HeartbeatResponder struct {
	session *Session
}

// NewHeartbeatResponder 创建心跳响应器
func NewHeartbeatResponder(session *Session) *HeartbeatResponder {
	return &HeartbeatResponder{session: session}
}

// Respond 响应 ping
func (h *HeartbeatResponder) Respond() error {
	return h.session.SendPong()
}

// ReadLoopWithHeartbeat 带心跳的读取循环
//
// 接收到 ping 自动回复 pong，其他消息放入 msgCh
//
// 参数:
//   - session: 客户端会话
//   - msgCh: 输出消息通道 (由本函数关闭)
//   - ctx: 取消上下文
func ReadLoopWithHeartbeat(session *Session, msgCh chan<- *protocol.Message, ctx context.Context) {
	defer close(msgCh)
	responder := NewHeartbeatResponder(session)

	for {
		select {
		case <-ctx.Done():
			return
		case <-session.Done():
			return
		default:
		}

		// 设置读超时
		if conn := session.Conn(); conn != nil {
			conn.SetReadDeadline(time.Now().Add(DefaultReadTimeout))
		}

		msg, err := session.ReadMessage()
		if err != nil {
			if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
				log.Println("⏰ 与服务器的连接超时")
			} else {
				log.Printf("⚠️ 读取消息失败: %v", err)
			}
			return
		}

		// 自动响应心跳
		if msg.Type == protocol.TypePing {
			if err := responder.Respond(); err != nil {
				log.Printf("⚠️ 发送心跳响应失败: %v", err)
				return
			}
			continue
		}

		// 发送给调用方
		select {
		case msgCh <- msg:
		case <-ctx.Done():
			return
		case <-session.Done():
			return
		}
	}
}
