// Package config 提供聊天室应用的配置管理
//
// 面向对象设计:
//   - Config 是值对象 (Value Object)，组合了所有子配置
//   - DefaultConfig() 提供生产环境默认值
//   - 字段公开，避免 getter/setter 样板代码
package config

import (
	"fmt"
	"time"
)

// Config 应用总配置 (组合模式)
type Config struct {
	Server    ServerCfg
	Heartbeat HeartbeatCfg
	RateLimit RateLimitCfg
	Validator ValidatorCfg
	Database  DatabaseCfg
}

// ServerCfg 服务端网络配置
type ServerCfg struct {
	Host            string
	Port            int
	MaxConnections  int
	ReadTimeoutSec  int
	WriteTimeoutSec int
}

// ReadTimeout 读超时 Duration
func (s ServerCfg) ReadTimeout() time.Duration {
	return time.Duration(s.ReadTimeoutSec) * time.Second
}

// WriteTimeout 写超时 Duration
func (s ServerCfg) WriteTimeout() time.Duration {
	return time.Duration(s.WriteTimeoutSec) * time.Second
}

// HeartbeatCfg 心跳检测配置
type HeartbeatCfg struct {
	IntervalSec int
	TimeoutSec  int
	MaxFailures int
}

// Interval 心跳间隔 Duration
func (h HeartbeatCfg) Interval() time.Duration {
	return time.Duration(h.IntervalSec) * time.Second
}

// Timeout 心跳超时 Duration
func (h HeartbeatCfg) Timeout() time.Duration {
	return time.Duration(h.TimeoutSec) * time.Second
}

// RateLimitCfg 限流配置
type RateLimitCfg struct {
	Enable     bool
	MsgsPerSec int
	Burst      int
}

// Enabled 是否启用限流
func (r RateLimitCfg) Enabled() bool { return r.Enable }

// MessagesPerSecond 每秒允许消息数
func (r RateLimitCfg) MessagesPerSecond() int { return r.MsgsPerSec }

// BurstSize 突发容量
func (r RateLimitCfg) BurstSize() int { return r.Burst }

// ValidatorCfg 验证器配置
type ValidatorCfg struct {
	MinUsernameLen int
	MaxUsernameLen int
	MaxMessageLen  int
}

// DatabaseCfg 数据持久化配置。
type DatabaseCfg struct {
	Enable      bool
	DSN         string
	AutoMigrate bool
}

// DefaultConfig 返回生产环境默认配置 (工厂函数)
func DefaultConfig() *Config {
	return &Config{
		Server: ServerCfg{
			Host:            "0.0.0.0",
			Port:            8888,
			MaxConnections:  1000,
			ReadTimeoutSec:  90,
			WriteTimeoutSec: 10,
		},
		Heartbeat: HeartbeatCfg{
			IntervalSec: 30,
			TimeoutSec:  10,
			MaxFailures: 3,
		},
		RateLimit: RateLimitCfg{
			Enable:     true,
			MsgsPerSec: 10,
			Burst:      20,
		},
		Validator: ValidatorCfg{
			MinUsernameLen: 1,
			MaxUsernameLen: 20,
			MaxMessageLen:  1000,
		},
		Database: DatabaseCfg{
			Enable:      false,
			DSN:         "",
			AutoMigrate: true,
		},
	}
}

// Validate 检查配置值是否可用。
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port 必须在 1 到 65535 之间")
	}
	if c.Server.MaxConnections <= 0 {
		return fmt.Errorf("server.max_connections 必须大于 0")
	}
	if c.Server.ReadTimeoutSec <= 0 {
		return fmt.Errorf("server.read_timeout_sec 必须大于 0")
	}
	if c.Server.WriteTimeoutSec <= 0 {
		return fmt.Errorf("server.write_timeout_sec 必须大于 0")
	}
	if c.Heartbeat.IntervalSec <= 0 {
		return fmt.Errorf("heartbeat.interval_sec 必须大于 0")
	}
	if c.Heartbeat.TimeoutSec <= 0 {
		return fmt.Errorf("heartbeat.timeout_sec 必须大于 0")
	}
	if c.Heartbeat.MaxFailures <= 0 {
		return fmt.Errorf("heartbeat.max_failures 必须大于 0")
	}
	if c.RateLimit.MsgsPerSec <= 0 {
		return fmt.Errorf("rate_limit.messages_per_second 必须大于 0")
	}
	if c.RateLimit.Burst <= 0 {
		return fmt.Errorf("rate_limit.burst_size 必须大于 0")
	}
	if c.Validator.MinUsernameLen <= 0 {
		return fmt.Errorf("validator.min_username_len 必须大于 0")
	}
	if c.Validator.MaxUsernameLen < c.Validator.MinUsernameLen {
		return fmt.Errorf("validator.max_username_len 不能小于 min_username_len")
	}
	if c.Validator.MaxMessageLen <= 0 {
		return fmt.Errorf("validator.max_message_len 必须大于 0")
	}
	if c.Database.Enable && c.Database.DSN == "" {
		return fmt.Errorf("database.dsn 不能为空")
	}
	return nil
}
