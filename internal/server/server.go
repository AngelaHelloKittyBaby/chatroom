// Package server — 聊天室服务器
//
// 面向对象设计:
//   - Server 是顶层编排器 (Facade Pattern)
//   - 管理所有子组件的生命周期
//   - 通过依赖注入组装组件
//   - 实现 Lifecycle 接口
//
// 架构层次:
//
//	Server (编排器)
//	├── RoomManager (房间管理)
//	│   └── Room (聚合根, Actor模式)
//	│       └── Client (状态机)
//	├── RateLimiter (令牌桶)
package server

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"chatroom/internal/auth"
	"chatroom/internal/config"
	"chatroom/internal/database"
	"chatroom/internal/protocol"
	"chatroom/internal/redisstore"
	"chatroom/internal/validator"
)

// ============================================================
// Server 顶层对象
// ============================================================

// Server 聊天室服务器
//
// 职责:
//   - TCP 连接监听与分发
//   - 组件生命周期管理
//   - 优雅关闭
//   - 连接数量控制
type Server struct {
	cfg         *config.Config
	listener    net.Listener
	roomMgr     *DefaultRoomManager
	rateLimiter RateLimiter
	validator   validator.MessageValidator
	userStore   auth.UserStore
	redis       *redisstore.Store
	slots       chan struct{}

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewServer 创建服务器 (构造函数 + 依赖注入)
func NewServer(cfg *config.Config) (*Server, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// 组装依赖
	v := validator.NewStandardValidator(cfg.Validator)
	var userStore auth.UserStore
	if cfg.Database.Enable {
		dbCtx, dbCancel := context.WithTimeout(ctx, 10*time.Second)
		defer dbCancel()

		store, err := database.OpenMySQL(dbCtx, cfg.Database)
		if err != nil {
			cancel()
			return nil, err
		}
		userStore = store
	}
	var redisStore *redisstore.Store
	if cfg.Redis.Enable {
		redisCtx, redisCancel := context.WithTimeout(ctx, 5*time.Second)
		var err error
		redisStore, err = redisstore.Open(redisCtx, cfg.Redis)
		redisCancel()
		if err != nil {
			cancel()
			if userStore != nil {
				userStore.Close()
			}
			return nil, err
		}
	}
	roomMgr := NewRoomManager()
	rl := NewTokenBucket(cfg.RateLimit)

	srv := &Server{
		cfg:         cfg,
		redis:       redisStore,
		roomMgr:     roomMgr,
		rateLimiter: rl,
		validator:   v,
		userStore:   userStore,
		slots:       make(chan struct{}, cfg.Server.MaxConnections),
		ctx:         ctx,
		cancel:      cancel,
	}

	return srv, nil
}

// ============================================================
// Lifecycle 接口实现
// ============================================================

// Start 启动服务器
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)

	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听 %s 失败: %w", addr, err)
	}

	log.Printf("🚀 服务器启动: %s", addr)
	log.Printf("👥 最大连接数: %d", s.cfg.Server.MaxConnections)
	log.Printf("💓 心跳间隔: %v, 最大失败: %d", s.cfg.Heartbeat.Interval(), s.cfg.Heartbeat.MaxFailures)
	log.Printf("🚦 限流: %v, %d条/秒", s.cfg.RateLimit.Enabled(), s.cfg.RateLimit.MessagesPerSecond())
	if s.userStore != nil {
		log.Printf("🗄️ MySQL 用户持久化已启用")
	} else {
		log.Printf("🗄️ MySQL 用户持久化未启用")
	}

	// 接受连接循环
	if s.redis != nil {
		s.wg.Add(1)
		go func() { defer s.wg.Done(); s.redis.Consume(s.ctx, s.deliverQueued) }()
	}
	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop 优雅关闭服务器
func (s *Server) Stop() error {
	log.Println("🛑 正在关闭服务器...")

	s.cancel()

	// 关闭监听器 (不再接受新连接)
	if s.listener != nil {
		s.listener.Close()
	}

	// 关闭所有房间
	for _, room := range s.roomMgr.ListRooms() {
		room.Close()
	}
	s.rateLimiter.Close()
	if s.userStore != nil {
		if err := s.userStore.Close(); err != nil {
			log.Printf("⚠️ 关闭用户存储失败: %v", err)
		}
	}

	// 等待所有 goroutine 完成
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("✅ 所有连接已安全关闭")
	case <-time.After(30 * time.Second):
		log.Println("⚠️ 关闭超时 (30s)")
	}

	if s.redis != nil {
		s.redis.Close()
	}
	return nil
}

// Wait 等待服务器停止
func (s *Server) Wait() {
	s.wg.Wait()
}

