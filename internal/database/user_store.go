package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"chatroom/internal/auth"
)

const defaultRoleCode = "member"

// AuthenticateOrCreate 校验已有用户；用户不存在时自动注册并分配 member 角色。
func (s *MySQLStore) AuthenticateOrCreate(ctx context.Context, username, password string) (*auth.User, bool, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, false, auth.ErrInvalidCredentials
	}

	// 登录和注册必须放在同一个事务里，避免同时注册出重复账号。
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("begin auth tx: %w", err)
	}
	defer tx.Rollback()

	user, hash, err := findUserForUpdate(ctx, tx, username)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		created, err := s.createUser(ctx, tx, username, password)
		if err != nil {
			return nil, false, err
		}
		if err := tx.Commit(); err != nil {
			return nil, false, fmt.Errorf("commit user registration: %w", err)
		}
		return created, true, nil
	case err != nil:
		return nil, false, err
	}

	if !auth.VerifyPassword(password, hash) {
		return nil, false, auth.ErrInvalidCredentials
	}

	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("commit user login: %w", err)
	}

	return user, false, nil
}

func (s *MySQLStore) createUser(ctx context.Context, tx *sql.Tx, username, password string) (*auth.User, error) {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO sys_users (username, password_hash)
		VALUES (?, ?)
	`, username, hash)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("read user id: %w", err)
	}

	if err := assignDefaultRole(ctx, tx, uint64(id)); err != nil {
		return nil, err
	}

	return &auth.User{ID: uint64(id), Username: username}, nil
}

func findUserForUpdate(ctx context.Context, tx *sql.Tx, username string) (*auth.User, string, error) {
	var user auth.User
	var hash string

	err := tx.QueryRowContext(ctx, `
		SELECT id, username, password_hash
		FROM sys_users
		WHERE username = ?
		FOR UPDATE
	`, username).Scan(&user.ID, &user.Username, &hash)
	if err != nil {
		return nil, "", err
	}
	return &user, hash, nil
}

func assignDefaultRole(ctx context.Context, tx *sql.Tx, userID uint64) error {
	var roleID uint64
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM sys_roles
		WHERE role_code = ?
		FOR UPDATE
	`, defaultRoleCode).Scan(&roleID)
	if errors.Is(err, sql.ErrNoRows) {
		result, insertErr := tx.ExecContext(ctx, `
			INSERT INTO sys_roles (role_code, role_name)
			VALUES (?, ?)
		`, defaultRoleCode, "member")
		if insertErr != nil {
			return fmt.Errorf("insert default role: %w", insertErr)
		}
		id, idErr := result.LastInsertId()
		if idErr != nil {
			return fmt.Errorf("read role id: %w", idErr)
		}
		roleID = uint64(id)
	} else if err != nil {
		return fmt.Errorf("find default role: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO sys_user_roles (user_id, role_id)
		VALUES (?, ?)
	`, userID, roleID); err != nil {
		return fmt.Errorf("assign default role: %w", err)
	}
	return nil
}

// FindUser retrieves public profile data without reading password hashes.
func (s *MySQLStore) FindUser(ctx context.Context, id uint64) (*auth.User, error) {
	var user auth.User
	err := s.db.QueryRowContext(ctx, "SELECT id, username FROM sys_users WHERE id = ?", id).Scan(&user.ID, &user.Username)
	return &user, err
}
