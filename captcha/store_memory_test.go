package captcha

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryStoreConsumeIsAtomic(t *testing.T) {
	store := NewMemoryStore()
	if err := store.SaveProof(t.Context(), "proof", ProofRecord{Scene: "login"}, time.Minute); err != nil {
		t.Fatalf("SaveProof() error = %v", err)
	}

	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.ConsumeProof(t.Context(), "proof"); err == nil {
				successes.Add(1)
			} else if !errors.Is(err, ErrStoreNotFound) {
				t.Errorf("ConsumeProof() error = %v", err)
			}
		}()
	}
	wg.Wait()
	if got := successes.Load(); got != 1 {
		t.Fatalf("successful consumes = %d, want 1", got)
	}
}

func TestMemoryStoreExpiresRecords(t *testing.T) {
	store := NewMemoryStore()
	if err := store.SaveChallenge(t.Context(), "challenge", ChallengeRecord{}, time.Nanosecond); err != nil {
		t.Fatalf("SaveChallenge() error = %v", err)
	}
	time.Sleep(time.Millisecond)
	if _, err := store.ConsumeChallenge(t.Context(), "challenge"); !errors.Is(err, ErrStoreNotFound) {
		t.Fatalf("ConsumeChallenge() error = %v, want ErrStoreNotFound", err)
	}
}

func TestMemoryStoreRemovesExpiredRecordsDuringWrites(t *testing.T) {
	store := NewMemoryStore()
	if err := store.SaveChallenge(
		t.Context(),
		"expired-challenge",
		ChallengeRecord{Scene: "login"},
		-time.Second,
	); err != nil {
		t.Fatalf("SaveChallenge(expired) error = %v", err)
	}
	if err := store.SaveProof(
		t.Context(),
		"expired-proof",
		ProofRecord{Scene: "login"},
		-time.Second,
	); err != nil {
		t.Fatalf("SaveProof(expired) error = %v", err)
	}
	if err := store.SaveChallenge(
		t.Context(),
		"active",
		ChallengeRecord{Scene: "login"},
		time.Minute,
	); err != nil {
		t.Fatalf("SaveChallenge(active) error = %v", err)
	}
	if _, ok := store.challenges["expired-challenge"]; ok {
		t.Fatal("expired challenge was not cleaned")
	}
	if _, ok := store.proofs["expired-proof"]; ok {
		t.Fatal("expired proof was not cleaned")
	}
}
