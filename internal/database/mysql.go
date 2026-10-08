package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"chatroom/internal/config"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLStore 保存聊天室使用的 MySQL 数据库连接。
type MySQLStore struct {
	db *sql.DB
}

// OpenMySQL 连接 MySQL；配置开启自动建表时，同时创建表和默认数据。
func OpenMySQL(ctx context.Context, cfg config.DatabaseCfg) (*MySQLStore, error) {
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	if cfg.AutoMigrate {
		if err := migrate(ctx, db); err != nil {
			db.Close()
			return nil, err
		}
	}

	return &MySQLStore{db: db}, nil
}

func (s *MySQLStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// migrate 创建用户、角色、权限、菜单相关表，并写入默认数据。
func migrate(ctx context.Context, db *sql.DB) error {
	for _, stmt := range schemaStatements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate schema: %w", err)
		}
	}
	for _, stmt := range seedStatements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("seed schema: %w", err)
		}
	}
	return nil
}
