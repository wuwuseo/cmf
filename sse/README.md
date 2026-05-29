# cmf/sse — 通用 Server-Sent Events 库

提供 Event 协议、Hub 连接管理、一次性 Ticket、Fiber v3 流式响应封装，可被任何业务系统集成。

## 安装

```bash
go get github.com/wuwuseo/cmf/sse
```

## 核心组件

| 组件 | 说明 |
|------|------|
| `Event` | SSE 协议事件结构（ID / Event / Data），实现 `WriteTo(io.Writer)` |
| `Client` | 单个 SSE 连接表示，含缓冲 `Send` channel 与 `Done` 关闭信号 |
| `Hub` | 连接管理器，支持按 userUID 推送、全员广播、慢客户端写超时丢弃 |
| `TicketStore` | 一次性凭证存储，进程内 sync.Map + TTL，支持后台清理协程 |
| `HandleConnect` | Fiber v3 流式响应封装：消费 ticket → 注册客户端 → 补发 → 心跳循环 |

## 公共 API 概览

```go
// Event
type Event struct{ ID, Event string; Data any }
func (e *Event) WriteTo(w io.Writer) (int64, error)

// Hub
hub := sse.NewHub(sse.WithLogger(logger), sse.WithWriteTimeout(500*time.Millisecond))
hub.Register(client); hub.Unregister(client)
hub.PushToUser(uid, evt); hub.Broadcast(evt)
hub.Shutdown()

// TicketStore
store := sse.NewTicketStore(5 * time.Minute)
ticket, expireAt, err := store.Issue(uid)
uid, err := store.Consume(ticket)
go store.StartCleanup(ctx)

// Handler
return sse.HandleConnect(c, sse.StreamConfig{
    Hub:               hub,
    TicketStore:       store,
    HeartbeatInterval: 25 * time.Second,
    MissedFetcher:     fetcher, // 可选
})
```

## 快速集成（Fiber v3）

```go
hub := sse.NewHub(sse.WithLogger(logger))
store := sse.NewTicketStore(5 * time.Minute)
go store.StartCleanup(ctx)

// 鉴权路由：颁发 ticket
app.Post("/sse/ticket", func(c fiber.Ctx) error {
    uid := getUIDFromJWT(c)
    t, exp, _ := sse.IssueTicket(c, store, uid)
    return c.JSON(fiber.Map{"ticket": t, "expire_at": exp})
})

// 开放路由：建立 SSE 长连接
app.Get("/sse/connect", func(c fiber.Ctx) error {
    return sse.HandleConnect(c, sse.StreamConfig{
        Hub:           hub,
        TicketStore:   store,
        MissedFetcher: myFetcher, // 可选：断线补发
    })
})

// 业务侧推送
hub.PushToUser("user-1", sse.Event{ID: "42", Event: "message", Data: payload})
hub.Broadcast(sse.Event{Event: "system", Data: "维护通知"})
```

## MissedFetcher 实现示例

```go
fetcher := func(ctx context.Context, uid string, lastID int) ([]sse.Event, error) {
    rows, err := db.Query(ctx, "SELECT id, type, payload FROM messages WHERE user_uid=? AND id>?", uid, lastID)
    if err != nil { return nil, err }
    var out []sse.Event
    for rows.Next() {
        var id int; var typ, payload string
        rows.Scan(&id, &typ, &payload)
        out = append(out, sse.Event{ID: strconv.Itoa(id), Event: typ, Data: payload})
    }
    return out, nil
}
```

## Nginx 部署

```nginx
location /sse/ {
    proxy_pass http://backend;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 1h;
    chunked_transfer_encoding on;
}
```

## 设计要点

1. **先落库后推送**：业务方应先持久化消息拿到自增 ID，再以该 ID 作为 SSE 事件 ID 推送，便于 `Last-Event-ID` 断线补发。
2. **慢客户端保护**：`Hub.deliver` 使用 `select + writeTimeout(500ms)`，超时丢弃事件并记录日志，避免阻塞推送。
3. **心跳**：默认 25 秒一次 `event: heartbeat`，防止 Nginx 60s 默认超时断连。
4. **优雅关闭**：`Hub.Shutdown()` 关闭所有连接的 `Done` channel，主循环自动 return。
5. **多实例支持**：当前为单实例 Hub；多实例部署时可在业务层接入 Redis Pub/Sub 跨实例转发。
6. **鉴权独立**：EventSource 无法携带 Header，使用一次性 ticket（5 分钟过期）独立鉴权。
