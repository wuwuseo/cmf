package captcha

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type staticPolicySource map[string]ScenePolicy

func (s staticPolicySource) Policy(_ context.Context, scene string) (ScenePolicy, error) {
	policy, ok := s[scene]
	if !ok {
		return ScenePolicy{}, ErrSceneInvalid
	}
	return policy, nil
}

type staticConfigSource map[string]json.RawMessage

func (s staticConfigSource) Config(_ context.Context, provider string) (json.RawMessage, error) {
	config, ok := s[provider]
	if !ok {
		return nil, ErrProviderMisconfigured
	}
	return config, nil
}

type testProvider struct {
	createCalls atomic.Int32
	verifyCalls atomic.Int32
	verifyErr   error
}

func (p *testProvider) Name() string { return "test" }

func (p *testProvider) Create(_ context.Context, _ json.RawMessage, _ ClientMeta) (Challenge, error) {
	p.createCalls.Add(1)
	return Challenge{
		Payload:      json.RawMessage(`{"question":"answer me"}`),
		PrivateState: json.RawMessage(`{"answer":"ok"}`),
	}, nil
}

func (p *testProvider) Verify(_ context.Context, _, _, _ json.RawMessage, _ ClientMeta) error {
	p.verifyCalls.Add(1)
	return p.verifyErr
}

func (p *testProvider) ValidateConfig(json.RawMessage) error { return nil }

func newTestService(t *testing.T, provider *testProvider, policies staticPolicySource) *Service {
	t.Helper()
	registry := NewRegistry()
	if err := registry.Register(provider); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	return NewService(
		registry,
		policies,
		staticConfigSource{"test": json.RawMessage(`{"enabled":true}`)},
		NewMemoryStore(),
	)
}

func TestServiceCreateChallengeDisabledScene(t *testing.T) {
	provider := &testProvider{}
	service := newTestService(t, provider, staticPolicySource{
		"public_form": {Enabled: false},
	})

	result, err := service.CreateChallenge(t.Context(), "public_form", ClientMeta{})
	if err != nil {
		t.Fatalf("CreateChallenge() error = %v", err)
	}
	if result.Required {
		t.Fatal("CreateChallenge() Required = true, want false")
	}
	if provider.createCalls.Load() != 0 {
		t.Fatal("disabled scene called provider")
	}
}

func TestServiceChallengeVerifyAndConsumeProof(t *testing.T) {
	provider := &testProvider{}
	service := newTestService(t, provider, staticPolicySource{
		"admin_login": {
			Enabled:      true,
			Provider:     "test",
			ChallengeTTL: 2 * time.Minute,
			ProofTTL:     time.Minute,
		},
	})

	challenge, err := service.CreateChallenge(t.Context(), "admin_login", ClientMeta{IP: "127.0.0.1"})
	if err != nil {
		t.Fatalf("CreateChallenge() error = %v", err)
	}
	if !challenge.Required || challenge.ChallengeID == "" || challenge.Provider != "test" {
		t.Fatalf("CreateChallenge() = %+v", challenge)
	}

	proof, err := service.VerifyChallenge(
		t.Context(),
		challenge.ChallengeID,
		json.RawMessage(`{"answer":"ok"}`),
		ClientMeta{IP: "127.0.0.1"},
	)
	if err != nil {
		t.Fatalf("VerifyChallenge() error = %v", err)
	}
	if proof.Token == "" {
		t.Fatal("VerifyChallenge() returned empty proof token")
	}

	if err := service.ConsumeProof(t.Context(), "admin_login", proof.Token); err != nil {
		t.Fatalf("ConsumeProof() error = %v", err)
	}
	if err := service.ConsumeProof(t.Context(), "admin_login", proof.Token); !errors.Is(err, ErrInvalid) {
		t.Fatalf("second ConsumeProof() error = %v, want ErrInvalid", err)
	}
}

func TestServiceFailedVerificationConsumesChallenge(t *testing.T) {
	provider := &testProvider{verifyErr: ErrInvalid}
	service := newTestService(t, provider, staticPolicySource{
		"admin_login": {
			Enabled:      true,
			Provider:     "test",
			ChallengeTTL: time.Minute,
			ProofTTL:     time.Minute,
		},
	})
	challenge, err := service.CreateChallenge(t.Context(), "admin_login", ClientMeta{})
	if err != nil {
		t.Fatalf("CreateChallenge() error = %v", err)
	}

	if _, err := service.VerifyChallenge(t.Context(), challenge.ChallengeID, nil, ClientMeta{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("first VerifyChallenge() error = %v, want ErrInvalid", err)
	}
	provider.verifyErr = nil
	if _, err := service.VerifyChallenge(t.Context(), challenge.ChallengeID, nil, ClientMeta{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("second VerifyChallenge() error = %v, want ErrInvalid", err)
	}
	if provider.verifyCalls.Load() != 1 {
		t.Fatalf("Verify() calls = %d, want 1", provider.verifyCalls.Load())
	}
}

func TestServiceRejectsProofForDifferentScene(t *testing.T) {
	provider := &testProvider{}
	service := newTestService(t, provider, staticPolicySource{
		"user_login":  {Enabled: true, Provider: "test", ChallengeTTL: time.Minute, ProofTTL: time.Minute},
		"admin_login": {Enabled: true, Provider: "test", ChallengeTTL: time.Minute, ProofTTL: time.Minute},
	})
	challenge, err := service.CreateChallenge(t.Context(), "user_login", ClientMeta{})
	if err != nil {
		t.Fatalf("CreateChallenge() error = %v", err)
	}
	proof, err := service.VerifyChallenge(t.Context(), challenge.ChallengeID, nil, ClientMeta{})
	if err != nil {
		t.Fatalf("VerifyChallenge() error = %v", err)
	}

	if err := service.ConsumeProof(t.Context(), "admin_login", proof.Token); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ConsumeProof(other scene) error = %v, want ErrInvalid", err)
	}
}
