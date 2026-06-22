package secureconfig

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
)

var errTestNotFound = errors.New("not found")

type memoryRepository struct {
	mu            sync.Mutex
	values        map[string]string
	forceConflict bool
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{values: make(map[string]string)}
}

func (r *memoryRepository) Get(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[key]
	if !ok {
		return "", errTestNotFound
	}
	return value, nil
}

func (r *memoryRepository) Put(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values[key] = value
	return nil
}

func (r *memoryRepository) ListPrefix(_ context.Context, prefix string) ([]Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries := make([]Entry, 0)
	for key, value := range r.values {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			entries = append(entries, Entry{Key: key, Value: value})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	return entries, nil
}

func (r *memoryRepository) CompareAndSwap(_ context.Context, key, oldValue, newValue string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.forceConflict || r.values[key] != oldValue {
		return false, nil
	}
	r.values[key] = newValue
	return true, nil
}

func TestManagerSetAndGetJSON(t *testing.T) {
	repo := newMemoryRepository()
	keyring, err := NewKeyring("v1", map[string]string{"v1": testKey(1)})
	if err != nil {
		t.Fatalf("NewKeyring() error = %v", err)
	}
	manager := NewManager(repo, NewAESGCMCipher(keyring))

	input := map[string]any{"captcha_id": "public", "captcha_key": "secret"}
	if err := manager.SetJSON(t.Context(), "captcha.provider.geetest_gt4", input); err != nil {
		t.Fatalf("SetJSON() error = %v", err)
	}
	stored := repo.values["captcha.provider.geetest_gt4"]
	if stored == "" || stored == `{"captcha_id":"public","captcha_key":"secret"}` {
		t.Fatalf("repository stored plaintext or empty value: %q", stored)
	}

	var output map[string]any
	if err := manager.GetJSON(t.Context(), "captcha.provider.geetest_gt4", &output); err != nil {
		t.Fatalf("GetJSON() error = %v", err)
	}
	if output["captcha_key"] != "secret" {
		t.Fatalf("GetJSON() captcha_key = %v, want secret", output["captcha_key"])
	}
}

func TestManagerInspectReturnsVersionWithoutDecrypting(t *testing.T) {
	repo := newMemoryRepository()
	keyring, err := NewKeyring("v3", map[string]string{"v3": testKey(3)})
	if err != nil {
		t.Fatalf("NewKeyring() error = %v", err)
	}
	manager := NewManager(repo, NewAESGCMCipher(keyring))
	if err := manager.SetJSON(t.Context(), "captcha.provider.geetest_gt4", map[string]string{"key": "secret"}); err != nil {
		t.Fatalf("SetJSON() error = %v", err)
	}

	metadata, err := manager.Inspect(t.Context(), "captcha.provider.geetest_gt4")
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if metadata.KeyVersion != "v3" || !metadata.Encrypted {
		t.Fatalf("Inspect() = %+v, want encrypted v3", metadata)
	}
}

func TestManagerReencryptDryRunDoesNotWrite(t *testing.T) {
	repo := newMemoryRepository()
	oldKeyring, err := NewKeyring("v1", map[string]string{"v1": testKey(1)})
	if err != nil {
		t.Fatalf("NewKeyring(old) error = %v", err)
	}
	oldManager := NewManager(repo, NewAESGCMCipher(oldKeyring))
	if err := oldManager.SetJSON(t.Context(), "captcha.provider.geetest_gt4", map[string]string{"key": "secret"}); err != nil {
		t.Fatalf("old SetJSON() error = %v", err)
	}
	before := repo.values["captcha.provider.geetest_gt4"]

	rotatedKeyring, err := NewKeyring("v2", map[string]string{"v1": testKey(1), "v2": testKey(2)})
	if err != nil {
		t.Fatalf("NewKeyring(rotated) error = %v", err)
	}
	manager := NewManager(repo, NewAESGCMCipher(rotatedKeyring))
	report, err := manager.Reencrypt(t.Context(), "captcha.provider.", true)
	if err != nil {
		t.Fatalf("Reencrypt(dry-run) error = %v", err)
	}
	if report.Scanned != 1 || report.WouldReencrypt != 1 || report.Reencrypted != 0 {
		t.Fatalf("Reencrypt(dry-run) report = %+v", report)
	}
	if got := repo.values["captcha.provider.geetest_gt4"]; got != before {
		t.Fatal("Reencrypt(dry-run) changed repository value")
	}
}

func TestManagerReencryptUsesCompareAndSwap(t *testing.T) {
	repo := newMemoryRepository()
	oldKeyring, err := NewKeyring("v1", map[string]string{"v1": testKey(1)})
	if err != nil {
		t.Fatalf("NewKeyring(old) error = %v", err)
	}
	if err := NewManager(repo, NewAESGCMCipher(oldKeyring)).SetJSON(
		t.Context(),
		"captcha.provider.geetest_gt4",
		map[string]string{"key": "secret"},
	); err != nil {
		t.Fatalf("old SetJSON() error = %v", err)
	}

	rotatedKeyring, err := NewKeyring("v2", map[string]string{"v1": testKey(1), "v2": testKey(2)})
	if err != nil {
		t.Fatalf("NewKeyring(rotated) error = %v", err)
	}
	manager := NewManager(repo, NewAESGCMCipher(rotatedKeyring))
	report, err := manager.Reencrypt(t.Context(), "captcha.provider.", false)
	if err != nil {
		t.Fatalf("Reencrypt() error = %v", err)
	}
	if report.Reencrypted != 1 || report.Conflicts != 0 {
		t.Fatalf("Reencrypt() report = %+v", report)
	}
	metadata, err := manager.Inspect(t.Context(), "captcha.provider.geetest_gt4")
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if metadata.KeyVersion != "v2" {
		t.Fatalf("Inspect() version = %q, want v2", metadata.KeyVersion)
	}

	repo.forceConflict = true
	if err := NewManager(repo, NewAESGCMCipher(oldKeyring)).SetJSON(
		t.Context(),
		"captcha.provider.other",
		map[string]string{"key": "secret"},
	); err != nil {
		t.Fatalf("old SetJSON(other) error = %v", err)
	}
	report, err = manager.Reencrypt(t.Context(), "captcha.provider.other", false)
	if err != nil {
		t.Fatalf("Reencrypt(conflict) error = %v", err)
	}
	if report.Conflicts != 1 || report.Reencrypted != 0 {
		t.Fatalf("Reencrypt(conflict) report = %+v", report)
	}
}
