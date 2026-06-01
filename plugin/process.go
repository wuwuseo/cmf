package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const processStopTimeout = 5 * time.Second
const processStartupGrace = 200 * time.Millisecond

type ProcessRuntime struct {
	mu    sync.Mutex
	procs map[string]*runningProcess
}

type runningProcess struct {
	cmd  *exec.Cmd
	done chan error
}

func NewProcessRuntime() *ProcessRuntime {
	return &ProcessRuntime{procs: make(map[string]*runningProcess)}
}

func (r *ProcessRuntime) Start(ctx context.Context, plugin PluginInfo) error {
	if plugin.Manifest == nil {
		return fmt.Errorf("%w: manifest is nil", ErrInvalidManifest)
	}
	entry, err := ResolveProcessEntry(plugin)
	if err != nil {
		return err
	}

	r.mu.Lock()
	if proc, ok := r.procs[plugin.ID]; ok {
		if processRunning(proc) {
			r.mu.Unlock()
			return nil
		}
		delete(r.procs, plugin.ID)
	}
	r.mu.Unlock()

	runtimeEnv, err := ResolveProcessEnvironment(plugin)
	if err != nil {
		return err
	}
	cmd := exec.Command(entry)
	cmd.Dir = plugin.InstallPath
	cmd.Env = append(os.Environ(), runtimeEnv...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	proc := &runningProcess{
		cmd:  cmd,
		done: make(chan error, 1),
	}
	go func() {
		proc.done <- cmd.Wait()
	}()
	startupTimer := time.NewTimer(processStartupGrace)
	defer startupTimer.Stop()
	select {
	case err := <-proc.done:
		if err != nil {
			return fmt.Errorf("%w: process exited during startup: %v", ErrRuntimeNotStarted, err)
		}
		return fmt.Errorf("%w: process exited during startup", ErrRuntimeNotStarted)
	case <-ctx.Done():
		_ = stopProcess(context.Background(), proc)
		return ctx.Err()
	case <-startupTimer.C:
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.procs[plugin.ID]; ok && processRunning(existing) {
		r.mu.Unlock()
		_ = stopProcess(context.Background(), proc)
		r.mu.Lock()
		return nil
	}
	r.procs[plugin.ID] = proc
	return nil
}

func (r *ProcessRuntime) Stop(ctx context.Context, pluginID string) error {
	r.mu.Lock()
	proc, ok := r.procs[pluginID]
	if !ok {
		r.mu.Unlock()
		return nil
	}
	delete(r.procs, pluginID)
	r.mu.Unlock()

	return stopProcess(ctx, proc)
}

func (r *ProcessRuntime) Health(ctx context.Context, pluginID string) error {
	r.mu.Lock()
	proc, ok := r.procs[pluginID]
	if !ok {
		r.mu.Unlock()
		return ErrRuntimeNotStarted
	}
	if processRunning(proc) {
		r.mu.Unlock()
		return nil
	}
	delete(r.procs, pluginID)
	r.mu.Unlock()
	return ErrRuntimeNotStarted
}

func ResolveProcessEntry(plugin PluginInfo) (string, error) {
	installPath, err := filepath.Abs(plugin.InstallPath)
	if err != nil {
		return "", fmt.Errorf("%w: invalid install path: %v", ErrInvalidPluginPath, err)
	}
	platform := currentPlatform()
	entry, ok := plugin.Manifest.Engine.Entry[platform]
	if !ok {
		return "", fmt.Errorf("%w: missing entry for %s", ErrInvalidPluginPath, platform)
	}
	if filepath.IsAbs(entry) || pathEscapesRoot(entry) {
		return "", fmt.Errorf("%w: entry escapes root: %s", ErrInvalidPluginPath, entry)
	}
	fullPath := filepath.Join(installPath, entry)
	if !IsWithinRoot(installPath, fullPath) {
		return "", fmt.Errorf("%w: entry outside plugin root: %s", ErrInvalidPluginPath, entry)
	}
	return fullPath, nil
}

func ResolveProcessEnvironment(plugin PluginInfo) ([]string, error) {
	if plugin.Manifest == nil {
		return nil, fmt.Errorf("%w: manifest is nil", ErrInvalidManifest)
	}
	installPath, err := filepath.Abs(plugin.InstallPath)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid install path: %v", ErrInvalidPluginPath, err)
	}
	platform := currentPlatform()
	transport := resolvePlatformValue(plugin.Manifest.Engine.Transport, platform, defaultTransport(plugin.Manifest.Engine.Protocol))
	address := resolvePlatformValue(plugin.Manifest.Engine.Address, platform, "")
	socket := resolvePlatformValue(plugin.Manifest.Engine.Socket, platform, "")
	if transport == TransportUnix {
		if socket == "" {
			return nil, fmt.Errorf("%w: missing socket for %s", ErrInvalidManifest, platform)
		}
		if filepath.IsAbs(socket) || pathEscapesRoot(socket) {
			return nil, fmt.Errorf("%w: socket escapes root: %s", ErrInvalidPluginPath, socket)
		}
		socket = filepath.Join(installPath, socket)
		if !IsWithinRoot(installPath, socket) {
			return nil, fmt.Errorf("%w: socket outside plugin root: %s", ErrInvalidPluginPath, socket)
		}
		if err := os.MkdirAll(filepath.Dir(socket), 0700); err != nil {
			return nil, fmt.Errorf("%w: create socket dir: %v", ErrInvalidPluginPath, err)
		}
	}
	return []string{
		"PLUGIN_ID=" + plugin.ID,
		"PLUGIN_PROTOCOL=" + plugin.Manifest.Engine.Protocol,
		"PLUGIN_TRANSPORT=" + transport,
		"PLUGIN_ADDRESS=" + address,
		"PLUGIN_SOCKET=" + socket,
	}, nil
}

func currentPlatform() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

func defaultTransport(protocol string) string {
	if protocol == ProtocolStdio {
		return TransportStdio
	}
	return TransportTCP
}

func resolvePlatformValue(values map[string]string, platform string, fallback string) string {
	if values == nil {
		return fallback
	}
	if value, ok := values[platform]; ok {
		return value
	}
	return fallback
}

func processRunning(proc *runningProcess) bool {
	select {
	case <-proc.done:
		return false
	default:
		return true
	}
}

func stopProcess(ctx context.Context, proc *runningProcess) error {
	if proc == nil || proc.cmd == nil || proc.cmd.Process == nil {
		return nil
	}
	select {
	case <-proc.done:
		return nil
	default:
	}
	if err := proc.cmd.Process.Kill(); err != nil {
		select {
		case <-proc.done:
			return nil
		default:
			return err
		}
	}
	timeout := time.NewTimer(processStopTimeout)
	defer timeout.Stop()
	select {
	case <-proc.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timeout.C:
		return fmt.Errorf("stop plugin process: timeout")
	}
}
