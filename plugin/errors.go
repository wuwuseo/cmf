package plugin

import "errors"

var (
	ErrInvalidManifest     = errors.New("invalid plugin manifest")
	ErrInvalidPluginID     = errors.New("invalid plugin id")
	ErrUnsupportedEngine   = errors.New("unsupported plugin engine")
	ErrUnsupportedProtocol = errors.New("unsupported plugin protocol")
	ErrInvalidPluginPath   = errors.New("invalid plugin path")
	ErrPluginNotFound      = errors.New("plugin not found")
	ErrRuntimeNotStarted   = errors.New("plugin runtime not started")
)
