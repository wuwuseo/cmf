package secureconfig

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func testKey(versionByte byte) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{versionByte}, 32))
}

func TestAESGCMCipherEncryptDecryptWithActiveVersion(t *testing.T) {
	keyring, err := NewKeyring("v2", map[string]string{
		"v1": testKey(1),
		"v2": testKey(2),
	})
	if err != nil {
		t.Fatalf("NewKeyring() error = %v", err)
	}
	cipher := NewAESGCMCipher(keyring)

	encrypted, err := cipher.Encrypt([]byte(`{"secret":"value"}`), []byte("captcha.provider.geetest_gt4"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if !strings.HasPrefix(encrypted, "enc:v2:") {
		t.Fatalf("Encrypt() = %q, want active key version prefix", encrypted)
	}

	decrypted, err := cipher.Decrypt(encrypted, []byte("captcha.provider.geetest_gt4"))
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if got, want := string(decrypted), `{"secret":"value"}`; got != want {
		t.Fatalf("Decrypt() = %q, want %q", got, want)
	}
}

func TestAESGCMCipherUsesRandomNonce(t *testing.T) {
	keyring, err := NewKeyring("v1", map[string]string{"v1": testKey(1)})
	if err != nil {
		t.Fatalf("NewKeyring() error = %v", err)
	}
	cipher := NewAESGCMCipher(keyring)

	first, err := cipher.Encrypt([]byte("same"), []byte("same-key"))
	if err != nil {
		t.Fatalf("first Encrypt() error = %v", err)
	}
	second, err := cipher.Encrypt([]byte("same"), []byte("same-key"))
	if err != nil {
		t.Fatalf("second Encrypt() error = %v", err)
	}
	if first == second {
		t.Fatal("Encrypt() produced identical ciphertexts for repeated plaintext")
	}
}

func TestAESGCMCipherRejectsDifferentAAD(t *testing.T) {
	keyring, err := NewKeyring("v1", map[string]string{"v1": testKey(1)})
	if err != nil {
		t.Fatalf("NewKeyring() error = %v", err)
	}
	cipher := NewAESGCMCipher(keyring)

	encrypted, err := cipher.Encrypt([]byte("secret"), []byte("captcha.provider.geetest_gt4"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if _, err := cipher.Decrypt(encrypted, []byte("captcha.provider.tencent")); err == nil {
		t.Fatal("Decrypt() with different AAD succeeded, want error")
	}
}

func TestAESGCMCipherDecryptsOldVersion(t *testing.T) {
	oldKeyring, err := NewKeyring("v1", map[string]string{"v1": testKey(1)})
	if err != nil {
		t.Fatalf("NewKeyring(old) error = %v", err)
	}
	oldCipher := NewAESGCMCipher(oldKeyring)
	encrypted, err := oldCipher.Encrypt([]byte("legacy"), []byte("config-key"))
	if err != nil {
		t.Fatalf("old Encrypt() error = %v", err)
	}

	rotatedKeyring, err := NewKeyring("v2", map[string]string{
		"v1": testKey(1),
		"v2": testKey(2),
	})
	if err != nil {
		t.Fatalf("NewKeyring(rotated) error = %v", err)
	}
	rotatedCipher := NewAESGCMCipher(rotatedKeyring)
	decrypted, err := rotatedCipher.Decrypt(encrypted, []byte("config-key"))
	if err != nil {
		t.Fatalf("Decrypt(old version) error = %v", err)
	}
	if got, want := string(decrypted), "legacy"; got != want {
		t.Fatalf("Decrypt(old version) = %q, want %q", got, want)
	}
}

func TestNewKeyringRejectsInvalidKeys(t *testing.T) {
	tests := []struct {
		name   string
		active string
		keys   map[string]string
	}{
		{name: "missing active version", active: "v2", keys: map[string]string{"v1": testKey(1)}},
		{name: "invalid base64", active: "v1", keys: map[string]string{"v1": "not-base64"}},
		{name: "wrong key length", active: "v1", keys: map[string]string{"v1": base64.StdEncoding.EncodeToString([]byte("short"))}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewKeyring(tt.active, tt.keys); err == nil {
				t.Fatal("NewKeyring() succeeded, want error")
			}
		})
	}
}
