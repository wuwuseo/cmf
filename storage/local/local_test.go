package local_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wuwuseo/cmf/storage/local"
)

// newTempStorage 创建使用临时目录的本地存储实例
func newTempStorage(t *testing.T) *local.Storage {
	t.Helper()
	tmpDir := t.TempDir()
	store := local.New(local.Config{BasePath: tmpDir})
	return store
}

// =============================================================================
// New 测试
// =============================================================================

func TestNew_DefaultPath(t *testing.T) {
	store := local.New()
	if store.BasePath != "./data/storage" {
		t.Fatalf("默认路径不正确: 期望 %q, 得到 %q", "./data/storage", store.BasePath)
	}
}

func TestNew_CustomPath(t *testing.T) {
	customPath := filepath.Join(t.TempDir(), "custom_store")
	store := local.New(local.Config{BasePath: customPath})
	if store.BasePath != customPath {
		t.Fatalf("自定义路径不正确: 期望 %q, 得到 %q", customPath, store.BasePath)
	}
	// 确认目录已创建
	if _, err := os.Stat(customPath); os.IsNotExist(err) {
		t.Fatal("存储基础目录未被创建")
	}
}

func TestNew_BasePath(t *testing.T) {
	tmpDir := t.TempDir()
	store := local.New(local.Config{BasePath: tmpDir})
	if store.BasePath != tmpDir {
		t.Fatalf("BasePath 不正确: 期望 %q, 得到 %q", tmpDir, store.BasePath)
	}
}

// =============================================================================
// Set / Get 测试
// =============================================================================

func TestSetGet(t *testing.T) {
	store := newTempStorage(t)

	key := "test_key"
	val := []byte("hello world")

	err := store.Set(key, val, 0)
	if err != nil {
		t.Fatalf("Set 失败: %v", err)
	}

	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if !bytes.Equal(got, val) {
		t.Fatalf("读取的值不正确: 期望 %q, 得到 %q", val, got)
	}
}

func TestSet_EmptyKey(t *testing.T) {
	store := newTempStorage(t)

	err := store.Set("", []byte("data"), 0)
	if err != nil {
		t.Fatalf("空键不应报错: %v", err)
	}
}

func TestSet_NilValue(t *testing.T) {
	store := newTempStorage(t)

	err := store.Set("some_key", nil, 0)
	if err != nil {
		t.Fatalf("空值不应报错: %v", err)
	}
}

func TestGet_NonExistentKey(t *testing.T) {
	store := newTempStorage(t)

	got, err := store.Get("non_existent_key")
	if err != nil {
		t.Fatalf("获取不存在的键不应报错: %v", err)
	}
	if got != nil {
		t.Fatalf("不存在的键应返回 nil, 得到 %v", got)
	}
}

// =============================================================================
// SetWithContext / GetWithContext 测试
// =============================================================================

func TestSetGetWithContext(t *testing.T) {
	store := newTempStorage(t)

	ctx := context.Background()
	key := "ctx_key"
	val := []byte("context data")

	err := store.SetWithContext(ctx, key, val, 0)
	if err != nil {
		t.Fatalf("SetWithContext 失败: %v", err)
	}

	got, err := store.GetWithContext(ctx, key)
	if err != nil {
		t.Fatalf("GetWithContext 失败: %v", err)
	}
	if !bytes.Equal(got, val) {
		t.Fatalf("读取的值不正确: 期望 %q, 得到 %q", val, got)
	}
}

