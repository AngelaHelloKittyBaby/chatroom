package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// LoadFile 读取项目内的简单 YAML 配置。
//
// 这个加载器只支持本项目 config.yaml 使用的二级 key/value 结构，
// 目的是避免为一个小项目引入额外依赖。
func LoadFile(path string) (*Config, error) {
	cfg := DefaultConfig()
	if strings.TrimSpace(path) == "" {
		return cfg, nil
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("打开配置文件失败: %w", err)
	}
	defer file.Close()

	section := ""
	scanner := bufio.NewScanner(file)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := cleanLine(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasSuffix(line, ":") {
			section = strings.TrimSuffix(line, ":")
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("配置第 %d 行格式错误: %s", lineNo, line)
		}

		key := strings.TrimSpace(parts[0])
		value := trimValue(parts[1])
		if err := applyValue(cfg, section, key, value); err != nil {
			return nil, fmt.Errorf("配置第 %d 行错误: %w", lineNo, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := applyEnvOverrides(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func cleanLine(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
}

func trimValue(value string) string {
	value = strings.TrimSpace(value)
	return strings.Trim(value, `"'`)
}

func applyValue(cfg *Config, section, key, value string) error {
	fullKey := section + "." + key

	switch fullKey {
	case "server.host":
		cfg.Server.Host = value
	case "server.port":
		return setInt(value, &cfg.Server.Port)
	case "server.max_connections":
		return setInt(value, &cfg.Server.MaxConnections)
	case "server.read_timeout_sec":
		return setInt(value, &cfg.Server.ReadTimeoutSec)
	case "server.write_timeout_sec":
		return setInt(value, &cfg.Server.WriteTimeoutSec)

	case "heartbeat.interval_sec":
		return setInt(value, &cfg.Heartbeat.IntervalSec)
	case "heartbeat.timeout_sec":
		return setInt(value, &cfg.Heartbeat.TimeoutSec)
	case "heartbeat.max_failures":
		return setInt(value, &cfg.Heartbeat.MaxFailures)

	case "rate_limit.enable":
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("%s 需要是布尔值", fullKey)
		}
		cfg.RateLimit.Enable = enabled
	case "rate_limit.messages_per_second":
		return setInt(value, &cfg.RateLimit.MsgsPerSec)
	case "rate_limit.burst_size":
		return setInt(value, &cfg.RateLimit.Burst)

	case "validator.min_username_len":
		return setInt(value, &cfg.Validator.MinUsernameLen)
	case "validator.max_username_len":
		return setInt(value, &cfg.Validator.MaxUsernameLen)
	case "validator.max_message_len":
		return setInt(value, &cfg.Validator.MaxMessageLen)

	case "database.enable":
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("%s 需要是布尔值", fullKey)
		}
		cfg.Database.Enable = enabled
	case "database.dsn":
		cfg.Database.DSN = value
	case "database.auto_migrate":
		autoMigrate, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("%s 需要是布尔值", fullKey)
		}
		cfg.Database.AutoMigrate = autoMigrate
	}

	return nil
}

func applyEnvOverrides(cfg *Config) error {
	if value := strings.TrimSpace(os.Getenv("CHATROOM_DB_ENABLE")); value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("CHATROOM_DB_ENABLE 需要是布尔值")
		}
		cfg.Database.Enable = enabled
	}
	if value := strings.TrimSpace(os.Getenv("CHATROOM_DB_DSN")); value != "" {
		cfg.Database.DSN = value
	}
	if value := strings.TrimSpace(os.Getenv("CHATROOM_DB_AUTO_MIGRATE")); value != "" {
		autoMigrate, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("CHATROOM_DB_AUTO_MIGRATE 需要是布尔值")
		}
		cfg.Database.AutoMigrate = autoMigrate
	}
	return nil
}

func setInt(value string, target *int) error {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("%q 需要是整数", value)
	}
	*target = n
	return nil
}
