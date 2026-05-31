package plugin

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestSafeUnzipRejectsTraversal(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "bad.zip")
	writeTestZip(t, zipPath, map[string]string{
		"../evil.txt": "owned",
	})
	err := SafeUnzip(zipPath, filepath.Join(t.TempDir(), "plugins"))
	if err == nil {
		t.Fatal("expected traversal zip to fail")
	}
}

func TestSafeUnzipAndReadManifest(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "plugin.zip")
	writeTestZip(t, zipPath, map[string]string{
		"plugin.yaml": validManifestYAML,
		"README.md":   "example",
	})
	target := filepath.Join(root, "install")
	if err := SafeUnzip(zipPath, target); err != nil {
		t.Fatalf("SafeUnzip returned error: %v", err)
	}
	manifest, err := ReadManifestFile(filepath.Join(target, "plugin.yaml"))
	if err != nil {
		t.Fatalf("ReadManifestFile returned error: %v", err)
	}
	if manifest.ID != "admin.example-plugin" {
		t.Fatalf("manifest ID = %q", manifest.ID)
	}
}

func TestIsWithinRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "plugins")
	inside := filepath.Join(root, "admin.example-plugin", "1.0.0")
	outside := filepath.Join(root, "..", "outside")
	if !IsWithinRoot(root, inside) {
		t.Fatal("expected inside path to be within root")
	}
	if IsWithinRoot(root, outside) {
		t.Fatal("expected outside path to be rejected")
	}
}

func writeTestZip(t *testing.T, zipPath string, files map[string]string) {
	t.Helper()
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
}
