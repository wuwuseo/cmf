package filesystem_test

import (
	"context"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wuwuseo/cmf/config"
	"github.com/wuwuseo/cmf/filesystem"
)

func s3TestConfig(appName string) *config.Config {
	cfg := &config.Config{}
	cfg.App.Name = appName
	cfg.Filesystem.Default = "s3"
	cfg.Filesystem.Disks = map[string]config.DiskConfig{
		"local": {
			Driver:  "local",
			Options: map[string]any{"root": "./data/storage"},
		},
		"s3": {
			Driver: "s3",
			Options: map[string]any{
				"bucket": "bucket",
			},
		},
	}
	return cfg
}

func TestNewStorageDriverS3MissingBucketReturnsError(t *testing.T) {
	cfg := s3TestConfig("s3-missing-bucket")
	cfg.Filesystem.Disks["s3"] = config.DiskConfig{
		Driver:  "s3",
		Options: map[string]any{},
	}

	if _, err := filesystem.NewStorageDriver(cfg, "s3"); err == nil {
		t.Fatal("缺少 bucket 时应返回错误")
	}
}

func TestNewStorageDriverS3ToleratesMissingOptionalOptions(t *testing.T) {
	// 历史实现直接对 options["region"] 做 .(string)，缺失时会 panic；
	// 这里保证只填 bucket 也能创建驱动（region 自动回落 us-east-1）。
	cfg := s3TestConfig("s3-minimal-options")

	driver, err := filesystem.NewStorageDriver(cfg, "s3")
	if err != nil {
		t.Fatalf("创建 s3 驱动失败: %v", err)
	}
	if driver == nil {
		t.Fatal("驱动不应为 nil")
	}
}

func TestLocalPublicRootDerivesFromLocalDiskRoot(t *testing.T) {
	cfg := s3TestConfig("local-public-root")
	cfg.Filesystem.Default = "s3"

	got, ok := filesystem.LocalPublicRoot(*cfg, "")
	if !ok {
		t.Fatal("存在 local 磁盘时应返回挂载目录")
	}
	want := filepath.Join("./data/storage", "public")
	if got != want {
		t.Fatalf("LocalPublicRoot = %q, want %q", got, want)
	}
}

func TestLocalPublicRootPrefersLocalDefaultDisk(t *testing.T) {
	cfg := s3TestConfig("local-public-root-default")
	cfg.Filesystem.Default = "local"
	cfg.Filesystem.Disks["local"] = config.DiskConfig{
		Driver:  "local",
		Options: map[string]any{"root": "/var/lib/app-storage"},
	}

	got, ok := filesystem.LocalPublicRoot(*cfg, "")
	if !ok {
		t.Fatal("默认磁盘为 local 时应返回挂载目录")
	}
	want := filepath.Join("/var/lib/app-storage", "public")
	if got != want {
		t.Fatalf("LocalPublicRoot = %q, want %q", got, want)
	}
}

func TestLocalPublicRootReturnsFalseWithoutLocalDisk(t *testing.T) {
	cfg := s3TestConfig("local-public-root-missing")
	delete(cfg.Filesystem.Disks, "local")

	if got, ok := filesystem.LocalPublicRoot(*cfg, ""); ok {
		t.Fatalf("未配置 local 磁盘时应跳过挂载，got %q", got)
	}
}

func TestS3PublicURLRequiresDiskBaseURL(t *testing.T) {
	cfg := accessURLTestConfig()
	cfg.Filesystem.Disks["s3"] = config.DiskConfig{
		Driver: "s3",
		Options: map[string]any{
			"access_key": "AKIA_TEST",
			"secret_key": "secret",
			"region":     "us-east-1",
			"bucket":     "bucket",
		},
	}

	if _, err := filesystem.S3PublicURL(cfg, "s3", "public/avatar/image.png"); err == nil {
		t.Fatal("S3 公开直链缺少 public_base_url 时应返回错误，而不是回落到 /uploads")
	}
}

func TestS3PresignedGetURLUsesPathStyleForCustomEndpoint(t *testing.T) {
	cfg := accessURLTestConfig()
	cfg.Filesystem.Disks["s3"] = config.DiskConfig{
		Driver: "s3",
		Options: map[string]any{
			"access_key": "AKIA_TEST",
			"secret_key": "secret",
			"region":     "us-east-1",
			"bucket":     "bucket",
			"endpoint":   "127.0.0.1:9000",
		},
	}

	got, err := filesystem.S3PresignedGetURL(context.Background(), cfg, "s3", "attachments/image.png", time.Minute)
	if err != nil {
		t.Fatalf("S3PresignedGetURL returned error: %v", err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("presigned URL is invalid: %v", err)
	}
	if parsed.Host != "127.0.0.1:9000" {
		t.Fatalf("host = %q, want 127.0.0.1:9000", parsed.Host)
	}
	// 自定义 endpoint 未显式配置 use_path_style 时也应按 path-style 签名
	if !strings.HasPrefix(parsed.EscapedPath(), "/bucket/attachments/image.png") {
		t.Fatalf("expected path-style URL, got path %q", parsed.EscapedPath())
	}
}
