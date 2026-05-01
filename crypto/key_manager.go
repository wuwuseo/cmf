package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"sync"
	"time"
)

// KeyVersion 密钥版本信息
type KeyVersion struct {
	KeyID      string
	PublicKey  *rsa.PublicKey
	PrivateKey *rsa.PrivateKey
	CreatedAt  time.Time
	ExpiresAt  time.Time
	IsActive   bool
}

// KeyManager 密钥管理器
type KeyManager struct {
	keys           map[string]*KeyVersion
	activeKey      string
	mu             sync.RWMutex
	keyTTL         time.Duration
	rotateInterval time.Duration
}

var (
	keyManager *KeyManager
	keyOnce    sync.Once
)

// InitKeyManager 初始化密钥管理器
func InitKeyManager(keyTTL, rotateInterval time.Duration) *KeyManager {
	keyOnce.Do(func() {
		keyManager = &KeyManager{
			keys:           make(map[string]*KeyVersion),
			keyTTL:         keyTTL,
			rotateInterval: rotateInterval,
		}
		// 生成初始密钥
		keyManager.generateKey()
		// 启动密钥轮换定时任务
		go keyManager.startKeyRotation()
		// 启动清理过期密钥任务
		go keyManager.startCleanup()
	})
	return keyManager
}

// generateKey 生成新的密钥对
func (km *KeyManager) generateKey() {
	km.mu.Lock()
	defer km.mu.Unlock()

	// 将当前活跃密钥标记为非活跃
	for _, key := range km.keys {
		if key.IsActive {
			key.IsActive = false
		}
	}

	// 生成新的RSA密钥对
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(fmt.Sprintf("failed to generate RSA key: %v", err))
	}

	// 生成密钥ID
	keyID := fmt.Sprintf("my-key-%d", time.Now().Unix())
	now := time.Now()

	// 创建密钥版本
	keyVersion := &KeyVersion{
		KeyID:      keyID,
		PublicKey:  &privateKey.PublicKey,
		PrivateKey: privateKey,
		CreatedAt:  now,
		ExpiresAt:  now.Add(km.keyTTL),
		IsActive:   true,
	}

	km.keys[keyID] = keyVersion
	km.activeKey = keyID
}

// GetPublicKeyPEM 获取当前活跃公钥的PEM格式
func (km *KeyManager) GetPublicKeyPEM() (string, string, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	keyVersion, exists := km.keys[km.activeKey]
	if !exists {
		return "", "", fmt.Errorf("active key not found")
	}

	pubASN1, err := x509.MarshalPKIXPublicKey(keyVersion.PublicKey)
	if err != nil {
		return "", "", err
	}

	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubASN1,
	})

	return keyVersion.KeyID, string(pubPEM), nil
}

// DecryptData 使用指定密钥ID解密数据
func (km *KeyManager) DecryptData(keyID, encryptedBase64 string) (string, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	keyVersion, exists := km.keys[keyID]
	if !exists {
		return "", fmt.Errorf("key version %s not found", keyID)
	}

	// 检查密钥是否过期
	if time.Now().After(keyVersion.ExpiresAt) {
		return "", fmt.Errorf("key version %s has expired", keyID)
	}

	// 解码Base64
	encryptedData, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", fmt.Errorf("base64 decode error: %w", err)
	}

	// 解密数据
	decryptedData, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, keyVersion.PrivateKey, encryptedData, nil)
	if err != nil {
		return "", fmt.Errorf("rsa decrypt error: %w", err)
	}

	return string(decryptedData), nil
}

// startKeyRotation 定期轮换密钥
func (km *KeyManager) startKeyRotation() {
	ticker := time.NewTicker(km.rotateInterval)
	defer ticker.Stop()

	for range ticker.C {
		km.generateKey()
	}
}

// startCleanup 清理过期的密钥
func (km *KeyManager) startCleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		km.cleanup()
	}
}

// cleanup 清理过期密钥
func (km *KeyManager) cleanup() {
	km.mu.Lock()
	defer km.mu.Unlock()

	now := time.Now()
	for keyID, keyVersion := range km.keys {
		// 保留最近24小时的密钥（用于解密可能延迟的请求）
		if now.After(keyVersion.ExpiresAt.Add(24 * time.Hour)) {
			delete(km.keys, keyID)
		}
	}
}

// GetKeyManager 获取密钥管理器实例
func GetKeyManager() *KeyManager {
	if keyManager == nil {
		InitKeyManager(7*24*time.Hour, 24*time.Hour) // 默认密钥有效期7天，每天轮换
	}
	return keyManager
}

// GetActiveKeyID 获取当前活跃密钥ID
func (km *KeyManager) GetActiveKeyID() string {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.activeKey
}
