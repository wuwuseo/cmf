package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/wuwuseo/cmf/cache"
	"github.com/wuwuseo/cmf/config"
)

// newTestConfig 创建一个用于测试的缓存配置（使用 BigCache 驱动）
func newTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Cache.Default = "memory"
	cfg.Cache.Stores = map[string]struct {
		Driver     string `mapstructure:"driver"`
		DefaultTTL int    `mapstructure:"default_ttl"`
		Options    any    `mapstructure:"options"`
	}{
		"memory": {Driver: "memory", DefaultTTL: 3600},
	}
	return cfg
}

// =============================================================================
// Cache[[]byte] 测试（BigCache 驱动）
// =============================================================================

func TestNewCache_Create(t *testing.T) {
	cfg := newTestConfig()
	ctx := context.Background()

	c := cache.NewCache(ctx, cfg)
	if c == nil {
		t.Fatal("NewCache 返回 nil")
	}
}

func TestNewCache_GetSet(t *testing.T) {
	cfg := newTestConfig()
	ctx := context.Background()
	c := cache.NewCache(ctx, cfg)

	key := "test_key"
	val := []byte("hello world")

	// 写入缓存
	err := c.Set(ctx, key, val)
	if err != nil {
		t.Fatalf("Set 失败: %v", err)
	}

	// 读取缓存
	got, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}

	if string(got) != string(val) {
		t.Fatalf("读取的值不正确: 期望 %q, 得到 %q", val, got)
	}
}

func TestNewCache_Delete(t *testing.T) {
	cfg := newTestConfig()
	ctx := context.Background()
	c := cache.NewCache(ctx, cfg)

	key := "delete_key"
	val := []byte("to be deleted")

	// 先写入
	err := c.Set(ctx, key, val)
	if err != nil {
		t.Fatalf("Set 失败: %v", err)
	}

	// 删除
	err = c.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
}

func TestNewCache_Clear(t *testing.T) {
	cfg := newTestConfig()
	ctx := context.Background()
	c := cache.NewCache(ctx, cfg)

	// 写入多条数据
	err := c.Set(ctx, "key1", []byte("val1"))
	if err != nil {
		t.Fatalf("Set key1 失败: %v", err)
	}
	err = c.Set(ctx, "key2", []byte("val2"))
	if err != nil {
		t.Fatalf("Set key2 失败: %v", err)
	}

	// 清空缓存
	err = c.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear 失败: %v", err)
	}
}

func TestNewCache_GetNonExistentKey(t *testing.T) {
	cfg := newTestConfig()
	ctx := context.Background()
	c := cache.NewCache(ctx, cfg)

	_, err := c.Get(ctx, "non_existent_key")
	if err == nil {
		t.Fatal("获取不存在的 key 应该返回错误")
	}
}

// =============================================================================
// TypedCache 测试
// =============================================================================

// Person 测试用的自定义结构体
type Person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestNewTypedCache_Create(t *testing.T) {
	cfg := newTestConfig()
	ctx := context.Background()
	rawCache := cache.NewCache(ctx, cfg)

	tc := cache.NewTypedCache[Person](rawCache)
	if tc == nil {
		t.Fatal("NewTypedCache 返回 nil")
	}
}

func TestTypedCache_SetGet_Struct(t *testing.T) {
	cfg := newTestConfig()
	ctx := context.Background()
	rawCache := cache.NewCache(ctx, cfg)
	tc := cache.NewTypedCache[Person](rawCache)

	key := "person_alice"
	expected := Person{Name: "Alice", Age: 30}

	err := tc.Set(ctx, key, expected)
	if err != nil {
		t.Fatalf("TypedCache Set 失败: %v", err)
	}

	got, err := tc.Get(ctx, key)
	if err != nil {
		t.Fatalf("TypedCache Get 失败: %v", err)
	}

	if got.Name != expected.Name || got.Age != expected.Age {
		t.Fatalf("读取的值不正确: 期望 %+v, 得到 %+v", expected, got)
	}
}

func TestTypedCache_GetNonExistentKey(t *testing.T) {
	cfg := newTestConfig()
	ctx := context.Background()
	rawCache := cache.NewCache(ctx, cfg)
	tc := cache.NewTypedCache[Person](rawCache)

	_, err := tc.Get(ctx, "non_existent_person")
	if err == nil {
		t.Fatal("获取不存在的 key 应该返回错误")
	}
}

