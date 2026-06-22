package captcha

import (
	"context"
	"sync"
	"time"
)

type memoryValue[T any] struct {
	value     T
	expiresAt time.Time
}

type MemoryStore struct {
	mu         sync.Mutex
	challenges map[string]memoryValue[ChallengeRecord]
	proofs     map[string]memoryValue[ProofRecord]
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		challenges: make(map[string]memoryValue[ChallengeRecord]),
		proofs:     make(map[string]memoryValue[ProofRecord]),
	}
}

func (s *MemoryStore) SaveChallenge(_ context.Context, id string, record ChallengeRecord, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.cleanupExpired(now)
	s.challenges[id] = memoryValue[ChallengeRecord]{value: record, expiresAt: now.Add(ttl)}
	return nil
}

func (s *MemoryStore) ConsumeChallenge(_ context.Context, id string) (ChallengeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.challenges[id]
	delete(s.challenges, id)
	if !ok || time.Now().After(value.expiresAt) {
		return ChallengeRecord{}, ErrStoreNotFound
	}
	return value.value, nil
}

func (s *MemoryStore) SaveProof(_ context.Context, token string, record ProofRecord, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.cleanupExpired(now)
	s.proofs[token] = memoryValue[ProofRecord]{value: record, expiresAt: now.Add(ttl)}
	return nil
}

func (s *MemoryStore) ConsumeProof(_ context.Context, token string) (ProofRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.proofs[token]
	delete(s.proofs, token)
	if !ok || time.Now().After(value.expiresAt) {
		return ProofRecord{}, ErrStoreNotFound
	}
	return value.value, nil
}

func (s *MemoryStore) cleanupExpired(now time.Time) {
	for id, value := range s.challenges {
		if !now.Before(value.expiresAt) {
			delete(s.challenges, id)
		}
	}
	for token, value := range s.proofs {
		if !now.Before(value.expiresAt) {
			delete(s.proofs, token)
		}
	}
}
