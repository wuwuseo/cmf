package secureconfig

import (
	"context"
	"encoding/json"
	"fmt"
)

type Entry struct {
	Key   string
	Value string
}

type Repository interface {
	Get(ctx context.Context, key string) (string, error)
	Put(ctx context.Context, key, value string) error
	ListPrefix(ctx context.Context, prefix string) ([]Entry, error)
	CompareAndSwap(ctx context.Context, key, oldValue, newValue string) (bool, error)
}

type Cipher interface {
	Encrypt(plaintext, aad []byte) (string, error)
	Decrypt(ciphertext string, aad []byte) ([]byte, error)
	Inspect(ciphertext string) (Metadata, error)
	ActiveVersion() string
}

type Metadata struct {
	Encrypted  bool   `json:"encrypted"`
	KeyVersion string `json:"key_version"`
}

type ReencryptReport struct {
	Scanned        int               `json:"scanned"`
	AlreadyActive  int               `json:"already_active"`
	WouldReencrypt int               `json:"would_reencrypt"`
	Reencrypted    int               `json:"reencrypted"`
	Conflicts      int               `json:"conflicts"`
	Failures       map[string]string `json:"failures,omitempty"`
}

type Manager struct {
	repository Repository
	cipher     Cipher
}

func NewManager(repository Repository, cipher Cipher) *Manager {
	return &Manager{repository: repository, cipher: cipher}
}

func (m *Manager) SetJSON(ctx context.Context, key string, value any) error {
	if m == nil || m.repository == nil || m.cipher == nil {
		return fmt.Errorf("secure config manager is not configured")
	}
	plaintext, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal secure config %q: %w", key, err)
	}
	encrypted, err := m.cipher.Encrypt(plaintext, []byte(key))
	if err != nil {
		return fmt.Errorf("encrypt secure config %q: %w", key, err)
	}
	if err := m.repository.Put(ctx, key, encrypted); err != nil {
		return fmt.Errorf("store secure config %q: %w", key, err)
	}
	return nil
}

func (m *Manager) GetJSON(ctx context.Context, key string, target any) error {
	if m == nil || m.repository == nil || m.cipher == nil {
		return fmt.Errorf("secure config manager is not configured")
	}
	encrypted, err := m.repository.Get(ctx, key)
	if err != nil {
		return err
	}
	plaintext, err := m.cipher.Decrypt(encrypted, []byte(key))
	if err != nil {
		return fmt.Errorf("decrypt secure config %q: %w", key, err)
	}
	if err := json.Unmarshal(plaintext, target); err != nil {
		return fmt.Errorf("unmarshal secure config %q: %w", key, err)
	}
	return nil
}

func (m *Manager) Inspect(ctx context.Context, key string) (Metadata, error) {
	if m == nil || m.repository == nil || m.cipher == nil {
		return Metadata{}, fmt.Errorf("secure config manager is not configured")
	}
	encrypted, err := m.repository.Get(ctx, key)
	if err != nil {
		return Metadata{}, err
	}
	return m.cipher.Inspect(encrypted)
}

func (m *Manager) Reencrypt(ctx context.Context, prefix string, dryRun bool) (ReencryptReport, error) {
	report := ReencryptReport{Failures: make(map[string]string)}
	if m == nil || m.repository == nil || m.cipher == nil {
		return report, fmt.Errorf("secure config manager is not configured")
	}
	entries, err := m.repository.ListPrefix(ctx, prefix)
	if err != nil {
		return report, fmt.Errorf("list secure configs: %w", err)
	}
	for _, entry := range entries {
		report.Scanned++
		metadata, err := m.cipher.Inspect(entry.Value)
		if err != nil {
			report.Failures[entry.Key] = err.Error()
			continue
		}
		if metadata.KeyVersion == m.cipher.ActiveVersion() {
			report.AlreadyActive++
			continue
		}
		report.WouldReencrypt++
		if dryRun {
			continue
		}
		plaintext, err := m.cipher.Decrypt(entry.Value, []byte(entry.Key))
		if err != nil {
			report.Failures[entry.Key] = err.Error()
			continue
		}
		encrypted, err := m.cipher.Encrypt(plaintext, []byte(entry.Key))
		if err != nil {
			report.Failures[entry.Key] = err.Error()
			continue
		}
		swapped, err := m.repository.CompareAndSwap(ctx, entry.Key, entry.Value, encrypted)
		if err != nil {
			report.Failures[entry.Key] = err.Error()
			continue
		}
		if !swapped {
			report.Conflicts++
			continue
		}
		report.Reencrypted++
	}
	if len(report.Failures) == 0 {
		report.Failures = nil
	}
	return report, nil
}
