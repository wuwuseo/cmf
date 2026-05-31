package plugin

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

var pluginIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type PluginManifest struct {
	ID              string              `yaml:"id" json:"id"`
	Name            string              `yaml:"name" json:"name"`
	Version         string              `yaml:"version" json:"version"`
	Author          string              `yaml:"author" json:"author"`
	Description     string              `yaml:"description" json:"description"`
	Engine          PluginEngine        `yaml:"engine" json:"engine"`
	Compatibility   PluginCompatibility `yaml:"compatibility" json:"compatibility"`
	Permissions     []string            `yaml:"permissions" json:"permissions"`
	ExtensionPoints []string            `yaml:"extension_points" json:"extension_points"`
}

type PluginEngine struct {
	Type     string            `yaml:"type" json:"type"`
	Protocol string            `yaml:"protocol" json:"protocol"`
	Entry    map[string]string `yaml:"entry" json:"entry"`
}

type PluginCompatibility struct {
	App string `yaml:"app" json:"app"`
	API string `yaml:"api" json:"api"`
}

func ParseManifest(data []byte) (*PluginManifest, error) {
	var manifest PluginManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidManifest, err)
	}
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (m *PluginManifest) Validate() error {
	if strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidManifest)
	}
	if !pluginIDPattern.MatchString(m.ID) {
		return fmt.Errorf("%w: %s", ErrInvalidPluginID, m.ID)
	}
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidManifest)
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("%w: version is required", ErrInvalidManifest)
	}
	if strings.TrimSpace(m.Engine.Type) == "" {
		return fmt.Errorf("%w: engine.type is required", ErrInvalidManifest)
	}
	if m.Engine.Type != EngineTypeProcess {
		return fmt.Errorf("%w: %s", ErrUnsupportedEngine, m.Engine.Type)
	}
	if !isAllowedProtocol(m.Engine.Protocol) {
		return fmt.Errorf("%w: %s", ErrUnsupportedProtocol, m.Engine.Protocol)
	}
	if len(m.ExtensionPoints) == 0 {
		return fmt.Errorf("%w: extension_points is required", ErrInvalidManifest)
	}
	for platform, entry := range m.Engine.Entry {
		if strings.TrimSpace(platform) == "" || strings.TrimSpace(entry) == "" {
			return fmt.Errorf("%w: empty engine entry", ErrInvalidManifest)
		}
		if filepath.IsAbs(entry) || pathEscapesRoot(entry) {
			return fmt.Errorf("%w: entry %s escapes plugin root", ErrInvalidPluginPath, entry)
		}
	}
	return nil
}

func isAllowedProtocol(protocol string) bool {
	switch protocol {
	case ProtocolHTTP, ProtocolGRPC, ProtocolStdio:
		return true
	default:
		return false
	}
}

func pathEscapesRoot(path string) bool {
	clean := filepath.Clean(path)
	return clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || strings.HasPrefix(filepath.ToSlash(clean), "../")
}
