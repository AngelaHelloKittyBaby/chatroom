package server

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"chatroom/internal/auth"
	"chatroom/internal/config"
	"chatroom/internal/protocol"
	"chatroom/internal/redisstore"
	"github.com/redis/go-redis/v9"
)

func testRedis(t *testing.T) string {
	t.Helper()
	binary, err := exec.LookPath("redis-server")
	if err != nil {
		t.Skip("redis-server is required for integration tests")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	cmd := exec.Command(binary, "--bind", "127.0.0.1", "--port", fmt.Sprint(port), "--save", "", "--appendonly", "no", "--dir", filepath.ToSlash(t.TempDir()))
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	c := redis.NewClient(&redis.Options{Addr: addr})
	defer c.Close()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if c.Ping(context.Background()).Err() == nil {
			return addr
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("redis did not start")
	return ""
}

func TestRedisQueueRecoveryRankingAndCache(t *testing.T) {
	cfg := config.DefaultConfig().Redis
	cfg.Addr = testRedis(t)
	cfg.CacheTTLSec = 1
	ctx := context.Background()
	store, err := redisstore.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = store.CacheUser(ctx, &auth.User{ID: 7, Username: "alice"}); err != nil {
		t.Fatal(err)
	}
	u, err := store.User(ctx, 7)
	if err != nil || u.Username != "alice" {
		t.Fatalf("cache: %v %v", u, err)
	}
	ttl := store.Client.TTL(ctx, cfg.Prefix+"user:7").Val()
	if ttl <= 0 || ttl > time.Second {
		t.Fatalf("ttl: %v", ttl)
	}
	store.Client.PExpire(ctx, cfg.Prefix+"user:7", time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	if _, err = store.User(ctx, 7); err != redis.Nil {
		t.Fatalf("cache should expire: %v", err)
	}
	for _, name := range []string{"alice", "bob", "alice"} {
		if err = store.Enqueue(ctx, "lobby", protocol.NewChatMessage(name, "hello")); err != nil {
			t.Fatal(err)
		}
	}
	ranks, err := store.Ranking(ctx, 10)
	if err != nil || len(ranks) != 2 || ranks[0].Member != "alice" || ranks[0].Score != 2 {
		t.Fatalf("ranks: %v %v", ranks, err)
	}
	// Simulate a server crash after reading, before ACK.
	_, err = store.Client.XReadGroup(ctx, &redis.XReadGroupArgs{Group: "delivery", Consumer: "server", Streams: []string{cfg.Prefix + "messages", ">"}, Count: 1}).Result()
	if err != nil {
		t.Fatal(err)
	}
	consumeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	received := make(chan redisstore.Event, 3)
	done := make(chan struct{})
	go func() {
		defer close(done)
		store.Consume(consumeCtx, func(e redisstore.Event) error { received <- e; return nil })
	}()
	for i := 0; i < 3; i++ {
		select {
		case <-received:
		case <-time.After(5 * time.Second):
			t.Fatal("message was lost during recovery")
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && store.Client.XLen(ctx, cfg.Prefix+"messages").Val() != 0 {
		time.Sleep(20 * time.Millisecond)
	}
	if store.Client.XLen(ctx, cfg.Prefix+"messages").Val() != 0 {
		t.Fatal("acknowledged entries were not cleaned up")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not stop")
	}
}

func TestRedisChatCommandsAndPrivateDelivery(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.Host = "127.0.0.1"
	cfg.Redis.Enable = true
	cfg.Redis.Addr = testRedis(t)
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Use an OS-assigned listening port for isolated TCP integration tests.
	srv.cfg.Server.Port = 0
	if err = srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()
	connect := func(name string) net.Conn {
		c, e := net.Dial("tcp", srv.listener.Addr().String())
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { c.Close() })
		protocol.WriteMessage(c, protocol.NewJoinMessage(name, "password"))
		for {
			m := readTestMessage(t, c)
			if m.Type == protocol.TypeUsers {
				break
			}
		}
		return c
	}
	alice := connect("alice")
	bob := connect("bob")
	protocol.WriteMessage(alice, protocol.NewChatMessage("forged", "hello"))
	for _, c := range []net.Conn{alice, bob} {
		for {
			m := readTestMessage(t, c)
			if m.Type == protocol.TypeChat {
				if m.From != "alice" || m.Content != "hello" {
					t.Fatal(m)
				}
				break
			}
		}
	}
	protocol.WriteMessage(alice, protocol.NewChatMessage("forged", "/msg bob secret"))
	for _, c := range []net.Conn{alice, bob} {
		for {
			m := readTestMessage(t, c)
			if m.Type == protocol.TypePrivate {
				if m.From != "alice" || m.To != "bob" || m.Content != "secret" {
					t.Fatal(m)
				}
				break
			}
		}
	}
	protocol.WriteMessage(alice, protocol.NewChatMessage("alice", "/rank"))
	m := readTestMessage(t, alice)
	if m.Type != protocol.TypeSystem || !strings.Contains(m.Content, "alice: 2") {
		t.Fatalf("ranking response: %+v", m)
	}
	protocol.WriteMessage(alice, protocol.NewChatMessage("alice", "/rank 101"))
	if m = readTestMessage(t, alice); m.Type != protocol.TypeError {
		t.Fatal(m)
	}
	// Commands and invalid private recipients must not increase activity.
	protocol.WriteMessage(alice, protocol.NewChatMessage("alice", "/msg nobody ignored"))
	if m = readTestMessage(t, alice); m.Type != protocol.TypeError {
		t.Fatal(m)
	}
	ranks, err := srv.redis.Ranking(context.Background(), 10)
	if err != nil || len(ranks) != 1 || ranks[0].Score != 2 {
		t.Fatalf("unexpected activity: %v %v", ranks, err)
	}
	alice.Close()
	bob.Close()
}
func readTestMessage(t *testing.T, c net.Conn) *protocol.Message {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	m, err := protocol.ReadMessage(c)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// profileStore records database reads to verify cache hits and fallback.
type profileStore struct{ reads int }

func (s *profileStore) AuthenticateOrCreate(context.Context, string, string) (*auth.User, bool, error) {
	return nil, false, auth.ErrInvalidCredentials
}
func (s *profileStore) Close() error { return nil }
func (s *profileStore) FindUser(context.Context, uint64) (*auth.User, error) {
	s.reads++
	return &auth.User{ID: 7, Username: "alice"}, nil
}
func TestProfileCacheHitAndDatabaseFallback(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Redis.Enable = true
	cfg.Redis.Addr = testRedis(t)
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()
	db := &profileStore{}
	srv.userStore = db
	client := &Client{userID: 7, sendCh: make(chan *protocol.Message, 5)}
	srv.showProfile(client)
	if db.reads != 1 {
		t.Fatal("cache miss did not read database")
	}
	srv.showProfile(client)
	if db.reads != 1 {
		t.Fatal("cache hit read database")
	}
	// A corrupt cache entry must not prevent loading the authoritative profile.
	srv.redis.Client.Set(context.Background(), cfg.Redis.Prefix+"user:7", "broken json", time.Minute)
	srv.showProfile(client)
	if db.reads != 2 {
		t.Fatal("corrupt cache did not fall back")
	}
	srv.redis.Close()
	srv.showProfile(client)
	if db.reads != 3 {
		t.Fatal("Redis outage did not fall back")
	}
	for i := 0; i < 4; i++ {
		m := <-client.sendCh
		if m.Type != protocol.TypeSystem || !strings.Contains(m.Content, "alice") {
			t.Fatalf("profile response: %+v", m)
		}
	}
}
