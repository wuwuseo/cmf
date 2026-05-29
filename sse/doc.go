// Package sse 提供 Server-Sent Events 通用能力：事件协议、连接管理（Hub）、
// 一次性凭证（TicketStore）以及 Fiber v3 流式响应封装。
//
// 典型用法：
//
//	hub := sse.NewHub(sse.WithLogger(logger))
//	store := sse.NewTicketStore(5 * time.Minute)
//	// 鉴权后颁发 ticket
//	ticket, exp, _ := sse.IssueTicket(c, store, currentUserUID)
//	// 客户端 EventSource 连接到 /sse/connect?ticket=xxx
//	return sse.HandleConnect(c, sse.StreamConfig{Hub: hub, TicketStore: store})
//
// 业务方通过 hub.PushToUser / hub.Broadcast 推送事件，事件结构由业务方填充：
//
//	hub.PushToUser(uid, sse.Event{ID: "123", Event: "message", Data: payload})
package sse
