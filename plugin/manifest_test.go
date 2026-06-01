package plugin

import (
	"errors"
	"strings"
	"testing"
)

const validManifestYAML = `
id: admin.example-plugin
name: Example Plugin
version: 1.0.0
author: example
description: Example plugin
engine:
  type: process
  protocol: http
  entry:
    windows-amd64: bin/windows-amd64/plugin.exe
    linux-amd64: bin/linux-amd64/plugin
compatibility:
  app: ">=1.0.0 <2.0.0"
  api: "v1"
permissions:
  - route:register
extension_points:
  - admin.routes
`

func TestParseManifestSuccess(t *testing.T) {
	manifest, err := ParseManifest([]byte(validManifestYAML))
	if err != nil {
		t.Fatalf("ParseManifest returned error: %v", err)
	}
	if manifest.ID != "admin.example-plugin" {
		t.Fatalf("ID = %q, want admin.example-plugin", manifest.ID)
	}
	if manifest.Engine.Entry["linux-amd64"] != "bin/linux-amd64/plugin" {
		t.Fatalf("linux entry = %q", manifest.Engine.Entry["linux-amd64"])
	}
}

func TestValidateManifestMissingRequired(t *testing.T) {
	manifest, err := ParseManifest([]byte(strings.Replace(validManifestYAML, "name: Example Plugin\n", "", 1)))
	if err == nil {
		err = manifest.Validate()
	}
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestValidateManifestInvalidID(t *testing.T) {
	manifest, err := ParseManifest([]byte(strings.Replace(validManifestYAML, "admin.example-plugin", "admin/example", 1)))
	if err == nil {
		err = manifest.Validate()
	}
	if err == nil {
		t.Fatal("expected error for invalid plugin id")
	}
}

func TestValidateManifestRejectsNonProcessEngine(t *testing.T) {
	manifest, err := ParseManifest([]byte(strings.Replace(validManifestYAML, "type: process", "type: native", 1)))
	if err == nil {
		err = manifest.Validate()
	}
	if err == nil {
		t.Fatal("expected error for unsupported engine type")
	}
}

func TestValidateManifestRejectsTraversalEntry(t *testing.T) {
	manifest, err := ParseManifest([]byte(strings.Replace(validManifestYAML, "bin/linux-amd64/plugin", "../plugin", 1)))
	if err == nil {
		err = manifest.Validate()
	}
	if err == nil {
		t.Fatal("expected error for path traversal entry")
	}
}

func TestParseManifestSupportsUnixTransportSocket(t *testing.T) {
	data := strings.Replace(validManifestYAML, "  entry:\n", "  transport:\n    linux-amd64: unix\n  socket:\n    linux-amd64: runtime/plugin.sock\n  entry:\n", 1)
	manifest, err := ParseManifest([]byte(data))
	if err != nil {
		t.Fatalf("ParseManifest returned error: %v", err)
	}
	if manifest.Engine.Transport["linux-amd64"] != TransportUnix {
		t.Fatalf("transport = %q, want unix", manifest.Engine.Transport["linux-amd64"])
	}
	if manifest.Engine.Socket["linux-amd64"] != "runtime/plugin.sock" {
		t.Fatalf("socket = %q, want runtime/plugin.sock", manifest.Engine.Socket["linux-amd64"])
	}
}

func TestValidateManifestRejectsTraversalSocket(t *testing.T) {
	data := strings.Replace(validManifestYAML, "  entry:\n", "  transport:\n    linux-amd64: unix\n  socket:\n    linux-amd64: ../plugin.sock\n  entry:\n", 1)
	_, err := ParseManifest([]byte(data))
	if !errors.Is(err, ErrInvalidPluginPath) {
		t.Fatalf("ParseManifest err = %v, want ErrInvalidPluginPath", err)
	}
}

func TestValidateManifestRejectsUnsupportedTransport(t *testing.T) {
	data := strings.Replace(validManifestYAML, "  entry:\n", "  transport:\n    linux-amd64: named-pipe\n  entry:\n", 1)
	_, err := ParseManifest([]byte(data))
	if !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("ParseManifest err = %v, want ErrInvalidManifest", err)
	}
}
