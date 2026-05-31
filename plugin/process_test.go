package plugin

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
)

func TestProcessRuntimeRejectsEntryOutsideInstallPath(t *testing.T) {
	pluginDir := t.TempDir()
	info := PluginInfo{
		ID:          "admin.example-plugin",
		InstallPath: pluginDir,
		Manifest: &PluginManifest{
			ID:   "admin.example-plugin",
			Name: "Example",
			Engine: PluginEngine{
				Type:     EngineTypeProcess,
				Protocol: ProtocolHTTP,
				Entry: map[string]string{
					runtime.GOOS + "-" + runtime.GOARCH: filepath.Join("..", "outside"),
				},
			},
			ExtensionPoints: []string{ExtensionAdminRoutes},
		},
	}
	r := NewProcessRuntime()
	if err := r.Start(context.Background(), info); err == nil {
		t.Fatal("expected Start to reject escaping entry")
	}
}
