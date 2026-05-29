package sse

import (
	"bufio"
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// MissedFetcher 业务方实现：根据 lastEventID 返回需要补发的事件列表
// 返回的 Event 数组将按顺序写出。若返回 error，将忽略本次补发但保持连接。
type MissedFetcher func(ctx context.Context, userUID string, lastEventID int) ([]Event, error)

// StreamConfig SSE 流式连接配置
type StreamConfig struct {
	// Hub 连接管理器（必填）
	Hub *Hub
	// TicketStore ticket 存储（必填）
	TicketStore *TicketStore
	// HeartbeatInterval 心跳间隔，默认 25s
	HeartbeatInterval time.Duration
	// SendBufferSize 单个客户端 Send channel 缓冲大小，默认 64
	SendBufferSize int
	// MissedFetcher 可选：客户端携带 lastEventId 时用于补发的回调
	MissedFetcher MissedFetcher
	// MissedLimit 单次补发上限，默认 200
	MissedLimit int
}

const (
	defaultHeartbeatInterval = 25 * time.Second
	defaultSendBufferSize    = 64
	defaultMissedLimit       = 200
)

// ErrInvalidTicket ticket 缺失或无效
var ErrInvalidTicket = errors.New("invalid ticket")

// IssueTicket 在已鉴权的 handler 中颁发 ticket
// uid 由调用方从自己的 JWT/Session 中提取，不为空
func IssueTicket(_ fiber.Ctx, store *TicketStore, uid string) (string, int64, error) {
	if store == nil {
		return "", 0, errors.New("ticket store 未初始化")
	}
	return store.Issue(uid)
}

// HandleConnect 处理 SSE 连接：消费 ticket -> 注册 client -> 补发 -> 主循环
//
// 业务方在 fiber handler 中：
//
//	func SSEConnect(c fiber.Ctx) error {
//	    return sse.HandleConnect(c, sse.StreamConfig{Hub: hub, TicketStore: store, MissedFetcher: fetcher})
//	}
func HandleConnect(c fiber.Ctx, cfg StreamConfig) error {
	if cfg.Hub == nil || cfg.TicketStore == nil {
		return c.Status(fiber.StatusInternalServerError).SendString("sse not configured")
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = defaultHeartbeatInterval
	}
	if cfg.SendBufferSize <= 0 {
		cfg.SendBufferSize = defaultSendBufferSize
	}
	if cfg.MissedLimit <= 0 {
		cfg.MissedLimit = defaultMissedLimit
	}

	ticket := c.Query("ticket")
	if ticket == "" {
		return c.Status(fiber.StatusUnauthorized).SendString("missing ticket")
	}
	uid, err := cfg.TicketStore.Consume(ticket)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).SendString(err.Error())
	}

	lastEventID := 0
	if v := c.Query("lastEventId"); v != "" {
		if n, e := strconv.Atoi(v); e == nil {
			lastEventID = n
		}
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	client := &Client{
		UserUID: uid,
		ConnID:  uuid.NewString(),
		Send:    make(chan Event, cfg.SendBufferSize),
		Done:    make(chan struct{}),
	}

	ctx := c.Context()
	return c.SendStreamWriter(func(w *bufio.Writer) {
		cfg.Hub.Register(client)
		defer cfg.Hub.Unregister(client)

		// 发送 ready 事件
		ready := Event{Event: "ready", Data: map[string]any{"conn_id": client.ConnID}}
		if _, err := ready.WriteTo(w); err != nil {
			return
		}
		if err := w.Flush(); err != nil {
			return
		}

		// 补发
		if lastEventID > 0 && cfg.MissedFetcher != nil {
			events, fErr := cfg.MissedFetcher(ctx, uid, lastEventID)
			if fErr == nil {
				for _, e := range events {
					if _, err := e.WriteTo(w); err != nil {
						return
					}
				}
				if err := w.Flush(); err != nil {
					return
				}
			}
		}

		ticker := time.NewTicker(cfg.HeartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-client.Done:
				return
			case <-ctx.Done():
				return
			case evt := <-client.Send:
				if _, err := evt.WriteTo(w); err != nil {
					return
				}
				if err := w.Flush(); err != nil {
					return
				}
			case <-ticker.C:
				hb := Event{Event: "heartbeat", Data: map[string]any{"ts": time.Now().Unix()}}
				if _, err := hb.WriteTo(w); err != nil {
					return
				}
				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	})
}