func TestSetWithContext_Cancelled(t *testing.T) {
	store := newTempStorage(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消上下文

	err := store.SetWithContext(ctx, "key", []byte("data"), 0)
	if err == nil {
		t.Fatal("上下文已取消时应返回错误")
	}
}

func TestGetWithContext_Cancelled(t *testing.T) {
	store := newTempStorage(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消上下文

	_, err := store.GetWithContext(ctx, "key")
	if err == nil {
		t.Fatal("上下文已取消时应返回错误")
	}
}

// =============================================================================
// Delete 测试
// =============================================================================

func TestDelete_ExistingKey(t *testing.T) {
	store := newTempStorage(t)

	key := "delete_key"
	val := []byte("to be deleted")

	err := store.Set(key, val, 0)
	if err != nil {
		t.Fatalf("Set 失败: %v", err)
	}

	err = store.Delete(key)
	if err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
}

func TestDelete_NonExistentKey(t *testing.T) {
	store := newTempStorage(t)

	err := store.Delete("non_existent_key")
	if err != nil {
		t.Fatalf("删除不存在的键不应报错: %v", err)
	}
}

func TestDelete_ThenGet(t *testing.T) {
	store := newTempStorage(t)

	key := "delete_then_get"
	val := []byte("some data")

	err := store.Set(key, val, 0)
	if err != nil {
		t.Fatalf("Set 失败: %v", err)
	}

	err = store.Delete(key)
	if err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}

	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if got != nil {
		t.Fatalf("删除后应返回 nil, 得到 %v", got)
	}
}

// =============================================================================
// Reset 测试
// =============================================================================

func TestReset(t *testing.T) {
	store := newTempStorage(t)

	// 写入多个键
	err := store.Set("key1", []byte("val1"), 0)
	if err != nil {
		t.Fatalf("Set key1 失败: %v", err)
	}
	err = store.Set("key2", []byte("val2"), 0)
	if err != nil {
		t.Fatalf("Set key2 失败: %v", err)
	}

	err = store.Reset()
	if err != nil {
		t.Fatalf("Reset 失败: %v", err)
	}

	// 重置后所有键应不存在
	got, err := store.Get("key1")
	if err != nil {
		t.Fatalf("Get key1 失败: %v", err)
	}
	if got != nil {
		t.Fatal("重置后 key1 应返回 nil")
	}

	got, err = store.Get("key2")
	if err != nil {
		t.Fatalf("Get key2 失败: %v", err)
	}
	if got != nil {
		t.Fatal("重置后 key2 应返回 nil")
	}
}

func TestReset_EmptyStorage(t *testing.T) {
	store := newTempStorage(t)

	err := store.Reset()
	if err != nil {
		t.Fatalf("重置空存储不应报错: %v", err)
	}
}

// =============================================================================
// Close 测试
// =============================================================================

func TestClose(t *testing.T) {
	store := newTempStorage(t)

	err := store.Close()
	if err != nil {
		t.Fatalf("Close 不应报错: %v", err)
	}
}

// =============================================================================
// 过期时间测试
// =============================================================================

func TestExpiration(t *testing.T) {
	store := newTempStorage(t)

	key := "expiring_key"
	val := []byte("expiring data")

	// 设置 1 秒后过期
	err := store.Set(key, val, 1*time.Second)
	if err != nil {
		t.Fatalf("Set 失败: %v", err)
	}

	// 过期前应能读取
	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if !bytes.Equal(got, val) {
		t.Fatalf("过期前的值不正确: 期望 %q, 得到 %q", val, got)
	}

	// 等待过期
	time.Sleep(2 * time.Second)

	// 过期后应返回 nil
	got, err = store.Get(key)
	if err != nil {
		t.Fatalf("过期后 Get 不应报错: %v", err)
	}
	if got != nil {
		t.Fatalf("过期后应返回 nil, 得到 %v", got)
	}
}

func TestNoExpiration(t *testing.T) {
	store := newTempStorage(t)

	key := "permanent_key"
	val := []byte("permanent data")

	// exp=0 表示永不过期
	err := store.Set(key, val, 0)
	if err != nil {
		t.Fatalf("Set 失败: %v", err)
	}

	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if !bytes.Equal(got, val) {
		t.Fatalf("永不过期的值不正确: 期望 %q, 得到 %q", val, got)
	}
}

// =============================================================================
// SetReader / SetReaderWithContext 测试
// =============================================================================

func TestSetReader(t *testing.T) {
	store := newTempStorage(t)

	key := "reader_key"
	data := []byte("streamed data from reader")
	reader := bytes.NewReader(data)

	err := store.SetReader(key, reader, 0)
	if err != nil {
		t.Fatalf("SetReader 失败: %v", err)
	}

	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("读取的值不正确: 期望 %q, 得到 %q", data, got)
	}
}

