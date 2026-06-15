package filesystem

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"strings"
	"unicode"

	"github.com/wuwuseo/cmf/config"
)

type DiskInfo struct {
	Disk   string
	Driver string
}

func NormalizeDirectory(directory string) (string, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return "", nil
	}
	if strings.Contains(directory, `\`) {
		return "", fmt.Errorf("directory must use forward slashes")
	}
	if path.IsAbs(directory) {
		return "", fmt.Errorf("directory must be relative")
	}
	for _, segment := range strings.Split(directory, "/") {
		if segment == ".." {
			return "", fmt.Errorf("directory must not escape storage root")
		}
	}

	cleaned := path.Clean(directory)
	if cleaned == "." {
		return "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("directory must not escape storage root")
	}

	for _, segment := range strings.Split(cleaned, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("directory contains unsafe segment")
		}
		if containsUnsafePathRune(segment) {
			return "", fmt.Errorf("directory contains unsafe character")
		}
	}
	return cleaned, nil
}

func SafeOriginalName(filename string) (string, error) {
	filename = strings.TrimSpace(strings.ReplaceAll(filename, `\`, "/"))
	if filename == "" {
		return "", fmt.Errorf("filename is required")
	}

	base := strings.TrimSpace(path.Base(filename))
	if base == "" || base == "." || base == ".." || containsUnsafePathRune(base) {
		return "", fmt.Errorf("filename is unsafe")
	}
	return base, nil
}

func ContentHashReader(reader io.Reader) (string, int64, error) {
	if reader == nil {
		return "", 0, fmt.Errorf("reader is required")
	}
	h := sha256.New()
	size, err := io.Copy(h, reader)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}

func HashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func BuildContentAddressedKey(directory, contentHash, originalNameHash, extension string) (string, error) {
	if len(contentHash) != 64 || !isLowerHex(contentHash) {
		return "", fmt.Errorf("content hash must be lowercase sha256 hex")
	}
	if len(originalNameHash) < 16 || !isLowerHex(originalNameHash) {
		return "", fmt.Errorf("original name hash must be lowercase hex")
	}

	dir, err := NormalizeDirectory(directory)
	if err != nil {
		return "", err
	}

	extension = strings.TrimSpace(strings.TrimPrefix(extension, "."))
	if extension != "" {
		if strings.ContainsAny(extension, `/\`) || containsUnsafePathRune(extension) {
			return "", fmt.Errorf("extension is unsafe")
		}
		extension = "." + strings.ToLower(extension)
	}

	filename := contentHash + "-" + originalNameHash[:16] + extension
	if dir == "" {
		return filename, nil
	}
	return path.Join(dir, filename), nil
}

func DefaultDiskInfo(cfg config.Config) DiskInfo {
	disk := strings.TrimSpace(cfg.Filesystem.Default)
	if disk == "" {
		disk = "local"
	}

	info := DiskInfo{Disk: disk}
	if cfg.Filesystem.Disks != nil {
		if configured, ok := cfg.Filesystem.Disks[disk]; ok {
			info.Driver = configured.Driver
		}
	}
	return info
}

func containsUnsafePathRune(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) || strings.ContainsRune(`:*?"<>|`, r) {
			return true
		}
	}
	return false
}

func isLowerHex(value string) bool {
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
