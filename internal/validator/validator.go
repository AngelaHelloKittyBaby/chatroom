// Package validator 提供聊天室数据验证
//
// 面向对象设计:
//   - MessageValidator 接口定义验证契约
//   - StandardValidator 是默认实现
//   - 可通过实现接口替换验证策略 (Strategy Pattern)
package validator

import (
	"fmt"
	"strings"
	"sync"
	"unicode"

	"chatroom/internal/config"
)

// ============================================================
// 验证接口 (面向接口编程)
// ============================================================

// MessageValidator 消息验证器接口
type MessageValidator interface {
	ValidateUsername(name string) error
	ValidateContent(content string) error
	Config() config.ValidatorCfg
}

// ============================================================
// 标准验证器实现
// ============================================================

// StandardValidator 标准验证器
type StandardValidator struct {
	cfg config.ValidatorCfg
	mu  sync.RWMutex
}

// NewStandardValidator 创建验证器 (构造函数)
func NewStandardValidator(cfg config.ValidatorCfg) *StandardValidator {
	return &StandardValidator{cfg: cfg}
}

func (v *StandardValidator) Config() config.ValidatorCfg {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.cfg
}

// ValidateUsername 验证用户名
//
// 规则:
//  1. 去除首尾空白后不能为空
//  2. 长度在 MinUsernameLen ~ MaxUsernameLen 之间
//  3. 只允许: 字母、数字、下划线、中文、日文假名
//  4. 不能使用系统保留名称
func (v *StandardValidator) ValidateUsername(name string) error {
	name = strings.TrimSpace(name)
	runes := []rune(name)

	if len(runes) < v.cfg.MinUsernameLen {
		return fmt.Errorf("用户名至少需要 %d 个字符", v.cfg.MinUsernameLen)
	}
	if len(runes) > v.cfg.MaxUsernameLen {
		return fmt.Errorf("用户名不能超过 %d 个字符 (当前 %d)", v.cfg.MaxUsernameLen, len(runes))
	}

	for _, r := range runes {
		if !isAllowedRune(r) {
			return fmt.Errorf("用户名包含非法字符: %q", r)
		}
	}

	if isReserved(name) {
		return fmt.Errorf("用户名 %q 是系统保留名称，请换一个", name)
	}

	return nil
}

// ValidateContent 验证消息内容
//
// 规则:
//  1. 去除首尾空白后不能为空
//  2. 不能超过 MaxMessageLen 字符
//  3. 不能全是空白字符
//  4. 不包含控制字符 (允许 \n \r \t)
func (v *StandardValidator) ValidateContent(content string) error {
	content = strings.TrimSpace(content)
	runes := []rune(content)

	if len(runes) == 0 {
		return fmt.Errorf("消息不能为空")
	}
	if len(runes) > v.cfg.MaxMessageLen {
		return fmt.Errorf("消息过长 (最大 %d 字符)", v.cfg.MaxMessageLen)
	}

	allSpace := true
	for _, r := range runes {
		if !unicode.IsSpace(r) {
			allSpace = false
		}
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return fmt.Errorf("消息包含非法控制字符 (U+%04X)", r)
		}
	}
	if allSpace {
		return fmt.Errorf("消息不能全是空白字符")
	}

	return nil
}

// ============================================================
// 辅助函数
// ============================================================

func isAllowedRune(r rune) bool {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return true
	}
	if r == '_' {
		return true
	}
	if unicode.Is(unicode.Han, r) {
		return true
	}
	if unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
		return true
	}
	return false
}

func isReserved(name string) bool {
	reserved := map[string]bool{
		"system": true, "server": true,
		"root": true, "all": true, "everyone": true,
	}
	return reserved[strings.ToLower(name)]
}

// TrimName 清理用户名首尾空白
func TrimName(name string) string {
	return strings.TrimSpace(name)
}

// TrimContent 清理消息首尾空白
func TrimContent(content string) string {
	return strings.TrimSpace(content)
}

// DefaultConfig 返回默认验证配置
func DefaultConfig() config.ValidatorCfg {
	return config.DefaultConfig().Validator
}
