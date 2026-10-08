package auth

import (
	"context"
	"errors"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
)

// User 是聊天室使用的用户账号。
type User struct {
	ID       uint64
	Username string
}

// UserStore 定义用户认证和持久化需要的方法。
type UserStore interface {
	AuthenticateOrCreate(ctx context.Context, username, password string) (*User, bool, error)
	Close() error
}
