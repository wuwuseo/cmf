package sse

import (
	"context"
	"testing"
	"time"
)

func TestTicketStore_IssueAndConsume(t *testing.T) {
	s := NewTicketStore(time.Minute)
	ticket, exp, err := s.Issue("user-1")
	if err != nil {
		t.Fatalf("颁发失败 %v", err)
	}
	if ticket == "" {
		t.Fatal("ticket 不应为空")
	}
	if exp <= time.Now().Unix() {
		t.Fatalf("过期时间不正确 %d", exp)
	}
	uid, err := s.Consume(ticket)
	if err != nil {
		t.Fatalf("消费失败 %v", err)
	}
	if uid != "user-1" {
		t.Fatalf("uid 不匹配 %s", uid)
	}
}

func TestTicketStore_ConsumeOnce(t *testing.T) {
	s := NewTicketStore(time.Minute)
	ticket, _, _ := s.Issue("user-1")
	if _, err := s.Consume(ticket); err != nil {
		t.Fatalf("首次消费应成功 %v", err)
	}
	if _, err := s.Consume(ticket); err == nil {
		t.Fatal("二次消费应失败")
	}
}

func TestTicketStore_ConsumeExpired(t *testing.T) {
	s := NewTicketStore(10 * time.Millisecond)
	ticket, _, _ := s.Issue("user-1")
	time.Sleep(50 * time.Millisecond)
	if _, err := s.Consume(ticket); err == nil {
		t.Fatal("过期 ticket 消费应失败")
	}
}

func TestTicketStore_ConsumeInvalid(t *testing.T) {
	s := NewTicketStore(time.Minute)
	if _, err := s.Consume("not-exist"); err == nil {
		t.Fatal("不存在的 ticket 消费应失败")
	}
}

func TestTicketStore_IssueEmptyUID(t *testing.T) {
	s := NewTicketStore(time.Minute)
	if _, _, err := s.Issue(""); err == nil {
		t.Fatal("空 uid 颁发应失败")
	}
}

func TestTicketStore_Cleanup(t *testing.T) {
	s := NewTicketStore(10 * time.Millisecond)
	_, _, _ = s.Issue("u1")
	_, _, _ = s.Issue("u2")
	if s.Size() != 2 {
		t.Fatalf("初始大小应为 2，实际 %d", s.Size())
	}
	time.Sleep(50 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	go s.StartCleanup(ctx)
	defer cancel()
	s.m.Range(func(k, _ any) bool {
		_, _ = s.Consume(k.(string))
		return true
	})
	if s.Size() != 0 {
		t.Fatalf("过期 ticket 应被清理，剩余 %d", s.Size())
	}
}
