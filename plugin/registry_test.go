package plugin

import "testing"

func TestMemoryExtensionRegistryRegisterListUnregister(t *testing.T) {
	registry := NewMemoryExtensionRegistry()
	if err := registry.Register("admin.example-plugin", ExtensionAdminRoutes, map[string]string{"path": "/hello"}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if err := registry.Register("admin.other-plugin", ExtensionAdminRoutes, "payload"); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	items := registry.List(ExtensionAdminRoutes)
	if len(items) != 2 {
		t.Fatalf("List returned %d items, want 2", len(items))
	}
	if err := registry.UnregisterPlugin("admin.example-plugin"); err != nil {
		t.Fatalf("UnregisterPlugin returned error: %v", err)
	}
	items = registry.List(ExtensionAdminRoutes)
	if len(items) != 1 || items[0].PluginID != "admin.other-plugin" {
		t.Fatalf("remaining items = %#v", items)
	}
}
