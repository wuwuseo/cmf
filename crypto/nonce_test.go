package crypto

import (
	"sync"
	"testing"
	"time"
)

// resetNonceStore 重置 NonceStore 单例状态（用于测试隔离）
func resetNonceStore() {
	nonceStore = nil
	nonceOnce = sync.Once{}
}

// TestInitNonceStore_Singleton 测试 InitNonceStore 单例模式（两次初始化返回同一实例）
func TestInitNonceStore_Singleton(t *testing.T) {
	resetNonceStore()

	ns1 := InitNonceStore()
	ns2 := InitNonceStore()

	if ns1 != ns2 {
		t.Error("两次 InitNonceStore 应返回同一实例（单例模式）")
	}
	if ns1 == nil {
		t.Error("InitNonceStore 不应返回 nil")
	}
}

// TestInitNonceStore_CustomTTL 测试自定义 TTL
func TestInitNonceStore_CustomTTL(t *testing.T) {
	resetNonceStore()

	ns := InitNonceStore(10 * time.Minute)
	if ns == nil {
		t.Fatal("InitNonceStore 不应返回 nil")
	}
	if ns.ttl != 10*time.Minute {
		t.Errorf("TTL 应为 10 分钟，实际为 %v", ns.ttl)
	}
}

// TestAdd_Success 测试添加 nonce 成功
func TestAdd_Success(t *testing.T) {
	resetNonceStore()

	ns := InitNonceStore()
	ok := ns.Add("unique-nonce-1")
	if !ok {
		t.Error("首次添加 nonce 应返回 true")
	}
}

// TestAdd_Duplicate 测试重复 nonce 返回 false
func TestAdd_Duplicate(t *testing.T) {
	resetNonceStore()

	ns := InitNonceStore()
	ns.Add("duplicate-nonce")

	ok := ns.Add("duplicate-nonce")
	if ok {
		t.Error("重复添加相同 nonce 应返回 false")
	}
}

// TestVerify_ValidNonce 测试有效 nonce 返回 true
func TestVerify_ValidNonce(t *testing.T) {
	resetNonceStore()

	ns := InitNonceStore()
	ns.Add("valid-nonce")

	ok := ns.Verify("valid-nonce")
	if !ok {
		t.Error("有效的 nonce 应验证通过")
	}
}

// TestVerify_NonExistentNonce 测试不存在的 nonce 返回 false
func TestVerify_NonExistentNonce(t *testing.T) {
	resetNonceStore()

	ns := InitNonceStore()

	ok := ns.Verify("nonexistent-nonce")
	if ok {
		t.Error("不存在的 nonce 不应验证通过")
	}
}

// TestVerify_ExpiredNonce 测试过期 nonce 返回 false
func TestVerify_ExpiredNonce(t *testing.T) {
	resetNonceStore()

	// 使用极短 TTL
	ns := InitNonceStore(1 * time.Millisecond)
	ns.Add("expired-nonce")

	// 等待 nonce 过期
	time.Sleep(10 * time.Millisecond)

	ok := ns.Verify("expired-nonce")
	if ok {
		t.Error("过期的 nonce 不应验证通过")
	}
}

// TestValidateTimestamp_Valid 测试有效时间戳
func TestValidateTimestamp_Valid(t *testing.T) {
	now := time.Now().Unix()
	ok := ValidateTimestamp(now, 1*time.Minute)
	if !ok {
		t.Error("有效时间戳应验证通过")
	}
}

// TestValidateTimestamp_Expired 测试过期时间戳
func TestValidateTimestamp_Expired(t *testing.T) {
	oldTimestamp := time.Now().Add(-2 * time.Minute).Unix()
	ok := ValidateTimestamp(oldTimestamp, 1*time.Minute)
	if ok {
		t.Error("过期时间戳不应验证通过")
	}
}

// TestValidateTimestamp_Future 测试未来时间戳
func TestValidateTimestamp_Future(t *testing.T) {
	futureTimestamp := time.Now().Add(1 * time.Minute).Unix()
	ok := ValidateTimestamp(futureTimestamp, 1*time.Minute)
	if ok {
		t.Error("未来时间戳不应验证通过")
	}
}

// TestGetNonceStore 测试获取 NonceStore 实例
func TestGetNonceStore(t *testing.T) {
	resetNonceStore()

	ns := GetNonceStore()
	if ns == nil {
		t.Fatal("GetNonceStore 不应返回 nil（应自动初始化）")
	}
}

// TestCleanup 测试清理过期 nonce
func TestCleanup(t *testing.T) {
	resetNonceStore()

	ns := InitNonceStore()
	ns.Add("to-be-cleaned")

	// 手动将过期时间设置为过去
	ns.mu.Lock()
	ns.store["to-be-cleaned"] = time.Now().Add(-1 * time.Hour)
	ns.mu.Unlock()

	// 执行清理
	ns.cleanup()

	// 验证 nonce 已被清理
	ns.mu.RLock()
	_, exists := ns.store["to-be-cleaned"]
	ns.mu.RUnlock()

	if exists {
		t.Error("过期的 nonce 应在 cleanup 后被删除")
	}
}
