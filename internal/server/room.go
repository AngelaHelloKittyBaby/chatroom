// Package server — 聊天室房间
//
// 面向对象设计:
//   - Room 是聚合根 (Aggregate Root)，管理一组 Client
//   - Actor 模式: Room 通过 channel 接收命令，线程安全
//   - 广播时对慢客户端采用优雅降级策略
package server

import (
	"fmt"
	"log"
	"sync"
	"time"

	"chatroom/internal/protocol"
)

// ============================================================
// 入队命令 (Actor 模式)
// ============================================================

// roomCmd 房间命令
type roomCmd int

const (
	cmdJoin roomCmd = iota
	cmdLeave
	cmdBroadcast
)

// roomAction 房间操作请求
type roomAction struct {
	cmd    roomCmd
	client *Client
	msg    *protocol.Message
}

// ============================================================
// Room 聚合根
// ============================================================

// Room 聊天室房间
//
// 职责:
//   - 管理房间内的客户端集合
//   - 消息广播 (推模型)
//   - 在线用户列表维护
//   - 用户名唯一性保证
type Room struct {
	name    string
	mu      sync.RWMutex
	clients map[string]*Client // key = client ID
	actions chan roomAction    // Actor 命令通道
	done    chan struct{}
	once    sync.Once

	// 统计
	createdAt time.Time
	msgCount  int64
}

// NewRoom 创建房间 (构造函数)
func NewRoom(name string) *Room {
	r := &Room{
		name:      name,
		clients:   make(map[string]*Client),
		actions:   make(chan roomAction, 256),
		done:      make(chan struct{}),
		createdAt: time.Now(),
	}
	go r.run() // Actor 循环
	return r
}

// --- Actor 主循环 ---

func (r *Room) run() {
	for {
		select {
		case <-r.done:
			return
		case action := <-r.actions:
			switch action.cmd {
			case cmdJoin:
				r.handleJoin(action.client)
			case cmdLeave:
				r.handleLeave(action.client)
			case cmdBroadcast:
				r.handleBroadcast(action.msg)
			}
		}
	}
}

// --- 命令接口 (线程安全，通过 channel 投递) ---

// Join 客户端加入房间
func (r *Room) Join(client *Client) {
	r.submit(roomAction{cmd: cmdJoin, client: client})
}

// Leave 客户端离开房间
func (r *Room) Leave(client *Client) {
	r.submit(roomAction{cmd: cmdLeave, client: client})
}

// Broadcast 广播消息到房间所有客户端
func (r *Room) Broadcast(msg *protocol.Message) {
	r.submit(roomAction{cmd: cmdBroadcast, msg: msg})
}

func (r *Room) submit(action roomAction) bool {
	select {
	case <-r.done:
		return false
	case r.actions <- action:
		return true
	}
}

// --- 内部处理 ---

func (r *Room) handleJoin(client *Client) {
	r.mu.Lock()
	r.clients[client.ID()] = client
	count := len(r.clients)
	r.mu.Unlock()

	client.SetState(StateActive)
	log.Printf("✅ [%s] 加入房间 %q (在线: %d)", client.Name(), r.name, count)

	// 广播加入通知
	r.directBroadcast(protocol.NewSystemMessage(
		fmt.Sprintf("%s 加入了聊天室", client.Name()),
	), client)

	// 发送在线列表
	users := r.UserNames()
	client.Send(protocol.NewUsersMessage(users))
}

func (r *Room) handleLeave(client *Client) {
	r.mu.Lock()
	if _, ok := r.clients[client.ID()]; !ok {
		r.mu.Unlock()
		client.Close()
		return
	}
	delete(r.clients, client.ID())
	count := len(r.clients)
	r.mu.Unlock()

	log.Printf("👋 [%s] 离开房间 %q (在线: %d)", client.Name(), r.name, count)

	// 广播离开通知
	if client.Name() != "" {
		r.directBroadcast(protocol.NewSystemMessage(
			fmt.Sprintf("%s 离开了聊天室", client.Name()),
		), nil)
	}

	client.Close()
}

