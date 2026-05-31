package plugin

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const ManifestFileName = "plugin.yaml"

func ReadManifestFile(path string) (*PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: read manifest: %v", ErrInvalidManifest, err)
	}
	return ParseManifest(data)
}

func ReadManifestFromDir(pluginDir string) (*PluginManifest, error) {
	return ReadManifestFile(filepath.Join(pluginDir, ManifestFileName))
}

func SafeUnzip(zipPath string, targetDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("%w: open zip: %v", ErrInvalidPluginPath, err)
	}
	defer reader.Close()

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	for _, file := range reader.File {
		targetPath := filepath.Join(targetDir, file.Name)
		if !IsWithinRoot(targetDir, targetPath) {
			return fmt.Errorf("%w: zip entry escapes root: %s", ErrInvalidPluginPath, file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, file.Mode()); err != nil {
				return fmt.Errorf("create zip dir %s: %w", targetPath, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("create parent dir %s: %w", targetPath, err)
		}
		if err := extractZipFile(file, targetPath); err != nil {
			return err
		}
	}
	return nil
}

func IsWithinRoot(root string, target string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !pathEscapesRoot(rel))
}

func extractZipFile(file *zip.File, targetPath string) error {
	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("open zip entry %s: %w", file.Name, err)
	}
	defer src.Close()

	dst, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return fmt.Errorf("create file %s: %w", targetPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("extract file %s: %w", targetPath, err)
	}
	return nil
}