func TestSetReaderWithContext(t *testing.T) {
	store := newTempStorage(t)

	ctx := context.Background()
	key := "reader_ctx_key"
	data := []byte("context streamed data")
	reader := bytes.NewReader(data)

	err := store.SetReaderWithContext(ctx, key, reader, 0)
	if err != nil {
		t.Fatalf("SetReaderWithContext 失败: %v", err)
	}

	got, err := store.GetWithContext(ctx, key)
	if err != nil {
		t.Fatalf("GetWithContext 失败: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("读取的值不正确: 期望 %q, 得到 %q", data, got)
	}
}

func TestSetReader_NilReader(t *testing.T) {
	store := newTempStorage(t)

	err := store.SetReader("key", nil, 0)
	if err != nil {
		t.Fatalf("nil reader 不应报错: %v", err)
	}
}

func TestSetReader_EmptyKey(t *testing.T) {
	store := newTempStorage(t)

	reader := bytes.NewReader([]byte("data"))
	err := store.SetReader("", reader, 0)
	if err != nil {
		t.Fatalf("空键不应报错: %v", err)
	}
}

func TestSetReaderContext_Cancelled(t *testing.T) {
	store := newTempStorage(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reader := bytes.NewReader([]byte("data"))

	err := store.SetReaderWithContext(ctx, "key", reader, 0)
	if err == nil {
		t.Fatal("上下文已取消时应返回错误")
	}
}

// =============================================================================
// configOrDefault 测试（通过 New 的副作用验证）
// =============================================================================

func TestConfigOrDefault_Custom(t *testing.T) {
	customPath := filepath.Join(t.TempDir(), "custom_config_store")
	store := local.New(local.Config{BasePath: customPath})

	if store.BasePath != customPath {
		t.Fatalf("自定义配置的 BasePath 不正确: 期望 %q, 得到 %q", customPath, store.BasePath)
	}
}

func TestConfigOrDefault_Default(t *testing.T) {
	store := local.New()

	if store.BasePath != "./data/storage" {
		t.Fatalf("默认配置的 BasePath 不正确: 期望 %q, 得到 %q", "./data/storage", store.BasePath)
	}
}

// =============================================================================
// DeleteWithContext 测试
// =============================================================================

func TestDeleteWithContext_Cancelled(t *testing.T) {
	store := newTempStorage(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := store.DeleteWithContext(ctx, "key")
	if err == nil {
		t.Fatal("上下文已取消时应返回错误")
	}
}

// =============================================================================
// ResetWithContext 测试
// =============================================================================

func TestResetWithContext_Cancelled(t *testing.T) {
	store := newTempStorage(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := store.ResetWithContext(ctx)
	if err == nil {
		t.Fatal("上下文已取消时应返回错误")
	}
}

// =============================================================================
// 多键读写测试
// =============================================================================

func TestMultipleKeys(t *testing.T) {
	store := newTempStorage(t)

	type kv struct {
		key string
		val []byte
	}

	pairs := []kv{
		{"key_a", []byte("alpha")},
		{"key_b", []byte("beta")},
		{"key_c", []byte("gamma")},
	}

	for _, p := range pairs {
		err := store.Set(p.key, p.val, 0)
		if err != nil {
			t.Fatalf("Set %s 失败: %v", p.key, err)
		}
	}

	for _, p := range pairs {
		got, err := store.Get(p.key)
		if err != nil {
			t.Fatalf("Get %s 失败: %v", p.key, err)
		}
		if !bytes.Equal(got, p.val) {
			t.Fatalf("键 %s 的值不正确: 期望 %q, 得到 %q", p.key, p.val, got)
		}
	}
}

// =============================================================================
// 带过期时间的 SetReader 测试
// =============================================================================

func TestSetReaderWithExpiration(t *testing.T) {
	store := newTempStorage(t)

	key := "reader_expiring"
	data := []byte("reader data with expiration")
	reader := bytes.NewReader(data)

	err := store.SetReader(key, reader, 1*time.Second)
	if err != nil {
		t.Fatalf("SetReader 失败: %v", err)
	}

	// 过期前读取
	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("过期前的值不正确: 期望 %q, 得到 %q", data, got)
	}

	// 等待过期
	time.Sleep(2 * time.Second)

	got, err = store.Get(key)
	if err != nil {
		t.Fatalf("过期后 Get 不应报错: %v", err)
	}
	if got != nil {
		t.Fatalf("过期后应返回 nil, 得到 %v", got)
	}
}
