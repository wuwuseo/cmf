package plugin

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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

func TestResolveProcessEntryReturnsAbsolutePath(t *testing.T) {
	pluginDir := filepath.Join(".", "relative-plugin")
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
					runtime.GOOS + "-" + runtime.GOARCH: filepath.Join("bin", "plugin"),
				},
			},
			ExtensionPoints: []string{ExtensionAdminRoutes},
		},
	}

	entry, err := ResolveProcessEntry(info)
	if err != nil {
		t.Fatalf("ResolveProcessEntry returned error: %v", err)
	}
	if !filepath.IsAbs(entry) {
		t.Fatalf("entry = %q, want absolute path", entry)
	}
}

func TestProcessRuntimeStartHealthAndStop(t *testing.T) {
	pluginDir := t.TempDir()
	entry := writeLongRunningPluginCommand(t, pluginDir)
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
					runtime.GOOS + "-" + runtime.GOARCH: entry,
				},
			},
			ExtensionPoints: []string{ExtensionAdminRoutes},
		},
	}

	r := NewProcessRuntime()
	if err := r.Start(context.Background(), info); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if err := r.Health(context.Background(), info.ID); err != nil {
		t.Fatalf("Health returned error for running process: %v", err)
	}
	if err := r.Stop(context.Background(), info.ID); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if err := r.Health(context.Background(), info.ID); !errors.Is(err, ErrRuntimeNotStarted) {
		t.Fatalf("Health err = %v, want ErrRuntimeNotStarted after stop", err)
	}
}

func TestProcessRuntimeHealthFailsWhenProcessExits(t *testing.T) {
	pluginDir := t.TempDir()
	entry := writeDelayedExitPluginCommand(t, pluginDir)
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
					runtime.GOOS + "-" + runtime.GOARCH: entry,
				},
			},
			ExtensionPoints: []string{ExtensionAdminRoutes},
		},
	}

	r := NewProcessRuntime()
	if err := r.Start(context.Background(), info); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := r.Health(context.Background(), info.ID)
		if errors.Is(err, ErrRuntimeNotStarted) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("Health did not report exited process, last err: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestProcessRuntimeStartFailsWhenProcessExitsDuringStartup(t *testing.T) {
	pluginDir := t.TempDir()
	entry := writeExitingPluginCommand(t, pluginDir)
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
					runtime.GOOS + "-" + runtime.GOARCH: entry,
				},
			},
			ExtensionPoints: []string{ExtensionAdminRoutes},
		},
	}

	r := NewProcessRuntime()
	if err := r.Start(context.Background(), info); !errors.Is(err, ErrRuntimeNotStarted) {
		t.Fatalf("Start err = %v, want ErrRuntimeNotStarted", err)
	}
}

func TestProcessRuntimeInjectsUnixSocketEnvironment(t *testing.T) {
	pluginDir := t.TempDir()
	entry := writeEnvCapturePluginCommand(t, pluginDir)
	info := PluginInfo{
		ID:          "admin.example-plugin",
		InstallPath: pluginDir,
		Manifest: &PluginManifest{
			ID:   "admin.example-plugin",
			Name: "Example",
			Engine: PluginEngine{
				Type:     EngineTypeProcess,
				Protocol: ProtocolHTTP,
				Transport: map[string]string{
					runtime.GOOS + "-" + runtime.GOARCH: TransportUnix,
				},
				Socket: map[string]string{
					runtime.GOOS + "-" + runtime.GOARCH: filepath.Join("runtime", "plugin.sock"),
				},
				Entry: map[string]string{
					runtime.GOOS + "-" + runtime.GOARCH: entry,
				},
			},
			ExtensionPoints: []string{ExtensionAdminRoutes},
		},
	}

	r := NewProcessRuntime()
	if err := r.Start(context.Background(), info); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer r.Stop(context.Background(), info.ID)

	envPath := filepath.Join(pluginDir, "env.txt")
	deadline := time.Now().Add(2 * time.Second)
	for {
		data, err := os.ReadFile(envPath)
		if err == nil {
			content := string(data)
			wantSocket := filepath.Join(pluginDir, "runtime", "plugin.sock")
			for _, want := range []string{
				"PLUGIN_ID=admin.example-plugin",
				"PLUGIN_PROTOCOL=http",
				"PLUGIN_TRANSPORT=unix",
				"PLUGIN_SOCKET=" + wantSocket,
			} {
				if !strings.Contains(content, want) {
					t.Fatalf("env file missing %q in %q", want, content)
				}
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("env file was not written: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func writeLongRunningPluginCommand(t *testing.T, dir string) string {
	t.Helper()
	return writePluginCommand(t, dir, "long-running", "time.Sleep(30*time.Second)")
}

func writeExitingPluginCommand(t *testing.T, dir string) string {
	t.Helper()
	return writePluginCommand(t, dir, "exiting", "")
}

func writeDelayedExitPluginCommand(t *testing.T, dir string) string {
	t.Helper()
	return writePluginCommand(t, dir, "delayed-exit", "time.Sleep(500*time.Millisecond)")
}

func writeEnvCapturePluginCommand(t *testing.T, dir string) string {
	t.Helper()
	return writePluginCommandWithImports(t, dir, "env-capture", []string{"os", "strings", "time"}, `
		keys := []string{"PLUGIN_ID", "PLUGIN_PROTOCOL", "PLUGIN_TRANSPORT", "PLUGIN_SOCKET", "PLUGIN_ADDRESS"}
		lines := make([]string, 0, len(keys))
		for _, key := range keys {
			lines = append(lines, key+"="+os.Getenv(key))
		}
		if err := os.WriteFile("env.txt", []byte(strings.Join(lines, "\n")), 0600); err != nil {
			panic(err)
		}
		time.Sleep(30*time.Second)
	`)
}

func writePluginCommand(t *testing.T, dir string, name string, body string) string {
	t.Helper()
	imports := []string(nil)
	if body != "" {
		imports = []string{"time"}
	}
	return writePluginCommandWithImports(t, dir, name, imports, body)
}

func writePluginCommandWithImports(t *testing.T, dir string, name string, imports []string, body string) string {
	t.Helper()
	src := filepath.Join(dir, name+".go")
	exeName := name
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}
	exe := filepath.Join(dir, exeName)
	importBlock := ""
	if len(imports) == 1 {
		importBlock = "\nimport \"" + imports[0] + "\"\n"
	}
	if len(imports) > 1 {
		quoted := make([]string, 0, len(imports))
		for _, item := range imports {
			quoted = append(quoted, "\t\""+item+"\"")
		}
		importBlock = "\nimport (\n" + strings.Join(quoted, "\n") + "\n)\n"
	}
	code := []byte("package main\n" + importBlock + "\nfunc main() { " + body + " }\n")
	if err := os.WriteFile(src, code, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", exe, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build plugin command: %v\n%s", err, string(out))
	}
	return exeName
}