// ============================================================
// 连接处理
// ============================================================

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return // 正常关闭
			default:
				log.Printf("⚠️ 接受连接失败: %v", err)
				continue
			}
		}

		if !s.acquireSlot() {
			log.Printf("⚠️ 连接数已满，拒绝: %s", conn.RemoteAddr())
			sendErrorToConn(conn, "服务器连接数已满，请稍后再试")
			conn.Close()
			continue
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer s.releaseSlot()
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	log.Printf("🔗 新连接: %s", remoteAddr)

	// --- 身份验证阶段 (循环，允许重试) ---
	room := s.roomMgr.DefaultRoom()
	var cleanName string
	var userID uint64

	for {
		conn.SetReadDeadline(time.Now().Add(s.cfg.Server.ReadTimeout()))

		msg, err := protocol.ReadMessage(conn)
		if err != nil {
			log.Printf("⚠️ 读取加入请求失败 [%s]: %v", remoteAddr, err)
			sendErrorToConn(conn, "读取加入请求超时，请重试")
			return
		}

		if msg.Type != protocol.TypeJoin {
			sendErrorToConn(conn, "请先发送加入请求 (type=join)")
			return
		}

		rawName := msg.From
		cleanName = validator.TrimName(rawName)

		// 验证用户名格式
		if err := s.validator.ValidateUsername(cleanName); err != nil {
			sendErrorToConn(conn, fmt.Sprintf("用户名不合法: %v", err))
			log.Printf("❌ 用户名验证失败 [%s]: %v", remoteAddr, err)
			continue // 让客户端重新输入
		}

		// 检查重名
		if room.IsNameTaken(cleanName) {
			sendErrorToConn(conn, fmt.Sprintf("用户名 %q 已被占用，请换一个", cleanName))
			log.Printf("❌ 用户名冲突 [%s]: %s", remoteAddr, cleanName)
			continue // 让客户端重新输入
		}

		if s.userStore != nil {
			authCtx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
			user, created, err := s.userStore.AuthenticateOrCreate(authCtx, cleanName, msg.Password)
			cancel()
			if err != nil {
				sendErrorToConn(conn, authErrorMessage(err))
				log.Printf("❌ 用户认证失败 [%s]: %s (%v)", remoteAddr, cleanName, err)
				continue
			}
			cleanName = user.Username
			userID = user.ID
			if s.redis != nil {
				if err := s.cacheUser(user); err != nil {
					log.Printf("cache user: %v", err)
				}
			}
			if created {
				log.Printf("✅ 新用户注册成功 [%s]: %s (id=%d)", remoteAddr, user.Username, user.ID)
			} else {
				log.Printf("✅ 用户登录成功 [%s]: %s (id=%d)", remoteAddr, user.Username, user.ID)
			}
		}

		// 验证通过
		break
	}

	// --- 创建客户端并加入房间 ---
	clientID := generateID()
	client := NewClient(clientID, cleanName, conn, room)
	client.server = s
	client.userID = userID

	// 发送欢迎消息
	client.Send(protocol.NewWelcomeMessage(
		fmt.Sprintf("🎉 欢迎 %s 来到聊天室！", cleanName),
	))
	client.Send(protocol.NewSystemMessage(
		"💡 /help 查看命令 | /msg 用户名 消息 私聊 | /users 在线 | /quit 退出",
	))

	room.Join(client)

	// --- 启动 I/O 协程 ---
	var wg sync.WaitGroup

	// 写协程
	wg.Add(1)
	go func() {
		defer wg.Done()
		WriteLoop(client, s.cfg.Server.WriteTimeout())
	}()

	// 心跳协程
	heartbeat := NewHeartbeat(s.cfg.Heartbeat, client, &wg, func(c *Client) {
		log.Printf("💀 [%s] 心跳检测失败，断开连接", c.Name())
		c.Room().Leave(c)
	})
	heartbeat.Start()

	// 读协程 (阻塞直到断开)
	ReadLoop(client, s.validator, s.rateLimiter, s.cfg.Server.ReadTimeout())
	s.rateLimiter.Reset(client.ID())

	// 等待写协程完成
	wg.Wait()
	log.Printf("🔌 [%s] 连接已完全关闭", cleanName)
}

func (s *Server) acquireSlot() bool {
	select {
	case s.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *Server) releaseSlot() {
	select {
	case <-s.slots:
	default:
	}
}

// ============================================================
// 工具函数
// ============================================================

// generateID 生成唯一客户端 ID
func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// sendErrorToConn 向 TCP 连接发送错误消息
func sendErrorToConn(conn net.Conn, errMsg string) {
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	protocol.WriteMessage(conn, protocol.NewErrorMessage(errMsg))
}

func authErrorMessage(err error) string {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		return "用户名或密码错误；如果是新用户，请输入非空密码完成注册"
	default:
		return "用户认证失败，请稍后重试"
	}
}

// ============================================================
// 主入口辅助
// ============================================================

// RunServer 一站式启动服务器 (处理信号和优雅关闭)
//
// 用法:
//
//	cfg := config.DefaultConfig()
//	if err := server.RunServer(cfg); err != nil {
//	    log.Fatal(err)
//	}
func RunServer(cfg *config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	srv, err := NewServer(cfg)
	if err != nil {
		return err
	}

	if err := srv.Start(); err != nil {
		srv.Stop()
		return err
	}

	// 等待信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	return srv.Stop()
}
