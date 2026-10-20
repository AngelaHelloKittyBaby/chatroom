package config

import "testing"

func TestRedisEnvironmentWithoutConfigFile(t *testing.T) {
	t.Setenv("CHATROOM_REDIS_ENABLE", "true")
	t.Setenv("CHATROOM_REDIS_ADDR", "localhost:6380")
	t.Setenv("CHATROOM_REDIS_CACHE_TTL_SEC", "60")
	for _, path := range []string{"", t.TempDir() + "/missing.yaml"} {
		cfg, err := LoadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !cfg.Redis.Enable || cfg.Redis.Addr != "localhost:6380" || cfg.Redis.CacheTTLSec != 60 {
			t.Fatalf("environment ignored: %+v", cfg.Redis)
		}
		if err = cfg.Validate(); err != nil {
			t.Fatal(err)
		}
		cfg.Redis.CacheTTLSec = 0
		if cfg.Validate() == nil {
			t.Fatal("zero cache TTL accepted")
		}
	}
}
