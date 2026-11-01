package ws

import "encoding/json"

// 帧类型常量（与服务端 → 客户端契约一致）
const (
	TypeReady          = "ready"
	TypeMessageCreated = "message.created"
	TypePing           = "ping"
	TypePong           = "pong"
)

// Event 服务端下行帧；只送信封，正文由客户端补拉
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

// Bytes 序列化事件；失败时退化为 error 帧，避免整条连接因单个事件卡死
func (e Event) Bytes() []byte {
	b, err := json.Marshal(e)
	if err != nil {
		return []byte(`{"type":"error"}`)
	}
	return b
}
