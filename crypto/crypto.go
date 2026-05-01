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

	"golang.org/x/crypto/bcrypt"
)

var (
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	once       sync.Once
)

// HashPassword 使用 bcrypt 算法和随机盐值对密码进行加密
// 参数:
//   - password: 原始密码
//   - cost: bcrypt 的计算成本(可选，默认为 10)
//
// 返回:
//   - hashedPassword: 加密后的密码
//   - salt: 使用的盐值
//   - error: 错误信息
func HashPassword(password string, cost ...int) (string, string, error) {
	// 生成随机盐值 (16 字节)
	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		return "", "", err
	}

	// 将盐值转换为 base64 字符串
	salt := base64.StdEncoding.EncodeToString(saltBytes)

	// 将密码和盐值组合
	saltedPassword := password + salt

	// 设置默认计算成本
	bcryptCost := 10
	if len(cost) > 0 && cost[0] > 0 {
		bcryptCost = cost[0]
	}

	// 使用 bcrypt 进行加密
	hash, err := bcrypt.GenerateFromPassword([]byte(saltedPassword), bcryptCost)
	if err != nil {
		return "", "", err
	}

	// 将加密后的密码转换为 base64 字符串
	hashedPassword := base64.StdEncoding.EncodeToString(hash)

	return hashedPassword, salt, nil
}

// VerifyPassword 验证密码是否正确
// 参数:
//   - password: 待验证的原始密码
//   - hashedPassword: 之前加密的密码
//   - salt: 之前使用的盐值
//
// 返回:
//   - bool: 密码是否匹配
//   - error: 错误信息
func VerifyPassword(password, hashedPassword, salt string) (bool, error) {
	// 解码已加密的密码
	hash, err := base64.StdEncoding.DecodeString(hashedPassword)
	if err != nil {
		return false, err
	}

	// 将密码和盐值组合
	saltedPassword := password + salt

	// 验证密码
	err = bcrypt.CompareHashAndPassword(hash, []byte(saltedPassword))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// InitRSAKeys 初始化RSA密钥对（单例模式）
func InitRSAKeys() error {
	var err error
	once.Do(func() {
		privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return
		}
		publicKey = &privateKey.PublicKey
	})
	return err
}

// GetPublicKeyPEM 获取公钥的PEM格式字符串
func GetPublicKeyPEM() (string, error) {
	if publicKey == nil {
		if err := InitRSAKeys(); err != nil {
			return "", err
		}
	}

	pubASN1, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}

	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubASN1,
	})

	return string(pubPEM), nil
}

// DecryptData 使用私钥解密数据（Base64编码的密文）
func DecryptData(encryptedBase64 string) (string, error) {
	if privateKey == nil {
		if err := InitRSAKeys(); err != nil {
			return "", err
		}
	}

	// 解码Base64
	encryptedData, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", fmt.Errorf("base64 decode error: %w", err)
	}

	// 解密数据
	decryptedData, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, encryptedData, nil)
	if err != nil {
		return "", fmt.Errorf("rsa decrypt error: %w", err)
	}

	return string(decryptedData), nil
}

// GetPrivateKey 获取私钥（用于测试或其他用途）
func GetPrivateKey() *rsa.PrivateKey {
	if privateKey == nil {
		if err := InitRSAKeys(); err != nil {
			return nil
		}
	}
	return privateKey
}

// GetPublicKey 获取公钥
func GetPublicKey() *rsa.PublicKey {
	if publicKey == nil {
		if err := InitRSAKeys(); err != nil {
			return nil
		}
	}
	return publicKey
}
