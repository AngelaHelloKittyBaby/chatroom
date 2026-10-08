// Package protocol 协议层错误定义
//
// 面向对象设计:
//   - 类型化错误，调用方可用 errors.Is / errors.As 判断
//   - 每个错误类型携带上下文信息
package protocol

import "fmt"

// --- 错误类型 ---

// ProtocolError 协议层错误 (实现 error 接口)
type ProtocolError struct {
	Code    string // 错误码 (机器可读)
	Message string // 错误描述 (人类可读)
	Cause   error  // 原始错误
}

func (e *ProtocolError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *ProtocolError) Unwrap() error {
	return e.Cause
}

// --- 预定义错误 ---

var (
	ErrInvalidMessageType = &ProtocolError{Code: "INVALID_TYPE", Message: "无效的消息类型"}
	ErrMessageTooLarge    = &ProtocolError{Code: "MSG_TOO_LARGE", Message: "消息超过大小限制"}
	ErrEmptyMessage       = &ProtocolError{Code: "EMPTY_MSG", Message: "消息不能为空"}
	ErrConnectionClosed   = &ProtocolError{Code: "CONN_CLOSED", Message: "连接已关闭"}
	ErrReadTimeout        = &ProtocolError{Code: "READ_TIMEOUT", Message: "读取超时"}
	ErrWriteTimeout       = &ProtocolError{Code: "WRITE_TIMEOUT", Message: "写入超时"}
)

// NewProtocolError 创建协议错误
func NewProtocolError(code, message string, cause error) *ProtocolError {
	return &ProtocolError{Code: code, Message: message, Cause: cause}
}

// WrapError 包装错误
func WrapError(code string, cause error) error {
	return &ProtocolError{
		Code:    code,
		Message: cause.Error(),
		Cause:   cause,
	}
}
