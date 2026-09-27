package genutil

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
)

// ModuleImport returns the import path of dir by finding its containing go.mod.
func ModuleImport(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for root := abs; ; root = filepath.Dir(root) {
		content, err := os.ReadFile(filepath.Join(root, "go.mod"))
		if err == nil {
			module := ""
			for _, line := range strings.Split(string(content), "\n") {
				if fields := strings.Fields(line); len(fields) == 2 && fields[0] == "module" {
					module = strings.Trim(fields[1], `"`)
					break
				}
			}
			if module == "" {
				return "", fmt.Errorf("no module directive in %s", filepath.Join(root, "go.mod"))
			}
			rel, err := filepath.Rel(root, abs)
			if err != nil {
				return "", err
			}
			if rel == "." {
				return module, nil
			}
			return module + "/" + filepath.ToSlash(rel), nil
		}
		parent := filepath.Dir(root)
		if parent == root {
			return "", fmt.Errorf("go.mod not found above %s", abs)
		}
	}
}

// WriteGo formats generated source and leaves the file untouched when unchanged.
func WriteGo(out string, source []byte) error {
	formatted, err := format.Source(source)
	if err != nil {
		return fmt.Errorf("format generated %s: %w", out, err)
	}
	if current, err := os.ReadFile(out); err == nil && bytes.Equal(current, formatted) {
		return nil
	}
	return os.WriteFile(out, formatted, 0o644)
}
