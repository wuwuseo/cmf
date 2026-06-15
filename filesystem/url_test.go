package filesystem_test

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	cmfconfig "github.com/wuwuseo/cmf/config"
	"github.com/wuwuseo/cmf/filesystem"
)

func accessURLTestConfig() cmfconfig.Config {
	var cfg cmfconfig.Config
	cfg.Attachment.PublicBaseURL = "/uploads"
	cfg.Attachment.XAccelEnabled = true
	cfg.Attachment.XAccelPrefix = "/_protected_attachments"
	cfg.Attachment.AccessURLTTL = 600
	cfg.Filesystem.Default = "local"
	cfg.Filesystem.Disks = map[string]struct {
		Driver  string `mapstructure:"driver"`
		Options any    `mapstructure:"options"`
	}{
		"local": {
			Driver: "local",
			Options: map[string]any{
				"root":            "./data/storage",
				"public_base_url": "https://local-cdn.example.com/uploads",
			},
		},
		"s3": {
			Driver: "s3",
			Options: map[string]any{
				"access_key":      "AKIA_TEST",
				"secret_key":      "secret",
				"region":          "us-east-1",
				"bucket":          "bucket",
				"endpoint":        "https://s3.example.com",
				"public_base_url": "https://s3-cdn.example.com",
				"presign_expires": 600,
				"use_path_style":  true,
			},
		},
	}
	return cfg
}

func TestIsPublicDirectory(t *testing.T) {
	for _, tc := range []struct {
		name      string
		directory string
		want      bool
	}{
		{name: "public root", directory: "public", want: true},
		{name: "public child", directory: "public/avatar", want: true},
		{name: "private default", directory: "attachments/2026/06", want: false},
		{name: "publicish is private", directory: "publicity/avatar", want: false},
		{name: "empty is private", directory: "", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := filesystem.IsPublicDirectory(tc.directory); got != tc.want {
				t.Fatalf("IsPublicDirectory(%q) = %v, want %v", tc.directory, got, tc.want)
			}
		})
	}
}

func TestLocalPublicURLPrefersDiskBaseURLAndStripsPublicPrefix(t *testing.T) {
	cfg := accessURLTestConfig()

	got, err := filesystem.LocalPublicURL(cfg, "local", "public/avatar/image.png")
	if err != nil {
		t.Fatalf("LocalPublicURL returned error: %v", err)
	}

	want := "https://local-cdn.example.com/uploads/avatar/image.png"
	if got != want {
		t.Fatalf("LocalPublicURL = %q, want %q", got, want)
	}
}

func TestLocalPublicURLFallsBackToAttachmentBaseURL(t *testing.T) {
	cfg := accessURLTestConfig()
	cfg.Filesystem.Disks["local"] = struct {
		Driver  string `mapstructure:"driver"`
		Options any    `mapstructure:"options"`
	}{
		Driver:  "local",
		Options: map[string]any{"root": "./data/storage"},
	}

	got, err := filesystem.LocalPublicURL(cfg, "local", "public/avatar/image.png")
	if err != nil {
		t.Fatalf("LocalPublicURL returned error: %v", err)
	}

	want := "/uploads/avatar/image.png"
	if got != want {
		t.Fatalf("LocalPublicURL = %q, want %q", got, want)
	}
}

func TestXAccelRedirectPath(t *testing.T) {
	cfg := accessURLTestConfig()

	got, err := filesystem.XAccelRedirectPath(cfg, "attachments/2026/06/image.png")
	if err != nil {
		t.Fatalf("XAccelRedirectPath returned error: %v", err)
	}

	want := "/_protected_attachments/attachments/2026/06/image.png"
	if got != want {
		t.Fatalf("XAccelRedirectPath = %q, want %q", got, want)
	}
}

func TestS3PublicURLUsesDiskPublicBaseURLAndKeepsStorageKey(t *testing.T) {
	cfg := accessURLTestConfig()

	got, err := filesystem.S3PublicURL(cfg, "s3", "public/avatar/image.png")
	if err != nil {
		t.Fatalf("S3PublicURL returned error: %v", err)
	}

	want := "https://s3-cdn.example.com/public/avatar/image.png"
	if got != want {
		t.Fatalf("S3PublicURL = %q, want %q", got, want)
	}
}

func TestS3PresignedGetURL(t *testing.T) {
	cfg := accessURLTestConfig()

	got, err := filesystem.S3PresignedGetURL(context.Background(), cfg, "s3", "attachments/2026/06/image.png", 10*time.Minute)
	if err != nil {
		t.Fatalf("S3PresignedGetURL returned error: %v", err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("presigned URL is invalid: %v", err)
	}
	query := parsed.Query()
	if query.Get("X-Amz-Expires") != "600" {
		t.Fatalf("X-Amz-Expires = %q, want 600", query.Get("X-Amz-Expires"))
	}
	if query.Get("X-Amz-Signature") == "" {
		t.Fatalf("expected X-Amz-Signature in %q", got)
	}
	if !strings.Contains(parsed.EscapedPath(), "attachments/2026/06/image.png") {
		t.Fatalf("expected storage key in path, got %q", parsed.EscapedPath())
	}
}
