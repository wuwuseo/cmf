package plugin

import "context"

type PluginRuntime interface {
	Start(ctx context.Context, plugin PluginInfo) error
	Stop(ctx context.Context, pluginID string) error
	Health(ctx context.Context, pluginID string) error
}
