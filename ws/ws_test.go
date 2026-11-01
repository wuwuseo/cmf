package ws

import (
	"testing"
	"time"
)

// newTestClient 构造一条不带真实连接的 client（conn 为 nil，close 有 nil 保护）
func newTestClient() *client {
	return &client{
		send: make(chan []byte, clientSendBuffer),
		done: make(chan struct{}),
	}
}

func TestSendReachesAllConnectionsOfAddress(t *testing.T) {
	hub := NewHub(Config{})
	a, b := newTestClient(), newTestClient()
	hub.join(a, []string{"u1"})
	hub.join(b, []string{"u1", "customer:1"})

	hub.Send("u1", Event{Type: TypeMessageCreated})
	if got := hub.Count("u1"); got != 2 {
		t.Fatalf("同一地址连接数 = %d, 期望 2", got)
	}
	for _, c := range []*client{a, b} {
		select {
		case <-c.send:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("同地址的连接未全部收到推送")
		}
	}

	// Broadcast 走同一房间语义
	hub.Broadcast("u1", Event{Type: TypePong})
	for _, c := range []*client{a, b} {
		select {
		case <-c.send:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("同房间广播未送达全部连接")
		}
	}
}

func TestTicketSingleUseAndExpiry(t *testing.T) {
	store := NewTicketStore(0)
	ticket, exp, err := store.Issue([]string{"u1", "u1", "", "customer:1"})
	if err != nil {
		t.Fatalf("签发 ticket 失败: %v", err)
	}
	if exp <= time.Now().Unix() {
		t.Fatalf("ticket 过期时间应晚于当前时间: %d", exp)
	}
	addrs, err := store.Consume(ticket)
	if err != nil {
		t.Fatalf("首次消费失败: %v", err)
	}
	if len(addrs) != 2 || addrs[0] != "u1" || addrs[1] != "customer:1" {
		t.Fatalf("地址未去重去空: %+v", addrs)
	}
	if _, err = store.Consume(ticket); err != ErrTicketInvalid {
		t.Fatalf("二次消费应失败, got %v", err)
	}

	short := NewTicketStore(20 * time.Millisecond)
	expired, _, err := short.Issue([]string{"u2"})
	if err != nil {
		t.Fatalf("签发 ticket 失败: %v", err)
	}
	time.Sleep(40 * time.Millisecond)
	if _, err = short.Consume(expired); err != ErrTicketInvalid {
		t.Fatalf("过期 ticket 应失败, got %v", err)
	}

	if _, _, err = store.Issue(nil); err != ErrTicketNoAddress {
		t.Fatalf("无地址时应拒绝签发, got %v", err)
	}
}

func TestSlowConnectionDropped(t *testing.T) {
	hub := NewHub(Config{})
	slow, fast := newTestClient(), newTestClient()
	hub.join(slow, []string{"u1"})
	hub.join(fast, []string{"u1"})

	// 塞满慢连接的写队列，模拟客户端卡顿
	for i := 0; i < clientSendBuffer; i++ {
		slow.send <- []byte("x")
	}
	hub.Send("u1", Event{Type: TypePong})

	select {
	case <-slow.done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("写队列已满的连接应被关闭")
	}
	select {
	case <-fast.send:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("同房间的正常连接不应受影响")
	}
}
