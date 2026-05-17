package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"sync"
	"testing"
)

// resetRSAKeys 重置 RSA 密钥单例状态（用于测试隔离）
func resetRSAKeys() {
	privateKey = nil
	publicKey = nil
	once = sync.Once{}
}

// TestHashPassword_Normal 测试正常的密码加密
func TestHashPassword_Normal(t *testing.T) {
	hashed, salt, err := HashPassword("my-password")
	if err != nil {
		t.Fatalf("HashPassword 失败: %v", err)
	}
	if hashed == "" {
		t.Error("加密后的密码不应为空")
	}
	if salt == "" {
		t.Error("盐值不应为空")
	}

	// 验证盐值的 base64 长度（16字节原始数据 -> base64）
	saltBytes, err := base64.StdEncoding.DecodeString(salt)
	if err != nil {
		t.Fatalf("盐值不是有效的 base64 编码: %v", err)
	}
	if len(saltBytes) != 16 {
		t.Errorf("盐值原始长度应为 16 字节，实际为 %d", len(saltBytes))
	}
}

// TestHashPassword_WithCost 测试指定 cost 的密码加密
func TestHashPassword_WithCost(t *testing.T) {
	hashed, salt, err := HashPassword("my-password", 5)
	if err != nil {
		t.Fatalf("指定 cost 的 HashPassword 失败: %v", err)
	}
	if hashed == "" {
		t.Error("加密后的密码不应为空")
	}
	if salt == "" {
		t.Error("盐值不应为空")
	}
}

// TestVerifyPassword_CorrectPassword 测试正确密码验证
func TestVerifyPassword_CorrectPassword(t *testing.T) {
	password := "correct-password"
	hashed, salt, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword 失败: %v", err)
	}

	ok, err := VerifyPassword(password, hashed, salt)
	if err != nil {
		t.Fatalf("VerifyPassword 失败: %v", err)
	}
	if !ok {
		t.Error("正确密码应该验证通过")
	}
}

// TestVerifyPassword_WrongPassword 测试错误密码验证
func TestVerifyPassword_WrongPassword(t *testing.T) {
	password := "correct-password"
	hashed, salt, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword 失败: %v", err)
	}

	ok, err := VerifyPassword("wrong-password", hashed, salt)
	if err != nil {
		t.Fatalf("VerifyPassword 失败: %v", err)
	}
	if ok {
		t.Error("错误密码不应该验证通过")
	}
}

// TestVerifyPassword_InvalidHash 测试无效的 base64 哈希
func TestVerifyPassword_InvalidHash(t *testing.T) {
	_, err := VerifyPassword("password", "invalid-base64!!!", "salt")
	if err == nil {
		t.Error("无效的 base64 哈希应该返回错误")
	}
}

// TestInitRSAKeys_Init 测试 RSA 密钥初始化
func TestInitRSAKeys_Init(t *testing.T) {
	resetRSAKeys()

	err := InitRSAKeys()
	if err != nil {
		t.Fatalf("InitRSAKeys 失败: %v", err)
	}
	if privateKey == nil {
		t.Error("初始化后私钥不应为 nil")
	}
	if publicKey == nil {
		t.Error("初始化后公钥不应为 nil")
	}
}

// TestGetPublicKeyPEM 测试获取 PEM 格式公钥
func TestGetPublicKeyPEM(t *testing.T) {
	resetRSAKeys()

	pemStr, err := GetPublicKeyPEM()
	if err != nil {
		t.Fatalf("GetPublicKeyPEM 失败: %v", err)
	}
	if pemStr == "" {
		t.Error("PEM 公钥字符串不应为空")
	}
	if !containsString(pemStr, "PUBLIC KEY") {
		t.Error("PEM 字符串应包含 'PUBLIC KEY'")
	}
}

// TestDecryptData_EncryptDecrypt 测试 RSA 加密解密流程
func TestDecryptData_EncryptDecrypt(t *testing.T) {
	resetRSAKeys()

	err := InitRSAKeys()
	if err != nil {
		t.Fatalf("InitRSAKeys 失败: %v", err)
	}

	pubKey := GetPublicKey()
	if pubKey == nil {
		t.Fatal("公钥不应为 nil")
	}

	plaintext := "Hello, RSA!"

	// 使用公钥加密
	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, []byte(plaintext), nil)
	if err != nil {
		t.Fatalf("RSA 加密失败: %v", err)
	}

	// Base64 编码
	encryptedBase64 := base64.StdEncoding.EncodeToString(ciphertext)

	// 使用 DecryptData 解密
	decrypted, err := DecryptData(encryptedBase64)
	if err != nil {
		t.Fatalf("DecryptData 失败: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("解密结果不匹配，期望: %s, 实际: %s", plaintext, decrypted)
	}
}

// TestGetPrivateKey_GetPublicKey 测试获取私钥和公钥
func TestGetPrivateKey_GetPublicKey(t *testing.T) {
	resetRSAKeys()

	privKey := GetPrivateKey()
	if privKey == nil {
		t.Fatal("GetPrivateKey 不应返回 nil（应自动初始化）")
	}

	pubKey := GetPublicKey()
	if pubKey == nil {
		t.Fatal("GetPublicKey 不应返回 nil（应自动初始化）")
	}
}

// containsString 检查字符串是否包含子串
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
