package redisstore

import (
	"chatroom/internal/auth"
	"chatroom/internal/config"
	"chatroom/internal/protocol"
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"time"
)

type Store struct {
	Client *redis.Client
	cfg    config.RedisCfg
}
type Event struct {
	Room    string            `json:"room"`
	Message *protocol.Message `json:"message"`
}

func Open(ctx context.Context, cfg config.RedisCfg) (*Store, error) {
	c := redis.NewClient(&redis.Options{Addr: cfg.Addr, Password: cfg.Password, DB: cfg.DB, DialTimeout: 3 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second})
	if err := c.Ping(ctx).Err(); err != nil {
		c.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	s := &Store{c, cfg}
	if err := c.XGroupCreateMkStream(ctx, s.key("messages"), "delivery", "0").Err(); err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		c.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) key(v string) string { return s.cfg.Prefix + v }
func (s *Store) Close() error        { return s.Client.Close() }

func (s *Store) Enqueue(ctx context.Context, room string, msg *protocol.Message) error {
	data, err := json.Marshal(Event{room, msg})
	if err != nil {
		return err
	}
	return s.Client.Eval(ctx, `redis.call('XADD', KEYS[1], '*', 'event', ARGV[1]); redis.call('ZINCRBY', KEYS[2], 1, ARGV[2]); return 1`, []string{s.key("messages"), s.key("activity")}, string(data), msg.From).Err()
}
func (s *Store) Ranking(ctx context.Context, n int64) ([]redis.Z, error) {
	return s.Client.ZRevRangeWithScores(ctx, s.key("activity"), 0, n-1).Result()
}
func (s *Store) CacheUser(ctx context.Context, u *auth.User) error {
	b, e := json.Marshal(u)
	if e != nil {
		return e
	}
	return s.Client.Set(ctx, s.key(fmt.Sprintf("user:%d", u.ID)), b, time.Duration(s.cfg.CacheTTLSec)*time.Second).Err()
}
func (s *Store) User(ctx context.Context, id uint64) (*auth.User, error) {
	b, e := s.Client.Get(ctx, s.key(fmt.Sprintf("user:%d", id))).Bytes()
	if e != nil {
		return nil, e
	}
	var u auth.User
	if e = json.Unmarshal(b, &u); e != nil {
		return nil, e
	}
	if u.ID != id || u.Username == "" {
		return nil, fmt.Errorf("invalid cached user")
	}
	return &u, nil
}

func (s *Store) Consume(ctx context.Context, deliver func(Event) error) {
	pending := true
	for ctx.Err() == nil {
		cursor := ">"
		if pending {
			cursor = "0"
		}
		streams, err := s.Client.XReadGroup(ctx, &redis.XReadGroupArgs{Group: "delivery", Consumer: "server", Streams: []string{s.key("messages"), cursor}, Count: 1, Block: time.Second}).Result()
		if err == redis.Nil || (err == nil && (len(streams) == 0 || len(streams[0].Messages) == 0)) {
			pending = false
			continue
		}
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}
		for _, stream := range streams {
			for _, m := range stream.Messages {
				var event Event
				raw, _ := m.Values["event"].(string)
				if err = json.Unmarshal([]byte(raw), &event); err == nil && event.Message != nil {
					err = deliver(event)
				} else {
					err = fmt.Errorf("invalid queue event")
				}
				if err != nil {
					pending = true
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Second):
					}
					continue
				}

				_, err = s.Client.TxPipelined(ctx, func(p redis.Pipeliner) error {
					p.XAck(ctx, s.key("messages"), "delivery", m.ID)
					p.XDel(ctx, s.key("messages"), m.ID)
					return nil
				})
				if err != nil {
					pending = true
				}
			}
		}
	}
}
