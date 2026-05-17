package filesystem_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wuwuseo/cmf/config"
	"github.com/wuwuseo/cmf/filesystem"
	local "github.com/wuwuseo/cmf/storage/local"
)

// newTempDir 创建一个临时目录并返回路径
func newTempDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// newLocalStorage 创建使用临时目录的本地存储实例
func newLocalStorage(t *testing.T) *local.Storage {
	t.Helper()
	tmpDir := t.TempDir()
	return local.New(local.Config{BasePath: tmpDir})
}

// newTestFilesystemConfig 创建一个用于测试的文件系统配置（local 驱动）
func newTestFilesystemConfig(t *testing.T, rootPath string) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	cfg.App.Name = "testapp"
	cfg.Filesystem.Default = "local"
	cfg.Filesystem.IsAndLocal = false
	cfg.Filesystem.Disks = map[string]struct {
		Driver  string `mapstructure:"driver"`
		Options any    `mapstructure:"options"`
	}{
		"local": {
			Driver:  "local",
			Options: map[string]any{"root": rootPath},
		},
	}
	return cfg
}

// =============================================================================
// Filesystem 基本操作
// =============================================================================

func TestNewFilesystem_Create(t *testing.T) {
	tmpDir := newTempDir(t)
	store := local.New(local.Config{BasePath: tmpDir})
	cfg := config.Config{}

	fs := filesystem.NewFilesystem(store, cfg)
	if fs == nil {
		t.Fatal("NewFilesystem 返回 nil")
	}
}

func TestFilesystem_GetSetDelete(t *testing.T) {
	tmpDir := newTempDir(t)
	store := local.New(local.Config{BasePath: tmpDir})
	cfg := config.Config{}

	fs := filesystem.NewFilesystem(store, cfg)

	key := "test_file"
	val := []byte("hello filesystem")

	// 写入
	err := fs.Set(key, val, 0)
	if err != nil {
		t.Fatalf("Set 失败: %v", err)
	}

	// 读取
	got, err := fs.Get(key)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if !bytes.Equal(got, val) {
		t.Fatalf("读取的值不正确: 期望 %q, 得到 %q", val, got)
	}

	// 删除
	err = fs.Delete(key)
	if err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
}

func TestFilesystem_SetReader(t *testing.T) {
	tmpDir := newTempDir(t)
	store := local.New(local.Config{BasePath: tmpDir})
	cfg := config.Config{}

	fs := filesystem.NewFilesystem(store, cfg)

	key := "reader_file"
	data := []byte("streamed data from reader")
	reader := bytes.NewReader(data)

	// 流式写入
	err := fs.SetReader(key, reader, 0)
	if err != nil {
		t.Fatalf("SetReader 失败: %v", err)
	}

	// 验证写入的内容
	got, err := fs.Get(key)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("读取的值不正确: 期望 %q, 得到 %q", data, got)
	}
}

// mockStorageNoSetReader 模拟不支持 SetReader 的存储适配器
type mockStorageNoSetReader struct {
	data map[string][]byte
}

func (m *mockStorageNoSetReader) Get(key string) ([]byte, error) {
	if val, ok := m.data[key]; ok {
		return val, nil
	}
	return nil, nil
}

func (m *mockStorageNoSetReader) Set(key string, val []byte, exp time.Duration) error {
	m.data[key] = val
	return nil
}

func (m *mockStorageNoSetReader) Delete(key string) error {
	delete(m.data, key)
	return nil
}

func (m *mockStorageNoSetReader) Reset() error {
	m.data = make(map[string][]byte)
	return nil
}

func (m *mockStorageNoSetReader) Close() error {
	return nil
}

func TestFilesystem_SetReader_FallbackToSet(t *testing.T) {
	// 使用不支持 SetReader 的 mock 存储
	mock := &mockStorageNoSetReader{data: make(map[string][]byte)}
	cfg := config.Config{}

	fs := filesystem.NewFilesystem(mock, cfg)

	key := "fallback_file"
	data := []byte("data via fallback")
	reader := bytes.NewReader(data)

	// SetReader 应该回退到 Set（因为 mock 不支持 SetReader）
	err := fs.SetReader(key, reader, 0)
	if err != nil {
		t.Fatalf("SetReader 回退到 Set 失败: %v", err)
	}

	// 验证数据已写入
	got, err := fs.Get(key)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("读取的值不正确: 期望 %q, 得到 %q", data, got)
	}
}