func TestTypedCache_SetWithExpiration(t *testing.T) {
	cfg := newTestConfig()
	ctx := context.Background()
	rawCache := cache.NewCache(ctx, cfg)
	tc := cache.NewTypedCache[Person](rawCache)

	key := "person_bob"
	expected := Person{Name: "Bob", Age: 25}

	// 设置 1 秒过期
	err := tc.SetWithExpiration(ctx, key, expected, 1*time.Second)
	if err != nil {
		t.Fatalf("SetWithExpiration 失败: %v", err)
	}

	// 立即读取应该成功
	got, err := tc.Get(ctx, key)
	if err != nil {
		t.Fatalf("过期前的 Get 失败: %v", err)
	}
	if got.Name != expected.Name || got.Age != expected.Age {
		t.Fatalf("过期前的值不正确: 期望 %+v, 得到 %+v", expected, got)
	}
}

func TestTypedCache_Delete(t *testing.T) {
	cfg := newTestConfig()
	ctx := context.Background()
	rawCache := cache.NewCache(ctx, cfg)
	tc := cache.NewTypedCache[Person](rawCache)

	key := "person_to_delete"
	expected := Person{Name: "Charlie", Age: 40}

	err := tc.Set(ctx, key, expected)
	if err != nil {
		t.Fatalf("Set 失败: %v", err)
	}

	err = tc.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
}

func TestTypedCache_Clear(t *testing.T) {
	cfg := newTestConfig()
	ctx := context.Background()
	rawCache := cache.NewCache(ctx, cfg)
	tc := cache.NewTypedCache[Person](rawCache)

	err := tc.Set(ctx, "person1", Person{Name: "A", Age: 1})
	if err != nil {
		t.Fatalf("Set person1 失败: %v", err)
	}
	err = tc.Set(ctx, "person2", Person{Name: "B", Age: 2})
	if err != nil {
		t.Fatalf("Set person2 失败: %v", err)
	}

	err = tc.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear 失败: %v", err)
	}
}

func TestTypedCache_MapType(t *testing.T) {
	cfg := newTestConfig()
	ctx := context.Background()
	rawCache := cache.NewCache(ctx, cfg)
	tc := cache.NewTypedCache[map[string]any](rawCache)

	key := "config_map"
	expected := map[string]any{
		"host": "localhost",
		"port": 8080,
	}

	err := tc.Set(ctx, key, expected)
	if err != nil {
		t.Fatalf("TypedCache Set map 失败: %v", err)
	}

	got, err := tc.Get(ctx, key)
	if err != nil {
		t.Fatalf("TypedCache Get map 失败: %v", err)
	}

	if got["host"] != expected["host"] {
		t.Fatalf("map 值不正确: host 期望 %v, 得到 %v", expected["host"], got["host"])
	}
}

func TestTypedCache_StringType(t *testing.T) {
	cfg := newTestConfig()
	ctx := context.Background()
	rawCache := cache.NewCache(ctx, cfg)
	tc := cache.NewTypedCache[string](rawCache)

	key := "greeting"
	expected := "Hello, 世界"

	err := tc.Set(ctx, key, expected)
	if err != nil {
		t.Fatalf("TypedCache Set string 失败: %v", err)
	}

	got, err := tc.Get(ctx, key)
	if err != nil {
		t.Fatalf("TypedCache Get string 失败: %v", err)
	}

	if got != expected {
		t.Fatalf("string 值不正确: 期望 %q, 得到 %q", expected, got)
	}
}

func TestCache_Store_Default(t *testing.T) {
	ctx := context.Background()
	cfg := newTestConfig()

	c := cache.NewCache(ctx, cfg)
	store, err := c.Store("memory")
	if err != nil {
		t.Fatalf("Store(memory) 失败: %v", err)
	}
	if store == nil {
		t.Fatal("store 应该非 nil")
	}
}

func TestCache_Store_Nonexistent(t *testing.T) {
	ctx := context.Background()
	cfg := newTestConfig()

	c := cache.NewCache(ctx, cfg)
	_, err := c.Store("nonexistent")
	if err == nil {
		t.Fatal("Store(nonexistent) 应该返回错误")
	}
}

func TestCache_Store_Singleton(t *testing.T) {
	ctx := context.Background()
	cfg := newTestConfig()

	// 手动添加 another 存储
	storesCopy := make(map[string]struct {
		Driver     string `mapstructure:"driver"`
		DefaultTTL int    `mapstructure:"default_ttl"`
		Options    any    `mapstructure:"options"`
	})
	for k, v := range cfg.Cache.Stores {
		storesCopy[k] = v
	}
	storesCopy["another"] = struct {
		Driver     string `mapstructure:"driver"`
		DefaultTTL int    `mapstructure:"default_ttl"`
		Options    any    `mapstructure:"options"`
	}{
		Driver:     "memory",
		DefaultTTL: 3600,
		Options:    nil,
	}
	cfg.Cache.Stores = storesCopy

	c := cache.NewCache(ctx, cfg)

	store1, _ := c.Store("another")
	store2, _ := c.Store("another")
	if store1 != store2 {
		t.Fatal("同一存储 Store() 两次应该返回同一实例")
	}
}
