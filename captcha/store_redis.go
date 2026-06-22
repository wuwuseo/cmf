package captcha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisAtomicClient interface {
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	GetDel(ctx context.Context, key string) (string, error)
}

type goRedisAtomicClient struct {
	client *redis.Client
}

func (c goRedisAtomicClient) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c goRedisAtomicClient) GetDel(ctx context.Context, key string) (string, error) {
	return c.client.GetDel(ctx, key).Result()
}

type RedisStore struct {
	client redisAtomicClient
	prefix string
}

func NewRedisStore(client *redis.Client, prefix string) *RedisStore {
	return newRedisStore(goRedisAtomicClient{client: client}, prefix)
}

func newRedisStore(client redisAtomicClient, prefix string) *RedisStore {
	prefix = strings.TrimSuffix(prefix, ":")
	if prefix == "" {
		prefix = "captcha"
	}
	return &RedisStore{client: client, prefix: prefix}
}

func (s *RedisStore) SaveChallenge(ctx context.Context, id string, record ChallengeRecord, ttl time.Duration) error {
	return s.save(ctx, s.key("challenge", id), record, ttl)
}

func (s *RedisStore) ConsumeChallenge(ctx context.Context, id string) (ChallengeRecord, error) {
	var record ChallengeRecord
	if err := s.consume(ctx, s.key("challenge", id), &record); err != nil {
		return ChallengeRecord{}, err
	}
	return record, nil
}

func (s *RedisStore) SaveProof(ctx context.Context, token string, record ProofRecord, ttl time.Duration) error {
	return s.save(ctx, s.key("proof", token), record, ttl)
}

func (s *RedisStore) ConsumeProof(ctx context.Context, token string) (ProofRecord, error) {
	var record ProofRecord
	if err := s.consume(ctx, s.key("proof", token), &record); err != nil {
		return ProofRecord{}, err
	}
	return record, nil
}

func (s *RedisStore) key(kind, id string) string {
	return s.prefix + ":" + kind + ":" + id
}

func (s *RedisStore) save(ctx context.Context, key string, value any, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("redis captcha store is not configured")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal captcha record: %w", err)
	}
	if err := s.client.Set(ctx, key, string(data), ttl); err != nil {
		return fmt.Errorf("store captcha record: %w", err)
	}
	return nil
}

func (s *RedisStore) consume(ctx context.Context, key string, target any) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("redis captcha store is not configured")
	}
	value, err := s.client.GetDel(ctx, key)
	if errors.Is(err, redis.Nil) {
		return ErrStoreNotFound
	}
	if err != nil {
		return fmt.Errorf("consume captcha record: %w", err)
	}
	if err := json.Unmarshal([]byte(value), target); err != nil {
		return fmt.Errorf("decode captcha record: %w", err)
	}
	return nil
}