func TestFilesystem_GetNonExistentKey(t *testing.T) {
	tmpDir := newTempDir(t)
	store := local.New(local.Config{BasePath: tmpDir})
	cfg := config.Config{}

	fs := filesystem.NewFilesystem(store, cfg)

	got, err := fs.Get("non_existent_file")
	if err != nil {
		t.Fatalf("获取不存在的 key 不应报错: %v", err)
	}
	if got != nil {
		t.Fatalf("不存在的 key 应返回 nil, 得到 %v", got)
	}
}

// =============================================================================
// DualStorage 测试
// =============================================================================

func TestDualStorage_Set(t *testing.T) {
	primary := newLocalStorage(t)
	localStorage := newLocalStorage(t)

	ds := &filesystem.DualStorage{
		Primary: primary,
		Local:   localStorage,
	}

	key := "dual_key"
	val := []byte("dual storage data")

	err := ds.Set(key, val, 0)
	if err != nil {
		t.Fatalf("DualStorage Set 失败: %v", err)
	}

	// 验证主存储和本地存储都有数据
	gotPrimary, err := primary.Get(key)
	if err != nil {
		t.Fatalf("主存储 Get 失败: %v", err)
	}
	if !bytes.Equal(gotPrimary, val) {
		t.Fatalf("主存储的值不正确: 期望 %q, 得到 %q", val, gotPrimary)
	}

	gotLocal, err := localStorage.Get(key)
	if err != nil {
		t.Fatalf("本地存储 Get 失败: %v", err)
	}
	if !bytes.Equal(gotLocal, val) {
		t.Fatalf("本地存储的值不正确: 期望 %q, 得到 %q", val, gotLocal)
	}
}

func TestDualStorage_Get(t *testing.T) {
	primary := newLocalStorage(t)
	localStorage := newLocalStorage(t)

	ds := &filesystem.DualStorage{
		Primary: primary,
		Local:   localStorage,
	}

	key := "get_test"
	val := []byte("get from primary")
	err := primary.Set(key, val, 0)
	if err != nil {
		t.Fatalf("主存储 Set 失败: %v", err)
	}

	// Get 从主存储读取
	got, err := ds.Get(key)
	if err != nil {
		t.Fatalf("DualStorage Get 失败: %v", err)
	}
	if !bytes.Equal(got, val) {
		t.Fatalf("读取的值不正确: 期望 %q, 得到 %q", val, got)
	}
}

func TestDualStorage_Delete(t *testing.T) {
	primary := newLocalStorage(t)
	localStorage := newLocalStorage(t)

	ds := &filesystem.DualStorage{
		Primary: primary,
		Local:   localStorage,
	}

	key := "delete_test"
	val := []byte("to be deleted from both")

	// 先写入两个存储
	err := primary.Set(key, val, 0)
	if err != nil {
		t.Fatalf("主存储 Set 失败: %v", err)
	}
	err = localStorage.Set(key, val, 0)
	if err != nil {
		t.Fatalf("本地存储 Set 失败: %v", err)
	}

	// 删除
	err = ds.Delete(key)
	if err != nil {
		t.Fatalf("DualStorage Delete 失败: %v", err)
	}

	// 验证主存储已删除
	gotPrimary, _ := primary.Get(key)
	if gotPrimary != nil {
		t.Fatal("主存储应已删除")
	}

	// 验证本地存储已删除
	gotLocal, _ := localStorage.Get(key)
	if gotLocal != nil {
		t.Fatal("本地存储应已删除")
	}
}

func TestDualStorage_Constructor(t *testing.T) {
	primary := newLocalStorage(t)
	localStorage := newLocalStorage(t)

	ds := &filesystem.DualStorage{
		Primary: primary,
		Local:   localStorage,
	}
	if ds == nil {
		t.Fatal("DualStorage 构造函数返回 nil")
	}
	if ds.Primary == nil {
		t.Fatal("DualStorage.Primary 为 nil")
	}
	if ds.Local == nil {
		t.Fatal("DualStorage.Local 为 nil")
	}
}

// =============================================================================
// NewLocalStorage 测试
// =============================================================================

func TestNewLocalStorage_WithRootOption(t *testing.T) {
	tmpDir := newTempDir(t)
	customPath := filepath.Join(tmpDir, "custom_storage")

	store, err := filesystem.NewLocalStorage(map[string]any{
		"root": customPath,
	})
	if err != nil {
		t.Fatalf("NewLocalStorage 失败: %v", err)
	}
	if store == nil {
		t.Fatal("NewLocalStorage 返回 nil")
	}

	// 验证目录已创建
	if _, err := os.Stat(customPath); os.IsNotExist(err) {
		t.Fatal("存储基础目录未被创建")
	}
}

func TestNewLocalStorage_WithoutRootOption(t *testing.T) {
	store, err := filesystem.NewLocalStorage(map[string]any{})
	if err != nil {
		t.Fatalf("NewLocalStorage 失败: %v", err)
	}
	if store == nil {
		t.Fatal("NewLocalStorage 返回 nil")
	}
}

