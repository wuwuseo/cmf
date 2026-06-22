package secureconfig

import (
	"fmt"
	"os"
	"strings"
)

const (
	ActiveVersionEnv = "CMF_CONFIG_SECRET_ACTIVE_VERSION"
	keyEnvPrefix     = "CMF_CONFIG_SECRET_KEY_"
)

func NewKeyringFromEnv() (*Keyring, error) {
	return NewKeyringFromLookup(os.Getenv)
}

func NewKeyringFromLookup(lookup func(string) string) (*Keyring, error) {
	if lookup == nil {
		return nil, fmt.Errorf("environment lookup is required")
	}
	active := strings.TrimSpace(lookup(ActiveVersionEnv))
	if active == "" {
		return nil, fmt.Errorf("%s is required", ActiveVersionEnv)
	}
	keys := make(map[string]string)
	for _, version := range candidateVersions(active) {
		if encoded := strings.TrimSpace(lookup(keyEnvName(version))); encoded != "" {
			keys[version] = encoded
		}
	}
	return NewKeyring(active, keys)
}

func candidateVersions(active string) []string {
	versions := []string{active}
	for i := 1; i <= 32; i++ {
		version := fmt.Sprintf("v%d", i)
		if version != active {
			versions = append(versions, version)
		}
	}
	return versions
}

func keyEnvName(version string) string {
	return keyEnvPrefix + strings.ToUpper(strings.ReplaceAll(version, "-", "_"))
}
