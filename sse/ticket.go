package sse

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

// ErrTicketInvalid ticket 不存在或已过期/已使用
var ErrTicketInvalid = errors.New("ticket invalid or expired")

// TicketStore 临时凭证存储（进程内 + 一次性消费）
type TicketStore struct {
	m   sync.Map
	ttl time.Duration
}

type ticketEntry struct {
	uid       string
	expireAt  time.Time
	consumed  bool
	consumeMu sync.Mutex
}

// NewTicketStore 创建 ticket 存储
func NewTicketStore(ttl time.Duration) *TicketStore {
	return &TicketStore{ttl: ttl}
}

// Issue 颁发一个新 ticket，返回 ticket 字符串与过期时间戳（秒）
func (s *TicketStore) Issue(uid string) (string, int64, error) {
	if uid == "" {
		return "", 0, errors.New("uid 不能为空")
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", 0, err
	}
	ticket := base64.RawURLEncoding.EncodeToString(buf)
	exp := time.Now().Add(s.ttl)
	s.m.Store(ticket, &ticketEntry{uid: uid, expireAt: exp})
	return ticket, exp.Unix(), nil
}

// Consume 一次性消费 ticket，返回对应的 uid
func (s *TicketStore) Consume(ticket string) (string, error) {
	v, ok := s.m.Load(ticket)
	if !ok {
		return "", ErrTicketInvalid
	}
	entry := v.(*ticketEntry)
	entry.consumeMu.Lock()
	defer entry.consumeMu.Unlock()
	if entry.consumed {
		return "", ErrTicketInvalid
	}
	if time.Now().After(entry.expireAt) {
		s.m.Delete(ticket)
		return "", ErrTicketInvalid
	}
	entry.consumed = true
	s.m.Delete(ticket)
	return entry.uid, nil
}

// StartCleanup 启动后台清理协程，每分钟清理过期 ticket
func (s *TicketStore) StartCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.m.Range(func(k, v any) bool {
				entry := v.(*ticketEntry)
				if now.After(entry.expireAt) {
					s.m.Delete(k)
				}
				return true
			})
		}
	}
}

// Size 当前 ticket 数量
func (s *TicketStore) Size() int {
	count := 0
	s.m.Range(func(_, _ any) bool { count++; return true })
	return count
}
