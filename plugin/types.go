package plugin

const (
	EngineTypeProcess = "process"

	ProtocolHTTP  = "http"
	ProtocolGRPC  = "grpc"
	ProtocolStdio = "stdio"

	ExtensionAdminRoutes      = "admin.routes"
	ExtensionAdminMenus       = "admin.menus"
	ExtensionAdminPermissions = "admin.permissions"
	ExtensionCronTasks        = "cron.tasks"
	ExtensionMessageHandlers  = "message.handlers"
)

type PluginStatus string

const (
	PluginStatusInstalled PluginStatus = "installed"
	PluginStatusEnabled   PluginStatus = "enabled"
	PluginStatusDisabled  PluginStatus = "disabled"
	PluginStatusError     PluginStatus = "error"
)

type PluginInfo struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Author      string          `json:"author"`
	Description string          `json:"description"`
	Status      PluginStatus    `json:"status"`
	InstallPath string          `json:"install_path"`
	Manifest    *PluginManifest `json:"manifest,omitempty"`
}

type Extension struct {
	PluginID string `json:"plugin_id"`
	Point    string `json:"point"`
	Payload  any    `json:"payload"`
}
