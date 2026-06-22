package captcha

import (
	"context"
	"encoding/json"
	"time"
)

const (
	DefaultChallengeTTL = 2 * time.Minute
	DefaultProofTTL     = 90 * time.Second
)

type ScenePolicy struct {
	Enabled      bool
	Provider     string
	ChallengeTTL time.Duration
	ProofTTL     time.Duration
}

type ClientMeta struct {
	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
}

type Challenge struct {
	Payload      json.RawMessage
	PrivateState json.RawMessage
}

type ChallengeResult struct {
	Required    bool            `json:"required"`
	ChallengeID string          `json:"challenge_id,omitempty"`
	Provider    string          `json:"provider,omitempty"`
	ExpiresIn   int64           `json:"expires_in,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

type ProofResult struct {
	Token     string `json:"captcha_token"`
	ExpiresIn int64  `json:"expires_in"`
}

type ChallengeRecord struct {
	Scene        string          `json:"scene"`
	Provider     string          `json:"provider"`
	PrivateState json.RawMessage `json:"private_state"`
}

type ProofRecord struct {
	Scene string `json:"scene"`
}

type Provider interface {
	Name() string
	Create(ctx context.Context, config json.RawMessage, meta ClientMeta) (Challenge, error)
	Verify(ctx context.Context, config, privateState, response json.RawMessage, meta ClientMeta) error
	ValidateConfig(config json.RawMessage) error
}

type PolicySource interface {
	Policy(ctx context.Context, scene string) (ScenePolicy, error)
}

type ProviderConfigSource interface {
	Config(ctx context.Context, provider string) (json.RawMessage, error)
}

type Store interface {
	SaveChallenge(ctx context.Context, id string, record ChallengeRecord, ttl time.Duration) error
	ConsumeChallenge(ctx context.Context, id string) (ChallengeRecord, error)
	SaveProof(ctx context.Context, token string, record ProofRecord, ttl time.Duration) error
	ConsumeProof(ctx context.Context, token string) (ProofRecord, error)
}
