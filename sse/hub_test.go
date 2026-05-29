package sse

import (
	"sync"
	"testing"
	"time"
)

func newTestClient(uid, connID string) *Client {
	return &Client{
		UserUID: uid,
		ConnID:  connID,
		Send:    make(chan Event, 64),
		Done:    make(chan struct{}),
	}
}

func TestHub_RegisterAndUnregister(t *testing.T) {
	hub := NewHub()
	c := newTestClient("u1", "conn-1")
	hub.Register(c)
	if got := hub.CountUserConnections("u1"); got != 1 {
		t.Fatalf("注册后用户连接数应为 1，实际 %d", got)
	}
	if got := hub.CountTotalConnections(); got != 1 {
		t.Fatalf("注册后总连接数应为 1，实际 %d", got)
	}
	hub.Unregister(c)
	if got := hub.CountUserConnections("u1"); got != 0 {
		t.Fatalf("注销后用户连接数应为 0，实际 %d", got)
	}
	if got := hub.CountTotalConnections(); got != 0 {
		t.Fatalf("注销后总连接数应为 0，实际 %d", got)
	}
}

func TestHub_PushToOnlineUser(t *testing.T) {
	hub := NewHub()
	c := newTestClient("u1", "conn-1")
	hub.Register(c)
	defer hub.Unregister(c)
	hub.PushToUser("u1", Event{ID: "1", Event: "message", Data: "hi"})
	select {
	case got := <-c.Send:
		if got.ID != "1" || got.Event != "message" {
			t.Fatalf("收到事件不匹配 %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("超时未收到事件")
	}
}

func TestHub_PushToOfflineUser(t *testing.T) {
	hub := NewHub()
	hub.PushToUser("not-exist", Event{ID: "1", Event: "message", Data: nil})
}

func TestHub_BroadcastToAll(t *testing.T) {
	hub := NewHub()
	c1 := newTestClient("u1", "conn-1")
	c2 := newTestClient("u2", "conn-2")
	hub.Register(c1)
	hub.Register(c2)
	defer hub.Unregister(c1)
	defer hub.Unregister(c2)
	hub.Broadcast(Event{ID: "10", Event: "message", Data: "all"})
	for _, c := range []*Client{c1, c2} {
		select {
		case got := <-c.Send:
			if got.ID != "10" {
				t.Fatalf("广播事件 ID 不匹配 %s", got.ID)
			}
		case <-time.After(time.Second):
			t.Fatalf("用户 %s 超时未收到广播", c.UserUID)
		}
	}
}

func TestHub_MultiConnectionsPerUser(t *testing.T) {
	hub := NewHub()
	c1 := newTestClient("u1", "conn-1")
	c2 := newTestClient("u1", "conn-2")
	hub.Register(c1)
	hub.Register(c2)
	defer hub.Unregister(c1)
	defer hub.Unregister(c2)
	if got := hub.CountUserConnections("u1"); got != 2 {
		t.Fatalf("同一用户应有 2 个连接，实际 %d", got)
	}
	hub.PushToUser("u1", Event{ID: "1", Event: "message", Data: "x"})
	var wg sync.WaitGroup
	wg.Add(2)
	for _, c := range []*Client{c1, c2} {
		go func(cl *Client) {
			defer wg.Done()
			select {
			case <-cl.Send:
			case <-time.After(time.Second):
				t.Errorf("连接 %s 未收到推送", cl.ConnID)
			}
		}(c)
	}
	wg.Wait()
}

func TestHub_DeliverDoesNotBlockOnFullBuffer(t *testing.T) {
	hub := NewHub(WithWriteTimeout(50 * time.Millisecond))
	c := &Client{
		UserUID: "u1", ConnID: "conn-1",
		Send: make(chan Event, 1),
		Done: make(chan struct{}),
	}
	hub.Register(c)
	defer hub.Unregister(c)
	hub.PushToUser("u1", Event{ID: "1"})
	start := time.Now()
	hub.PushToUser("u1", Event{ID: "2"})
	elapsed := time.Since(start)
	if elapsed > 200*time.Millisecond {
		t.Fatalf("缓冲满时不应长时间阻塞，实际耗时 %v", elapsed)
	}
}

func TestHub_PushAfterClientClosed(t *testing.T) {
	hub := NewHub()
	c := newTestClient("u1", "conn-1")
	hub.Register(c)
	hub.Unregister(c)
	hub.PushToUser("u1", Event{ID: "9"})
}

func TestHub_Shutdown(t *testing.T) {
	hub := NewHub()
	c := newTestClient("u1", "conn-1")
	hub.Register(c)
	hub.Shutdown()
	select {
	case <-c.Done:
	case <-time.After(time.Second):
		t.Fatal("Shutdown 后 Client.Done 未关闭")
	}
}
