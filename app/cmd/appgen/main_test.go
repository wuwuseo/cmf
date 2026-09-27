package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	root := t.TempDir()
	apps := filepath.Join(root, "internal", "apps")
	if err := os.MkdirAll(filepath.Join(apps, "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(root, "go.mod"):         "module example.com/test\n",
		filepath.Join(apps, "demo", "app.go"): "package demo\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(apps, "zz_generated.go")
	if err := generate(apps, out, "apps"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(content), `_ "example.com/test/internal/apps/demo"`) {
		t.Fatalf("generated imports: %s, %v", content, err)
	}
	before, _ := os.Stat(out)
	time.Sleep(20 * time.Millisecond)
	if err := generate(apps, out, "apps"); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(out)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("unchanged output was rewritten")
	}
}
