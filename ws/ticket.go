package ws

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

var (
	// ErrTicketInvalid ticket 不存在、已过期或已被使用
	ErrTicketInvalid = errors.New("ticket 不存在或已过期")
	// ErrTicketNoAddress ticket 必须至少绑定一个地址
	ErrTicketNoAddress = errors.New("ticket 必须至少绑定一个地址")
)

// 默认 ticket 有效期
const defaultTicketTTL = 2 * time.Minute

// TicketStore 一次性临时凭证存储（进程内），payload 为地址数组。
// 不复用 sse.TicketStore：后者只支持单个 uid，且管理端 SSE 正在使用。
type TicketStore struct {
	m   sync.Map
	ttl time.Duration
}

type ticketEntry struct {
	addresses []string
	expireAt  time.Time
}

// NewTicketStore 创建 ticket 存储
func NewTicketStore(ttl time.Duration) *TicketStore {
	if ttl <= 0 {
		ttl = defaultTicketTTL
	}
	return &TicketStore{ttl: ttl}
}

// Issue 颁发一个绑定多个地址的 ticket，返回 ticket 与过期时间戳（秒）
func (s *TicketStore) Issue(addresses []string) (string, int64, error) {
	list := normalizeAddresses(addresses)
	if len(list) == 0 {
		return "", 0, ErrTicketNoAddress
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", 0, err
	}
	ticket := base64.RawURLEncoding.EncodeToString(buf)
	exp := time.Now().Add(s.ttl)
	s.m.Store(ticket, &ticketEntry{addresses: list, expireAt: exp})
	return ticket, exp.Unix(), nil
}

// Consume 一次性消费 ticket；LoadAndDelete 保证并发下也只有一次成功
func (s *TicketStore) Consume(ticket string) ([]string, error) {
	if ticket == "" {
		return nil, ErrTicketInvalid
	}
	v, ok := s.m.LoadAndDelete(ticket)
	if !ok {
		return nil, ErrTicketInvalid
	}
	entry, ok := v.(*ticketEntry)
	if !ok {
		return nil, ErrTicketInvalid
	}
	if time.Now().After(entry.expireAt) {
		return nil, ErrTicketInvalid
	}
	return entry.addresses, nil
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
				if entry, ok := v.(*ticketEntry); ok && now.After(entry.expireAt) {
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

// normalizeAddresses 去空、去重
func normalizeAddresses(addresses []string) []string {
	seen := make(map[string]struct{}, len(addresses))
	out := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		if addr == "" {
			continue
		}
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		out = append(out, addr)
	}
	return out
}
