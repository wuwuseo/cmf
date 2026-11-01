package ws

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

// localsAddressesKey 升级前后传递绑定地址的 Locals 键（contrib 会复制请求 Locals）
const localsAddressesKey = "ws.addresses"

// 单帧最大读取字节数，防止恶意大包
const readLimitBytes = 4096

// NewConnectHandler 返回 WS 升级端点：先校验一次性 ticket，再升级。
// ticket 无效 → 401 不升级；某地址连接数超限 → 429。
func NewConnectHandler(hub *Hub) fiber.Handler {
	if hub == nil {
		return func(c fiber.Ctx) error {
			return c.Status(fiber.StatusInternalServerError).SendString("ws hub 未初始化")
		}
	}
	upgrade := websocket.New(func(conn *websocket.Conn) {
		handleConn(hub, conn)
	}, websocket.Config{
		HandshakeTimeout: 5 * time.Second,
		ReadBufferSize:   readLimitBytes,
		WriteBufferSize:  readLimitBytes,
	})

	return func(c fiber.Ctx) error {
		if !websocket.IsWebSocketUpgrade(c) {
			return fiber.ErrUpgradeRequired
		}
		var addresses []string
		if hub.cfg.Tickets != nil {
			var err error
			addresses, err = hub.cfg.Tickets.Consume(c.Query("ticket"))
			if err != nil {
				return c.Status(fiber.StatusUnauthorized).SendString("ticket 无效或已过期")
			}
		}
		if len(addresses) == 0 {
			return c.Status(fiber.StatusUnauthorized).SendString("ticket 无效或已过期")
		}
		// 只对身份地址（ticket 首个地址）做连接数限制：功能集可以把共享房间（如抢单大厅）
		// 绑定为附加地址，这类地址的成员数随在线用户增长，不能按「单用户多设备」的口径限流。
		if hub.Count(addresses[0]) >= hub.cfg.MaxConnsPerAddress {
			return c.Status(fiber.StatusTooManyRequests).SendString("连接数超过上限")
		}
		c.Locals(localsAddressesKey, addresses)
		return upgrade(c)
	}
}

// handleConn 单连接生命周期：注册房间 → ready 帧 → 写协程 → 读循环（ping / 读超时）
func handleConn(hub *Hub, conn *websocket.Conn) {
	addresses, _ := conn.Locals(localsAddressesKey).([]string)
	if len(addresses) == 0 {
		_ = conn.Close()
		return
	}
	cl := &client{
		conn: conn,
		send: make(chan []byte, clientSendBuffer),
		done: make(chan struct{}),
	}
	hub.join(cl, addresses)

	unread := 0
	if hub.cfg.UnreadCounter != nil {
		if n, err := hub.cfg.UnreadCounter(context.Background(), addresses); err == nil {
			unread = n
		} else if hub.cfg.Logger != nil {
			hub.cfg.Logger.Warn("WS 未读数统计失败", zap.Error(err))
		}
	}
	cl.enqueue(Event{
		Type: TypeReady,
		Data: map[string]any{"addresses": addresses, "unread": unread},
	}.Bytes())
	go cl.writeLoop(hub.cfg.WriteTimeout)

	conn.SetReadLimit(readLimitBytes)
	_ = conn.SetReadDeadline(time.Now().Add(hub.cfg.ReadTimeout))
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			break
		}
		_ = conn.SetReadDeadline(time.Now().Add(hub.cfg.ReadTimeout))
		var frame struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(payload, &frame) == nil && frame.Type == TypePing {
			cl.enqueue(Event{Type: TypePong}.Bytes())
		}
	}

	hub.leave(cl)
	cl.close()
}
