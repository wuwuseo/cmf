package captcha

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

type namedProvider string

func (p namedProvider) Name() string { return string(p) }
func (namedProvider) Create(context.Context, json.RawMessage, ClientMeta) (Challenge, error) {
	return Challenge{}, nil
}
func (namedProvider) Verify(
	context.Context,
	json.RawMessage,
	json.RawMessage,
	json.RawMessage,
	ClientMeta,
) error {
	return nil
}
func (namedProvider) ValidateConfig(json.RawMessage) error { return nil }

func TestRegistryNamesReturnsSortedProviders(t *testing.T) {
	registry := NewRegistry()
	for _, name := range []string{"zeta", "alpha", "middle"} {
		if err := registry.Register(namedProvider(name)); err != nil {
			t.Fatalf("Register(%s) error = %v", name, err)
		}
	}
	if got, want := registry.Names(), []string{"alpha", "middle", "zeta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() = %v, want %v", got, want)
	}
}
