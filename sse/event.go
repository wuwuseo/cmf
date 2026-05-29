package sse

import (
	"encoding/json"
	"fmt"
	"io"
)

// Event 表示一条 SSE 事件
type Event struct {
	// ID 事件 ID，用于客户端断线重连时 Last-Event-ID 补发
	ID string `json:"-"`
	// Event 事件名：message / heartbeat / ready / error
	Event string `json:"-"`
	// Data 数据载荷，将以 JSON 形式序列化
	Data any `json:"data"`
}

// WriteTo 按 SSE 协议将事件写入 w
func (e *Event) WriteTo(w io.Writer) (int64, error) {
	var total int64
	if e.ID != "" {
		n, err := fmt.Fprintf(w, "id: %s\n", e.ID)
		total += int64(n)
		if err != nil {
			return total, err
		}
	}
	if e.Event != "" {
		n, err := fmt.Fprintf(w, "event: %s\n", e.Event)
		total += int64(n)
		if err != nil {
			return total, err
		}
	}
	payload, err := json.Marshal(e.Data)
	if err != nil {
		return total, err
	}
	n, err := fmt.Fprintf(w, "data: %s\n\n", payload)
	total += int64(n)
	return total, err
}
