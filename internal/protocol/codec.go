package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// MaxMessageSize 单条消息最大字节数 (64KB)
const MaxMessageSize = 64 * 1024

// ============================================================
// Codec: 消息编解码器 (Strategy Pattern — 可替换为 Protobuf 等)
// ============================================================

// MessageCodec 消息编解码接口
type MessageCodec interface {
	Encode(w io.Writer, msg *Message) (int, error)
	Decode(r io.Reader) (*Message, error)
}

// LengthPrefixCodec 长度前缀编解码器实现
//
// 帧格式: [4字节大端序 uint32 = 负载长度][JSON 负载]
// 每次精确读取一条完整消息，彻底解决 TCP 粘包/拆包问题
type LengthPrefixCodec struct {
	maxSize uint32
}

// NewLengthPrefixCodec 创建编解码器
func NewLengthPrefixCodec() *LengthPrefixCodec {
	return &LengthPrefixCodec{maxSize: MaxMessageSize}
}

// Encode 将消息编码为长度前缀帧
func (c *LengthPrefixCodec) Encode(w io.Writer, msg *Message) (int, error) {
	if !msg.Type.IsValid() {
		return 0, fmt.Errorf("%w: %s", ErrInvalidMessageType, msg.Type)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return 0, fmt.Errorf("序列化消息失败: %w", err)
	}

	if uint32(len(data)) > c.maxSize {
		return 0, fmt.Errorf("%w: %d 字节 (最大 %d)", ErrMessageTooLarge, len(data), c.maxSize)
	}

	frame := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(frame[:4], uint32(len(data)))
	copy(frame[4:], data)

	n, err := w.Write(frame)
	if err != nil {
		return n, fmt.Errorf("写入消息帧失败: %w", err)
	}
	if n != len(frame) {
		return n, io.ErrShortWrite
	}
	return n, nil
}

// Decode 从 Reader 读取一个完整帧并解码
func (c *LengthPrefixCodec) Decode(r io.Reader) (*Message, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("读取长度头失败: %w", err)
	}

	length := binary.BigEndian.Uint32(header)

	if length > c.maxSize {
		return nil, fmt.Errorf("%w: 声明长度 %d", ErrMessageTooLarge, length)
	}
	if length == 0 {
		return nil, ErrEmptyMessage
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, fmt.Errorf("读取消息体失败: %w", err)
	}

	var msg Message
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, fmt.Errorf("反序列化失败: %w", err)
	}

	if !msg.Type.IsValid() {
		return nil, fmt.Errorf("%w: %s", ErrInvalidMessageType, msg.Type)
	}

	return &msg, nil
}

// --- 便捷函数 (保持向后兼容) ---

// DefaultCodec 全局默认编解码器
var DefaultCodec = NewLengthPrefixCodec()

// WriteMessage 使用默认编解码器写入消息
func WriteMessage(w io.Writer, msg *Message) (int, error) {
	return DefaultCodec.Encode(w, msg)
}

// ReadMessage 使用默认编解码器读取消息
func ReadMessage(r io.Reader) (*Message, error) {
	return DefaultCodec.Decode(r)
}
