package plugin

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

type ProcessRuntime struct {
	mu    sync.Mutex
	procs map[string]*exec.Cmd
}

func NewProcessRuntime() *ProcessRuntime {
	return &ProcessRuntime{procs: make(map[string]*exec.Cmd)}
}

func (r *ProcessRuntime) Start(ctx context.Context, plugin PluginInfo) error {
	if plugin.Manifest == nil {
		return fmt.Errorf("%w: manifest is nil", ErrInvalidManifest)
	}
	entry, err := ResolveProcessEntry(plugin)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, entry)
	cmd.Dir = plugin.InstallPath

	r.mu.Lock()
	defer r.mu.Unlock()
	r.procs[plugin.ID] = cmd
	return nil
}

func (r *ProcessRuntime) Stop(ctx context.Context, pluginID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.procs, pluginID)
	return nil
}

func (r *ProcessRuntime) Health(ctx context.Context, pluginID string) error {
	return nil
}

func ResolveProcessEntry(plugin PluginInfo) (string, error) {
	platform := runtime.GOOS + "-" + runtime.GOARCH
	entry, ok := plugin.Manifest.Engine.Entry[platform]
	if !ok {
		return "", fmt.Errorf("%w: missing entry for %s", ErrInvalidPluginPath, platform)
	}
	if filepath.IsAbs(entry) || pathEscapesRoot(entry) {
		return "", fmt.Errorf("%w: entry escapes root: %s", ErrInvalidPluginPath, entry)
	}
	fullPath := filepath.Join(plugin.InstallPath, entry)
	if !IsWithinRoot(plugin.InstallPath, fullPath) {
		return "", fmt.Errorf("%w: entry outside plugin root: %s", ErrInvalidPluginPath, entry)
	}
	return fullPath, nil
}