func (r *Room) handleBroadcast(msg *protocol.Message) {
	r.directBroadcast(msg, nil)
}

// directBroadcast 直接广播 (不经过 Actor channel)
//
// 策略:
//   - 如果客户端 sendCh 满 → 断开该客户端 (不静默丢消息)
//   - exclude 参数可排除特定客户端 (如发送者)
func (r *Room) directBroadcast(msg *protocol.Message, exclude *Client) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, client := range r.clients {
		if exclude != nil && client.ID() == exclude.ID() {
			continue
		}
		if !client.Send(msg) {
			// 缓冲区满 → 客户端处理太慢，断开
			log.Printf("⚠️ [%s] 发送缓冲区已满，断开连接", client.Name())
			go r.Leave(client)
		}
	}
}

// SendTo 发送消息给指定客户端
func (r *Room) SendTo(targetName string, msg *protocol.Message) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, client := range r.clients {
		if client.Name() == targetName {
			return client.Send(msg)
		}
	}
	return false
}

// --- 查询方法 (线程安全) ---

// Name 返回房间名称
func (r *Room) Name() string { return r.name }

// Count 返回在线人数
func (r *Room) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}

// UserNames 返回在线用户名列表
func (r *Room) UserNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.clients))
	for _, c := range r.clients {
		if c.Name() != "" {
			names = append(names, c.Name())
		}
	}
	return names
}

// FindByName 按用户名查找客户端
func (r *Room) FindByName(name string) (*Client, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, c := range r.clients {
		if c.Name() == name {
			return c, true
		}
	}
	return nil, false
}

// IsNameTaken 检查用户名是否被占用
func (r *Room) IsNameTaken(name string) bool {
	_, found := r.FindByName(name)
	return found
}

// --- 生命周期 ---

// Close 关闭房间
func (r *Room) Close() {
	r.once.Do(func() {
		r.mu.RLock()
		clients := make([]*Client, 0, len(r.clients))
		for _, client := range r.clients {
			clients = append(clients, client)
		}
		r.mu.RUnlock()

		close(r.done)

		for _, client := range clients {
			client.Close()
		}
	})
}

// ============================================================
// RoomManager 实现
// ============================================================

// DefaultRoomManager 默认房间管理器
type DefaultRoomManager struct {
	mu    sync.RWMutex
	rooms map[string]*Room
}

// NewRoomManager 创建房间管理器
func NewRoomManager() *DefaultRoomManager {
	mgr := &DefaultRoomManager{
		rooms: make(map[string]*Room),
	}
	// 创建默认大厅
	mgr.CreateRoom("lobby")
	return mgr
}

func (m *DefaultRoomManager) CreateRoom(name string) (*Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rooms[name]; exists {
		return nil, fmt.Errorf("房间 %q 已存在", name)
	}

	room := NewRoom(name)
	m.rooms[name] = room
	log.Printf("📁 创建房间: %q", name)
	return room, nil
}

func (m *DefaultRoomManager) GetRoom(name string) (*Room, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.rooms[name]
	return r, ok
}

func (m *DefaultRoomManager) RemoveRoom(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if name == "lobby" {
		return fmt.Errorf("不能删除默认大厅")
	}
	room, ok := m.rooms[name]
	if !ok {
		return fmt.Errorf("房间 %q 不存在", name)
	}
	room.Close()
	delete(m.rooms, name)
	log.Printf("🗑️ 删除房间: %q", name)
	return nil
}

func (m *DefaultRoomManager) ListRooms() []*Room {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rooms := make([]*Room, 0, len(m.rooms))
	for _, r := range m.rooms {
		rooms = append(rooms, r)
	}
	return rooms
}

func (m *DefaultRoomManager) DefaultRoom() *Room {
	r, _ := m.GetRoom("lobby")
	return r
}

func (m *DefaultRoomManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.rooms)
}
