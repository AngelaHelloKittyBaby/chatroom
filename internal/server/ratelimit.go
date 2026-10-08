// Package server — 限流器
//
// 面向对象设计:
//   - RateLimiter 接口定义限流契约
//   - TokenBucket 实现令牌桶算法
//   - 每个客户端独立限流
package server

import (
	"chatroom/internal/config"
	"sync"
	"time"
)

// ============================================================
// 限流器接口
// ============================================================

// RateLimiter 限流器接口
type RateLimiter interface {
	// Allow 检查是否允许通过，返回 false 表示被限流
	Allow(key string) bool
	// Reset 重置指定 key 的限流状态
	Reset(key string)
	// Close 释放限流器内部资源
	Close()
}

// ============================================================
// 令牌桶实现
// ============================================================

// TokenBucket 令牌桶限流器
type TokenBucket struct {
	cfg    config.RateLimitCfg
	mu     sync.Mutex
	tokens map[string]int       // key → 当前令牌数
	last   map[string]time.Time // key → 上次补充时间
	ticker *time.Ticker
	done   chan struct{}
}

// NewTokenBucket 创建令牌桶
func NewTokenBucket(cfg config.RateLimitCfg) *TokenBucket {
	tb := &TokenBucket{
		cfg:    cfg,
		tokens: make(map[string]int),
		last:   make(map[string]time.Time),
		ticker: time.NewTicker(time.Second),
		done:   make(chan struct{}),
	}
	go tb.refill()
	return tb
}

// Allow 检查是否允许
func (tb *TokenBucket) Allow(key string) bool {
	if !tb.cfg.Enabled() {
		return true
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.replenish(key)

	if tb.tokens[key] > 0 {
		tb.tokens[key]--
		return true
	}
	return false
}

// Reset 重置限流状态
func (tb *TokenBucket) Reset(key string) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	delete(tb.tokens, key)
	delete(tb.last, key)
}

// replenish 补充令牌 (内部，需持有锁)
func (tb *TokenBucket) replenish(key string) {
	now := time.Now()
	lastTime, ok := tb.last[key]
	if !ok {
		// 首次: 给满 burst
		tb.tokens[key] = tb.cfg.BurstSize()
		tb.last[key] = now
		return
	}

	elapsed := now.Sub(lastTime)
	newTokens := int(elapsed.Seconds()) * tb.cfg.MessagesPerSecond()
	if newTokens > 0 {
		tb.tokens[key] += newTokens
		if tb.tokens[key] > tb.cfg.BurstSize() {
			tb.tokens[key] = tb.cfg.BurstSize()
		}
		tb.last[key] = now
	}
}

// refill 定期清理过期状态
func (tb *TokenBucket) refill() {
	for {
		select {
		case <-tb.done:
			return
		case <-tb.ticker.C:
			tb.mu.Lock()
			threshold := time.Now().Add(-5 * time.Minute)
			for key, lastTime := range tb.last {
				if lastTime.Before(threshold) {
					delete(tb.tokens, key)
					delete(tb.last, key)
				}
			}
			tb.mu.Unlock()
		}
	}
}

// Close 关闭限流器
func (tb *TokenBucket) Close() {
	tb.ticker.Stop()
	close(tb.done)
}
