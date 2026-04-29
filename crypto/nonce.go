package crypto

import (
	"sync"
	"time"
)

// NonceStore nonce存储和验证
type NonceStore struct {
	store map[string]time.Time
	mu    sync.RWMutex
	ttl   time.Duration
}

var (
	nonceStore *NonceStore
	nonceOnce  sync.Once
)

// InitNonceStore 初始化nonce存储
func InitNonceStore(ttl ...time.Duration) *NonceStore {
	nonceOnce.Do(func() {
		ttlDuration := 5 * time.Minute // 默认5分钟过期
		if len(ttl) > 0 {
			ttlDuration = ttl[0]
		}
		nonceStore = &NonceStore{
			store: make(map[string]time.Time),
			ttl:   ttlDuration,
		}
		// 启动清理过期nonce的定时任务
		go nonceStore.startCleanup()
	})
	return nonceStore
}

// Add 添加nonce，如果已存在返回false
func (ns *NonceStore) Add(nonce string) bool {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// 检查nonce是否已存在
	if _, exists := ns.store[nonce]; exists {
		return false
	}

	// 添加新的nonce
	ns.store[nonce] = time.Now().Add(ns.ttl)
	return true
}

// Verify 验证nonce是否有效
func (ns *NonceStore) Verify(nonce string) bool {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	expireTime, exists := ns.store[nonce]
	if !exists {
		return false
	}

	// 检查是否已过期
	if time.Now().After(expireTime) {
		return false
	}

	return true
}

// ValidateTimestamp 验证时间戳是否在有效窗口内
func ValidateTimestamp(timestamp int64, window time.Duration) bool {
	now := time.Now().Unix()
	diff := now - timestamp

	// 检查时间戳是否在有效窗口内（允许前后window时间的偏差）
	return diff >= 0 && diff <= int64(window.Seconds())
}

// startCleanup 定期清理过期的nonce
func (ns *NonceStore) startCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ns.cleanup()
	}
}

// cleanup 清理过期的nonce
func (ns *NonceStore) cleanup() {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	now := time.Now()
	for nonce, expireTime := range ns.store {
		if now.After(expireTime) {
			delete(ns.store, nonce)
		}
	}
}

// GetNonceStore 获取nonce存储实例
func GetNonceStore() *NonceStore {
	if nonceStore == nil {
		InitNonceStore()
	}
	return nonceStore
}
