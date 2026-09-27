package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	root := t.TempDir()
	apps := filepath.Join(root, "internal", "apps")
	schema := filepath.Join(apps, "demo", "schema")
	outDir := filepath.Join(root, "internal", "ent", "schema")
	for _, dir := range []string{schema, outDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		filepath.Join(root, "go.mod"):         "module example.com/test\n",
		filepath.Join(apps, "demo", "app.go"): "package demo\n",
		filepath.Join(schema, "item.go"):      "package schema\nimport \"entgo.io/ent\"\ntype DemoItem struct { ent.Schema }\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(outDir, "zz_apps_gen.go")
	if err := generate(apps, out, "schema"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(content), "type DemoItem struct{ appschemademo.DemoItem }") {
		t.Fatalf("generated wrapper: %s, %v", content, err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "other.go"), []byte("package schema\ntype DemoItem struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := generate(apps, out, "schema"); err == nil || !strings.Contains(err.Error(), "collides") {
		t.Fatalf("expected collision, got %v", err)
	}
}
