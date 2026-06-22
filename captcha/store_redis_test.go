package captcha

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type fakeRedisAtomicClient struct {
	mu     sync.Mutex
	values map[string]string
	ttls   map[string]time.Duration
}

func newFakeRedisAtomicClient() *fakeRedisAtomicClient {
	return &fakeRedisAtomicClient{values: make(map[string]string), ttls: make(map[string]time.Duration)}
}

func (f *fakeRedisAtomicClient) Set(_ context.Context, key, value string, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.values[key] = value
	f.ttls[key] = ttl
	return nil
}

func (f *fakeRedisAtomicClient) GetDel(_ context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	value, ok := f.values[key]
	delete(f.values, key)
	if !ok {
		return "", redis.Nil
	}
	return value, nil
}

func TestRedisStoreUsesNamespacedAtomicRecords(t *testing.T) {
	client := newFakeRedisAtomicClient()
	store := newRedisStore(client, "app:captcha")

	if err := store.SaveChallenge(
		t.Context(),
		"challenge-id",
		ChallengeRecord{Scene: "login", Provider: "math"},
		time.Minute,
	); err != nil {
		t.Fatalf("SaveChallenge() error = %v", err)
	}
	if got := client.ttls["app:captcha:challenge:challenge-id"]; got != time.Minute {
		t.Fatalf("stored TTL = %v, want %v", got, time.Minute)
	}

	record, err := store.ConsumeChallenge(t.Context(), "challenge-id")
	if err != nil {
		t.Fatalf("ConsumeChallenge() error = %v", err)
	}
	if record.Scene != "login" || record.Provider != "math" {
		t.Fatalf("ConsumeChallenge() = %+v", record)
	}
	if _, err := store.ConsumeChallenge(t.Context(), "challenge-id"); !errors.Is(err, ErrStoreNotFound) {
		t.Fatalf("second ConsumeChallenge() error = %v, want ErrStoreNotFound", err)
	}
}

func TestRedisStoreRejectsMalformedRecord(t *testing.T) {
	client := newFakeRedisAtomicClient()
	client.values["captcha:proof:bad"] = "{"
	store := newRedisStore(client, "captcha")

	if _, err := store.ConsumeProof(t.Context(), "bad"); err == nil {
		t.Fatal("ConsumeProof() succeeded for malformed JSON")
	}
}