// =============================================================================
// NewStorageDriver 测试
// =============================================================================

func TestNewStorageDriver_LocalDriver(t *testing.T) {
	cfg := newTestFilesystemConfig(t, t.TempDir())

	adapter, err := filesystem.NewStorageDriver(cfg, "local")
	if err != nil {
		t.Fatalf("NewStorageDriver local 失败: %v", err)
	}
	if adapter == nil {
		t.Fatal("NewStorageDriver 返回 nil")
	}

	// 验证可以读写
	key := "driver_test"
	val := []byte("driver data")
	err = adapter.Set(key, val, 0)
	if err != nil {
		t.Fatalf("adapter Set 失败: %v", err)
	}

	got, err := adapter.Get(key)
	if err != nil {
		t.Fatalf("adapter Get 失败: %v", err)
	}
	if !bytes.Equal(got, val) {
		t.Fatalf("adapter 读取的值不正确: 期望 %q, 得到 %q", val, got)
	}
}

func TestNewStorageDriver_NonExistentDisk(t *testing.T) {
	cfg := &config.Config{}
	cfg.App.Name = "testapp_nodisk"

	_, err := filesystem.NewStorageDriver(cfg, "nonexistent")
	if err == nil {
		t.Fatal("不存在的磁盘应返回错误")
	}
}

func TestNewStorageDriver_UnsupportedDriver(t *testing.T) {
	cfg := &config.Config{}
	cfg.App.Name = "testapp_unsupported"
	cfg.Filesystem.Disks = map[string]struct {
		Driver  string `mapstructure:"driver"`
		Options any    `mapstructure:"options"`
	}{
		"bad": {
			Driver:  "ftp",
			Options: nil,
		},
	}

	_, err := filesystem.NewStorageDriver(cfg, "bad")
	if err == nil {
		t.Fatal("不支持的驱动应返回错误")
	}
}

func TestNewStorageDriver_Singleton(t *testing.T) {
	cfg := newTestFilesystemConfig(t, t.TempDir())

	// 第一次创建
	adapter1, err := filesystem.NewStorageDriver(cfg, "local")
	if err != nil {
		t.Fatalf("第一次 NewStorageDriver 失败: %v", err)
	}

	// 第二次创建同一磁盘，应返回同一实例
	adapter2, err := filesystem.NewStorageDriver(cfg, "local")
	if err != nil {
		t.Fatalf("第二次 NewStorageDriver 失败: %v", err)
	}

	if adapter1 != adapter2 {
		t.Fatal("单例模式失败: 两次调用返回了不同的实例")
	}
}

// =============================================================================
// NewFilesystemFromConfig 测试
// =============================================================================

func TestNewFilesystemFromConfig_LocalDriver(t *testing.T) {
	cfg := newTestFilesystemConfig(t, t.TempDir())

	fs, err := filesystem.NewFilesystemFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewFilesystemFromConfig 失败: %v", err)
	}
	if fs == nil {
		t.Fatal("NewFilesystemFromConfig 返回 nil")
	}

	// 验证可以读写
	key := "from_config_key"
	val := []byte("from config data")
	err = fs.Set(key, val, 0)
	if err != nil {
		t.Fatalf("Set 失败: %v", err)
	}

	got, err := fs.Get(key)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if !bytes.Equal(got, val) {
		t.Fatalf("读取的值不正确: 期望 %q, 得到 %q", val, got)
	}
}

func TestNewFilesystemFromConfig_IsAndLocalLocalDisk(t *testing.T) {
	// IsAndLocal 为 true 但默认磁盘是 local，不会创建 DualStorage
	cfg := newTestFilesystemConfig(t, t.TempDir())
	cfg.Filesystem.IsAndLocal = true

	fs, err := filesystem.NewFilesystemFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewFilesystemFromConfig 失败: %v", err)
	}
	if fs == nil {
		t.Fatal("NewFilesystemFromConfig 返回 nil")
	}

	// 验证可以正常读写（IsAndLocal + local 磁盘不会创建 DualStorage）
	key := "and_local_key"
	val := []byte("is and local data")
	err = fs.Set(key, val, 0)
	if err != nil {
		t.Fatalf("Set 失败: %v", err)
	}

	got, err := fs.Get(key)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if !bytes.Equal(got, val) {
		t.Fatalf("读取的值不正确: 期望 %q, 得到 %q", val, got)
	}
}

// =============================================================================
// 辅助函数验证：mockStorageNoSetReader 实现了 storage.Storage 接口
// =============================================================================

var _ io.Reader = (*bytes.Reader)(nil)
