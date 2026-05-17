package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"sync"
	"testing"
	"time"
)

// resetKeyManager 重置 KeyManager 单例状态（用于测试隔离）
func resetKeyManager() {
	keyManager = nil
	keyOnce = sync.Once{}
}

// TestInitKeyManager 测试创建密钥管理器
func TestInitKeyManager(t *testing.T) {
	resetKeyManager()

	km := InitKeyManager(7*24*time.Hour, 24*time.Hour)
	if km == nil {
		t.Fatal("InitKeyManager 不应返回 nil")
	}
	if km.keyTTL != 7*24*time.Hour {
		t.Errorf("keyTTL 应为 7 天，实际为 %v", km.keyTTL)
	}
	if km.rotateInterval != 24*time.Hour {
		t.Errorf("rotateInterval 应为 24 小时，实际为 %v", km.rotateInterval)
	}
	if len(km.keys) != 1 {
		t.Errorf("初始化后应有 1 个密钥，实际为 %d", len(km.keys))
	}
}

// TestGetActiveKeyID 测试获取活跃密钥 ID
func TestGetActiveKeyID(t *testing.T) {
	resetKeyManager()

	km := InitKeyManager(7*24*time.Hour, 24*time.Hour)
	keyID := km.GetActiveKeyID()

	if keyID == "" {
		t.Error("活跃密钥 ID 不应为空")
	}
}

// TestGetPublicKeyPEM_KeyManager 测试获取 PEM 格式公钥
func TestGetPublicKeyPEM_KeyManager(t *testing.T) {
	resetKeyManager()

	km := InitKeyManager(7*24*time.Hour, 24*time.Hour)
	keyID, pemStr, err := km.GetPublicKeyPEM()
	if err != nil {
		t.Fatalf("GetPublicKeyPEM 失败: %v", err)
	}
	if keyID == "" {
		t.Error("keyID 不应为空")
	}
	if pemStr == "" {
		t.Error("PEM 字符串不应为空")
	}
	if !containsString(pemStr, "PUBLIC KEY") {
		t.Error("PEM 字符串应包含 'PUBLIC KEY'")
	}
}

// TestDecryptData_KeyManager 测试 KeyManager 加密解密流程
func TestDecryptData_KeyManager(t *testing.T) {
	resetKeyManager()

	km := InitKeyManager(7*24*time.Hour, 24*time.Hour)

	// 获取活跃密钥和公钥
	km.mu.RLock()
	keyVersion := km.keys[km.activeKey]
	km.mu.RUnlock()

	pubKey := keyVersion.PublicKey
	keyID := keyVersion.KeyID

	plaintext := "Hello, KeyManager!"

	// 使用公钥加密
	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, []byte(plaintext), nil)
	if err != nil {
		t.Fatalf("RSA 加密失败: %v", err)
	}

	encryptedBase64 := base64.StdEncoding.EncodeToString(ciphertext)

	// 使用 DecryptData 解密
	decrypted, err := km.DecryptData(keyID, encryptedBase64)
	if err != nil {
		t.Fatalf("DecryptData 失败: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("解密结果不匹配，期望: %s, 实际: %s", plaintext, decrypted)
	}
}

// TestDecryptData_NonExistentKeyID 测试不存在的 keyID
func TestDecryptData_NonExistentKeyID(t *testing.T) {
	resetKeyManager()

	km := InitKeyManager(7*24*time.Hour, 24*time.Hour)

	_, err := km.DecryptData("nonexistent-key", "some-base64-data")
	if err == nil {
		t.Error("不存在的 keyID 应返回错误")
	}
}

// TestDecryptData_ExpiredKey 测试使用过期密钥解密
func TestDecryptData_ExpiredKey(t *testing.T) {
	resetKeyManager()

	km := InitKeyManager(7*24*time.Hour, 24*time.Hour)

	// 获取活跃密钥
	km.mu.RLock()
	keyVersion := km.keys[km.activeKey]
	keyID := keyVersion.KeyID
	pubKey := keyVersion.PublicKey
	km.mu.RUnlock()

	plaintext := "Expired Key Test"
	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, []byte(plaintext), nil)
	if err != nil {
		t.Fatalf("RSA 加密失败: %v", err)
	}
	encryptedBase64 := base64.StdEncoding.EncodeToString(ciphertext)

	// 手动使密钥过期
	km.mu.Lock()
	km.keys[keyID].ExpiresAt = time.Now().Add(-1 * time.Second)
	km.mu.Unlock()

	// 尝试解密，应失败
	_, err = km.DecryptData(keyID, encryptedBase64)
	if err == nil {
		t.Error("过期密钥解密应返回错误")
	}
}

// TestGetKeyManager_Singleton 测试单例模式
func TestGetKeyManager_Singleton(t *testing.T) {
	resetKeyManager()

	km1 := GetKeyManager()
	km2 := GetKeyManager()

	if km1 != km2 {
		t.Error("两次 GetKeyManager 应返回同一实例（单例模式）")
	}
	if km1 == nil {
		t.Error("GetKeyManager 不应返回 nil")
	}
}

// TestKeyRotation 测试密钥轮换（生成新密钥时旧密钥变为非活跃）
func TestKeyRotation(t *testing.T) {
	resetKeyManager()

	km := InitKeyManager(7*24*time.Hour, 24*time.Hour)

	// 获取旧密钥 ID
	oldKeyID := km.GetActiveKeyID()

	// 验证旧密钥是活跃的
	km.mu.RLock()
	if !km.keys[oldKeyID].IsActive {
		t.Error("旧密钥应为活跃状态")
	}
	km.mu.RUnlock()

	// 等待至少 1 秒确保 keyID（基于 Unix 时间戳）不同
	time.Sleep(1100 * time.Millisecond)

	// 执行密钥轮换
	km.generateKey()

	// 获取新密钥 ID
	newKeyID := km.GetActiveKeyID()

	if newKeyID == oldKeyID {
		t.Error("轮换后活跃密钥 ID 应有变化")
	}

	// 验证旧密钥变为非活跃
	km.mu.RLock()
	if km.keys[oldKeyID].IsActive {
		t.Error("轮换后旧密钥应为非活跃状态")
	}
	if !km.keys[newKeyID].IsActive {
		t.Error("轮换后新密钥应为活跃状态")
	}
	// 验证密钥总数
	if len(km.keys) != 2 {
		t.Errorf("轮换后应有 2 个密钥，实际为 %d", len(km.keys))
	}
	km.mu.RUnlock()
}
