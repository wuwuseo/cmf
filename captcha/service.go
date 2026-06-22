package captcha

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Service struct {
	registry     *Registry
	policies     PolicySource
	configs      ProviderConfigSource
	store        Store
	tokenBytes   int
	challengeTTL time.Duration
	proofTTL     time.Duration
}

func NewService(registry *Registry, policies PolicySource, configs ProviderConfigSource, store Store) *Service {
	return &Service{
		registry:     registry,
		policies:     policies,
		configs:      configs,
		store:        store,
		tokenBytes:   32,
		challengeTTL: DefaultChallengeTTL,
		proofTTL:     DefaultProofTTL,
	}
}

func (s *Service) CreateChallenge(ctx context.Context, scene string, meta ClientMeta) (ChallengeResult, error) {
	policy, err := s.policy(ctx, scene)
	if err != nil {
		return ChallengeResult{}, err
	}
	if !policy.Enabled {
		return ChallengeResult{Required: false}, nil
	}
	provider, config, err := s.provider(ctx, policy.Provider)
	if err != nil {
		return ChallengeResult{}, err
	}
	challenge, err := provider.Create(ctx, config, meta)
	if err != nil {
		return ChallengeResult{}, normalizeProviderError(err)
	}
	id, err := randomToken(s.tokenBytes)
	if err != nil {
		return ChallengeResult{}, fmt.Errorf("generate challenge id: %w", err)
	}
	ttl := durationOr(policy.ChallengeTTL, s.challengeTTL)
	record := ChallengeRecord{Scene: scene, Provider: policy.Provider, PrivateState: challenge.PrivateState}
	if err := s.store.SaveChallenge(ctx, id, record, ttl); err != nil {
		return ChallengeResult{}, fmt.Errorf("save captcha challenge: %w", err)
	}
	return ChallengeResult{
		Required:    true,
		ChallengeID: id,
		Provider:    policy.Provider,
		ExpiresIn:   int64(ttl / time.Second),
		Payload:     challenge.Payload,
	}, nil
}

func (s *Service) VerifyChallenge(
	ctx context.Context,
	challengeID string,
	response json.RawMessage,
	meta ClientMeta,
) (ProofResult, error) {
	if challengeID == "" {
		return ProofResult{}, ErrInvalid
	}
	record, err := s.store.ConsumeChallenge(ctx, challengeID)
	if err != nil {
		return ProofResult{}, ErrInvalid
	}
	policy, err := s.policy(ctx, record.Scene)
	if err != nil || !policy.Enabled || policy.Provider != record.Provider {
		return ProofResult{}, ErrInvalid
	}
	provider, config, err := s.provider(ctx, record.Provider)
	if err != nil {
		return ProofResult{}, err
	}
	if err := provider.Verify(ctx, config, record.PrivateState, response, meta); err != nil {
		return ProofResult{}, normalizeProviderError(err)
	}
	token, err := randomToken(s.tokenBytes)
	if err != nil {
		return ProofResult{}, fmt.Errorf("generate captcha proof: %w", err)
	}
	ttl := durationOr(policy.ProofTTL, s.proofTTL)
	if err := s.store.SaveProof(ctx, token, ProofRecord{Scene: record.Scene}, ttl); err != nil {
		return ProofResult{}, fmt.Errorf("save captcha proof: %w", err)
	}
	return ProofResult{Token: token, ExpiresIn: int64(ttl / time.Second)}, nil
}

func (s *Service) ConsumeProof(ctx context.Context, scene, token string) error {
	policy, err := s.policy(ctx, scene)
	if err != nil {
		return err
	}
	if !policy.Enabled {
		return nil
	}
	if token == "" {
		return ErrRequired
	}
	record, err := s.store.ConsumeProof(ctx, token)
	if err != nil || record.Scene != scene {
		return ErrInvalid
	}
	return nil
}

func (s *Service) ValidateProviderConfig(ctx context.Context, providerName string) error {
	provider, config, err := s.provider(ctx, providerName)
	if err != nil {
		return err
	}
	return provider.ValidateConfig(config)
}

func (s *Service) policy(ctx context.Context, scene string) (ScenePolicy, error) {
	if s == nil || s.policies == nil || s.registry == nil || s.configs == nil || s.store == nil {
		return ScenePolicy{}, ErrProviderMisconfigured
	}
	if scene == "" {
		return ScenePolicy{}, ErrSceneInvalid
	}
	policy, err := s.policies.Policy(ctx, scene)
	if err != nil {
		if errors.Is(err, ErrSceneInvalid) {
			return ScenePolicy{}, ErrSceneInvalid
		}
		return ScenePolicy{}, fmt.Errorf("load captcha policy: %w", err)
	}
	if policy.Enabled && policy.Provider == "" {
		return ScenePolicy{}, ErrProviderMisconfigured
	}
	return policy, nil
}

func (s *Service) provider(ctx context.Context, name string) (Provider, json.RawMessage, error) {
	provider, ok := s.registry.Get(name)
	if !ok {
		return nil, nil, ErrProviderMisconfigured
	}
	config, err := s.configs.Config(ctx, name)
	if err != nil {
		return nil, nil, ErrProviderMisconfigured
	}
	if err := provider.ValidateConfig(config); err != nil {
		return nil, nil, ErrProviderMisconfigured
	}
	return provider, config, nil
}

func durationOr(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func normalizeProviderError(err error) error {
	switch {
	case errors.Is(err, ErrInvalid):
		return ErrInvalid
	case errors.Is(err, ErrProviderMisconfigured):
		return ErrProviderMisconfigured
	default:
		return ErrProviderUnavailable
	}
}
