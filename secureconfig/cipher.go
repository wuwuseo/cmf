package secureconfig

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const ciphertextPrefix = "enc"

type Keyring struct {
	active string
	keys   map[string][]byte
}

func NewKeyring(active string, encodedKeys map[string]string) (*Keyring, error) {
	if active == "" {
		return nil, fmt.Errorf("active key version is required")
	}
	keys := make(map[string][]byte, len(encodedKeys))
	for version, encoded := range encodedKeys {
		if version == "" {
			return nil, fmt.Errorf("key version is required")
		}
		key, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode key %q: %w", version, err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("key %q must be 32 bytes", version)
		}
		keys[version] = append([]byte(nil), key...)
	}
	if _, ok := keys[active]; !ok {
		return nil, fmt.Errorf("active key version %q is not configured", active)
	}
	return &Keyring{active: active, keys: keys}, nil
}

func (k *Keyring) ActiveVersion() string {
	return k.active
}

func (k *Keyring) key(version string) ([]byte, error) {
	key, ok := k.keys[version]
	if !ok {
		return nil, fmt.Errorf("key version %q is not configured", version)
	}
	return key, nil
}

type AESGCMCipher struct {
	keyring *Keyring
	rand    io.Reader
}

func NewAESGCMCipher(keyring *Keyring) *AESGCMCipher {
	return &AESGCMCipher{keyring: keyring, rand: rand.Reader}
}

func (c *AESGCMCipher) ActiveVersion() string {
	if c == nil || c.keyring == nil {
		return ""
	}
	return c.keyring.ActiveVersion()
}

func (c *AESGCMCipher) Inspect(ciphertext string) (Metadata, error) {
	version, _, err := parseCiphertext(ciphertext)
	if err != nil {
		return Metadata{}, err
	}
	return Metadata{Encrypted: true, KeyVersion: version}, nil
}

func (c *AESGCMCipher) Encrypt(plaintext, aad []byte) (string, error) {
	if c == nil || c.keyring == nil {
		return "", fmt.Errorf("keyring is required")
	}
	version := c.keyring.ActiveVersion()
	key, err := c.keyring.key(version)
	if err != nil {
		return "", err
	}
	aead, err := newGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(c.rand, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := aead.Seal(nil, nonce, plaintext, aad)
	payload := append(nonce, sealed...)
	return ciphertextPrefix + ":" + version + ":" + base64.RawURLEncoding.EncodeToString(payload), nil
}

func (c *AESGCMCipher) Decrypt(ciphertext string, aad []byte) ([]byte, error) {
	if c == nil || c.keyring == nil {
		return nil, fmt.Errorf("keyring is required")
	}
	version, payload, err := parseCiphertext(ciphertext)
	if err != nil {
		return nil, err
	}
	key, err := c.keyring.key(version)
	if err != nil {
		return nil, err
	}
	aead, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(payload) < aead.NonceSize() {
		return nil, fmt.Errorf("ciphertext payload is too short")
	}
	nonce, sealed := payload[:aead.NonceSize()], payload[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, sealed, aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt ciphertext: %w", err)
	}
	return plaintext, nil
}

func parseCiphertext(value string) (string, []byte, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 3 || parts[0] != ciphertextPrefix || parts[1] == "" || parts[2] == "" {
		return "", nil, fmt.Errorf("invalid ciphertext format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", nil, fmt.Errorf("decode ciphertext: %w", err)
	}
	return parts[1], payload, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	return aead, nil
}
