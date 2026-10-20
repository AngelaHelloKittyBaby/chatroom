package server

import (
	"chatroom/internal/auth"
	"chatroom/internal/protocol"
	"chatroom/internal/redisstore"
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

func (s *Server) cacheUser(user *auth.User) error {
	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
	defer cancel()
	return s.redis.CacheUser(ctx, user)
}
func (s *Server) enqueue(room string, msg *protocol.Message) error {
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	return s.redis.Enqueue(ctx, room, msg)
}
func (s *Server) deliverQueued(event redisstore.Event) error {
	room, ok := s.roomMgr.GetRoom(event.Room)
	if !ok {
		return fmt.Errorf("room not found: %s", event.Room)
	}
	msg := event.Message
	if msg.Type == protocol.TypePrivate {
		if !room.SendTo(msg.To, msg) {
			room.SendTo(msg.From, protocol.NewErrorMessage("私聊接收方已离线或发送缓冲区已满"))
		}
		room.SendTo(msg.From, msg)
	} else if msg.Type == protocol.TypeChat {
		if !room.submit(roomAction{cmd: cmdBroadcast, msg: msg}) {
			return fmt.Errorf("room closed")
		}
	} else {
		return fmt.Errorf("unsupported queued message type")
	}
	return nil
}
func (s *Server) showRanking(c *Client, args []string) {
	if s.redis == nil {
		c.Send(protocol.NewErrorMessage("Redis 未启用"))
		return
	}
	n := 10
	if len(args) > 1 {
		c.Send(protocol.NewErrorMessage("用法: /rank [1-100]"))
		return
	}
	if len(args) == 1 {
		var err error
		n, err = strconv.Atoi(args[0])
		if err != nil || n < 1 || n > 100 {
			c.Send(protocol.NewErrorMessage("排名数量必须在 1 到 100 之间"))
			return
		}
	}
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	entries, err := s.redis.Ranking(ctx, int64(n))
	if err != nil {
		c.Send(protocol.NewErrorMessage("排名查询失败，请稍后重试"))
		return
	}
	var b strings.Builder
	b.WriteString("活跃度排名（有效入队群聊/私聊各计1分）")
	if len(entries) == 0 {
		b.WriteString("\n暂无数据")
	}
	for i, e := range entries {
		fmt.Fprintf(&b, "\n%d. %s: %.0f", i+1, e.Member, e.Score)
	}
	c.Send(protocol.NewSystemMessage(b.String()))
}
func (s *Server) showProfile(c *Client) {
	if c.userID == 0 {
		c.Send(protocol.NewErrorMessage("用户资料需要启用 MySQL 认证"))
		return
	}
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	var user *auth.User
	if s.redis != nil {
		user, _ = s.redis.User(ctx, c.userID)
	}
	if user == nil {
		store, ok := s.userStore.(interface {
			FindUser(context.Context, uint64) (*auth.User, error)
		})
		if !ok {
			c.Send(protocol.NewErrorMessage("用户资料不可用"))
			return
		}
		var err error
		user, err = store.FindUser(ctx, c.userID)
		if err != nil {
			c.Send(protocol.NewErrorMessage("查询用户资料失败"))
			return
		}
		if s.redis != nil {
			if err = s.cacheUser(user); err != nil {
				log.Printf("cache profile: %v", err)
			}
		}
	}
	c.Send(protocol.NewSystemMessage(fmt.Sprintf("用户ID: %d，用户名: %s", user.ID, user.Username)))
}
