package secureconfig

import "testing"

func TestNewKeyringFromLookup(t *testing.T) {
	values := map[string]string{
		"CMF_CONFIG_SECRET_ACTIVE_VERSION": "v2",
		"CMF_CONFIG_SECRET_KEY_V1":         testKey(1),
		"CMF_CONFIG_SECRET_KEY_V2":         testKey(2),
	}
	keyring, err := NewKeyringFromLookup(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("NewKeyringFromLookup() error = %v", err)
	}
	if got, want := keyring.ActiveVersion(), "v2"; got != want {
		t.Fatalf("ActiveVersion() = %q, want %q", got, want)
	}
	if _, err := keyring.key("v1"); err != nil {
		t.Fatalf("old key version was not loaded: %v", err)
	}
}

func TestNewKeyringFromLookupRequiresActiveKey(t *testing.T) {
	if _, err := NewKeyringFromLookup(func(string) string { return "" }); err == nil {
		t.Fatal("NewKeyringFromLookup() succeeded without active version")
	}
}
