package server

import (
	"log"
	"sync"
	"time"

	"chatroom/internal/config"
	"chatroom/internal/protocol"
)

// Heartbeat 心跳检测器 (Composition Pattern)
//
// 可组合到任何 Client 上，负责:
//   - 定期发送 ping
//   - 等待 pong 响应
//   - 超时或连续失败时回调 onFailure
type Heartbeat struct {
	cfg       config.HeartbeatCfg
	client    *Client
	wg        *sync.WaitGroup
	onFailure func(*Client)
}

// NewHeartbeat 创建心跳检测器
func NewHeartbeat(cfg config.HeartbeatCfg, client *Client, wg *sync.WaitGroup, onFailure func(*Client)) *Heartbeat {
	return &Heartbeat{
		cfg:       cfg,
		client:    client,
		wg:        wg,
		onFailure: onFailure,
	}
}

// Start 启动心跳循环
func (h *Heartbeat) Start() {
	h.wg.Add(1)
	go h.loop()
}

func (h *Heartbeat) loop() {
	defer h.wg.Done()

	ticker := time.NewTicker(h.cfg.Interval())
	defer ticker.Stop()

	failures := 0

	for {
		select {
		case <-h.client.Done():
			return

		case <-ticker.C:
			// 发送 ping
			if !h.client.Send(protocol.NewPingMessage()) {
				failures++
				if failures >= h.cfg.MaxFailures {
					h.onFailure(h.client)
					return
				}
				continue
			}

			// 等待 pong (同包内访问私有字段)
			timer := time.NewTimer(h.cfg.Timeout())
			select {
			case <-h.client.pongCh:
				failures = 0
				timer.Stop()

			case <-timer.C:
				failures++
				if failures >= h.cfg.MaxFailures {
					log.Printf("💀 [%s] 心跳超时 (%d次)，断开连接", h.client.Name(), failures)
					h.onFailure(h.client)
					return
				}

			case <-h.client.Done():
				timer.Stop()
				return
			}
		}
	}
}
