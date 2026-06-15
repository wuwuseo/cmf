package filesystem_test

import (
	"strings"
	"testing"

	"github.com/wuwuseo/cmf/config"
	"github.com/wuwuseo/cmf/filesystem"
)

func TestNormalizeDirectoryAcceptsSafeRelativePath(t *testing.T) {
	got, err := filesystem.NormalizeDirectory(" products/covers/2026 ")
	if err != nil {
		t.Fatalf("NormalizeDirectory returned error: %v", err)
	}
	if got != "products/covers/2026" {
		t.Fatalf("expected cleaned directory, got %q", got)
	}
}

func TestNormalizeDirectoryRejectsUnsafePath(t *testing.T) {
	cases := []string{
		"../secret",
		"/absolute",
		`products\covers`,
		"products/../secret",
	}
	for _, tc := range cases {
		if _, err := filesystem.NormalizeDirectory(tc); err == nil {
			t.Fatalf("expected %q to be rejected", tc)
		}
	}
}

func TestSafeOriginalNameUsesBaseName(t *testing.T) {
	got, err := filesystem.SafeOriginalName(`C:\fakepath\avatar.png`)
	if err != nil {
		t.Fatalf("SafeOriginalName returned error: %v", err)
	}
	if got != "avatar.png" {
		t.Fatalf("expected avatar.png, got %q", got)
	}
}

func TestContentHashReaderReturnsHashAndSize(t *testing.T) {
	gotHash, gotSize, err := filesystem.ContentHashReader(strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("ContentHashReader returned error: %v", err)
	}
	const wantHash = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if gotHash != wantHash {
		t.Fatalf("expected hash %s, got %s", wantHash, gotHash)
	}
	if gotSize != 5 {
		t.Fatalf("expected size 5, got %d", gotSize)
	}
}

func TestBuildContentAddressedKey(t *testing.T) {
	nameHash := filesystem.HashString("avatar.png")
	got, err := filesystem.BuildContentAddressedKey("avatars", strings.Repeat("a", 64), nameHash, "png")
	if err != nil {
		t.Fatalf("BuildContentAddressedKey returned error: %v", err)
	}
	want := "avatars/" + strings.Repeat("a", 64) + "-" + nameHash[:16] + ".png"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestDefaultDiskInfo(t *testing.T) {
	cfg := config.Config{}
	cfg.Filesystem.Default = "s3"
	cfg.Filesystem.Disks = map[string]struct {
		Driver  string `mapstructure:"driver"`
		Options any    `mapstructure:"options"`
	}{
		"s3": {Driver: "s3"},
	}

	got := filesystem.DefaultDiskInfo(cfg)
	if got.Disk != "s3" || got.Driver != "s3" {
		t.Fatalf("expected s3 disk info, got %+v", got)
	}
}
