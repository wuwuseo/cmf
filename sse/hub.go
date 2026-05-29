package sse

import (
	"sync"
	"time"

	"github.com/wuwuseo/cmf/log"
	"go.uber.org/zap"
)

// Client 表示一个 SSE 连接
type Client struct {
	UserUID string
	ConnID  string
	Send    chan Event
	Done    chan struct{}

	closeOnce sync.Once
}

// Close 安全关闭连接，仅会被执行一次
func (c *Client) Close() {
	c.closeOnce.Do(func() { close(c.Done) })
}

// Hub SSE 连接管理器
type Hub struct {
	userClients      sync.Map // userUID -> *sync.Map(connID -> *Client)
	broadcastClients sync.Map // connID -> *Client
	logger           log.Logger
	writeTimeout     time.Duration
}

// NewHub 创建 Hub
func NewHub(opts ...HubOption) *Hub {
	h := &Hub{writeTimeout: 500 * time.Millisecond}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Register 注册客户端连接
func (h *Hub) Register(c *Client) {
	if c == nil || c.ConnID == "" {
		return
	}
	bucket, _ := h.userClients.LoadOrStore(c.UserUID, &sync.Map{})
	bucket.(*sync.Map).Store(c.ConnID, c)
	h.broadcastClients.Store(c.ConnID, c)
}

// Unregister 注销客户端连接
func (h *Hub) Unregister(c *Client) {
	if c == nil || c.ConnID == "" {
		return
	}
	if bucket, ok := h.userClients.Load(c.UserUID); ok {
		bm := bucket.(*sync.Map)
		bm.Delete(c.ConnID)
		empty := true
		bm.Range(func(_, _ any) bool { empty = false; return false })
		if empty {
			h.userClients.Delete(c.UserUID)
		}
	}
	h.broadcastClients.Delete(c.ConnID)
	c.Close()
}

// PushToUser 推送事件给指定用户的所有在线连接
func (h *Hub) PushToUser(userUID string, evt Event) {
	if userUID == "" {
		return
	}
	bucket, ok := h.userClients.Load(userUID)
	if !ok {
		return
	}
	bucket.(*sync.Map).Range(func(_, v any) bool {
		h.deliver(v.(*Client), evt)
		return true
	})
}

// Broadcast 向所有在线连接广播事件
func (h *Hub) Broadcast(evt Event) {
	h.broadcastClients.Range(func(_, v any) bool {
		h.deliver(v.(*Client), evt)
		return true
	})
}

// deliver 单连接投递，带写入超时防慢客户端阻塞
func (h *Hub) deliver(c *Client, evt Event) {
	select {
	case <-c.Done:
		return
	default:
	}
	select {
	case c.Send <- evt:
	case <-c.Done:
	case <-time.After(h.writeTimeout):
		if h.logger != nil {
			h.logger.Warn("SSE 推送超时被丢弃",
				zap.String("user_uid", c.UserUID),
				zap.String("conn_id", c.ConnID),
				zap.String("event", evt.Event),
			)
		}
	}
}

// CountUserConnections 返回某用户当前在线连接数
func (h *Hub) CountUserConnections(userUID string) int {
	bucket, ok := h.userClients.Load(userUID)
	if !ok {
		return 0
	}
	count := 0
	bucket.(*sync.Map).Range(func(_, _ any) bool { count++; return true })
	return count
}

// CountTotalConnections 返回当前总在线连接数
func (h *Hub) CountTotalConnections() int {
	count := 0
	h.broadcastClients.Range(func(_, _ any) bool { count++; return true })
	return count
}

// Shutdown 优雅关闭所有连接
func (h *Hub) Shutdown() {
	h.broadcastClients.Range(func(_, v any) bool { v.(*Client).Close(); return true })
}
