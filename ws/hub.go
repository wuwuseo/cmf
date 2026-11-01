package ws

import (
	"context"
	"sync"
	"time"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/wuwuseo/cmf/log"
)

// 默认参数
const (
	defaultWriteTimeout       = 10 * time.Second
	defaultReadTimeout        = 60 * time.Second // 客户端每 25s 发一次 ping，两个周期未收到即断开
	defaultMaxConnsPerAddress = 5
	clientSendBuffer          = 32 // 每连接写队列容量：写满即断开该连接
)

// Config WebSocket 通道配置；框架层不碰数据库，未读统计由应用层注入
type Config struct {
	Logger             log.Logger
	WriteTimeout       time.Duration
	ReadTimeout        time.Duration
	MaxConnsPerAddress int
	Tickets            *TicketStore
	// UnreadCounter 建连 ready 帧的未读数；为 nil 时按 0 下发
	UnreadCounter func(ctx context.Context, addresses []string) (int, error)
}

// Hub 进程内地址房间管理器。address 对框架层是不透明字符串，一条连接可绑定多个。
// ponytail: 单实例，不支持水平扩展；多实例需换 centrifugo 或加 Redis pub/sub，只替换本实现
type Hub struct {
	cfg   Config
	mu    sync.RWMutex
	rooms map[string]map[*client]struct{}
}

// client 单条连接：读协程由 handler 持有，写协程独占所有下行帧
type client struct {
	conn *websocket.Conn
	send chan []byte
	done chan struct{}
	once sync.Once
}

// NewHub 创建 Hub 并填充默认参数
func NewHub(cfg Config) *Hub {
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = defaultWriteTimeout
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = defaultReadTimeout
	}
	if cfg.MaxConnsPerAddress <= 0 {
		cfg.MaxConnsPerAddress = defaultMaxConnsPerAddress
	}
	return &Hub{cfg: cfg, rooms: make(map[string]map[*client]struct{})}
}

// join 把连接加入全部地址房间
func (h *Hub) join(c *client, addresses []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, addr := range addresses {
		if addr == "" {
			continue
		}
		room := h.rooms[addr]
		if room == nil {
			room = make(map[*client]struct{})
			h.rooms[addr] = room
		}
		room[c] = struct{}{}
	}
}

// leave 把连接从所有房间移除，空房间删除避免 map 泄漏
func (h *Hub) leave(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for addr, room := range h.rooms {
		delete(room, c)
		if len(room) == 0 {
			delete(h.rooms, addr)
		}
	}
}

// Send 推送给某个地址上的全部连接
func (h *Hub) Send(address string, ev Event) {
	h.sendBytes(address, ev.Bytes())
}

// Broadcast 向房间广播；房间就是地址（多条连接共享同一地址即群发语义）
func (h *Hub) Broadcast(room string, ev Event) {
	h.Send(room, ev)
}

// sendBytes 以已序列化的字节推送，避免同一事件重复序列化
func (h *Hub) sendBytes(address string, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.rooms[address] {
		c.enqueue(payload)
	}
}

// Count 某地址当前连接数（升级前用于限流）
func (h *Hub) Count(address string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[address])
}

// Shutdown 关闭全部连接
func (h *Hub) Shutdown() {
	h.mu.Lock()
	rooms := h.rooms
	h.rooms = make(map[string]map[*client]struct{})
	h.mu.Unlock()
	for _, room := range rooms {
		for c := range room {
			c.close()
		}
	}
}

// enqueue 非阻塞入队；队列满即断开本连接，同房间其它连接不受影响
func (c *client) enqueue(payload []byte) {
	select {
	case <-c.done:
		return
	default:
	}
	select {
	case c.send <- payload:
	case <-c.done:
	default:
		c.close()
	}
}

// writeLoop 串行写；连接不可写时关闭自身
func (c *client) writeLoop(writeTimeout time.Duration) {
	for {
		select {
		case payload := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				c.close()
				return
			}
		case <-c.done:
			return
		}
	}
}

// close 幂等关闭；done 通道不关闭 send，避免并发发送时 panic
func (c *client) close() {
	c.once.Do(func() {
		close(c.done)
		if c.conn != nil {
			_ = c.conn.Close()
		}
	})
}
