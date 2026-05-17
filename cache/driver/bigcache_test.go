package driver_test

import (
	"context"
	"testing"

	"github.com/wuwuseo/cmf/cache/driver"
	"github.com/wuwuseo/cmf/config"
)

func newTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Cache.Default = "memory"
	cfg.Cache.Stores = map[string]struct {
		Driver     string `mapstructure:"driver"`
		DefaultTTL int    `mapstructure:"default_ttl"`
		Options    any    `mapstructure:"options"`
	}{
		"memory": {Driver: "memory", DefaultTTL: 3600, Options: nil},
	}
	return cfg
}

func TestNewBigCache_Create(t *testing.T) {
	cfg := newTestConfig()
	ctx := context.Background()
	store := driver.NewBigCache(ctx, cfg)
	if store == nil {
		t.Error("NewBigCache 返回值不应为 nil")
	}
}

func TestNewBigCache_GetSet(t *testing.T) {
	cfg := newTestConfig()
	ctx := context.Background()
	store := driver.NewBigCache(ctx, cfg)

	key := "test_key_1"
	value := []byte("test_value_1")

	err := store.Set(ctx, key, value)
	if err != nil {
		t.Errorf("Set 操作失败: %v", err)
	}

	result, err := store.Get(ctx, key)
	if err != nil {
		t.Errorf("Get 操作失败: %v", err)
	}
	if string(result.([]byte)) != string(value) {
		t.Errorf("期望值 %s，实际 %s", value, result)
	}
}

func TestNewBigCache_Delete(t *testing.T) {
	cfg := newTestConfig()
	ctx := context.Background()
	store := driver.NewBigCache(ctx, cfg)

	key := "test_key_delete"
	value := []byte("test_value_delete")

	_ = store.Set(ctx, key, value)
	_ = store.Delete(ctx, key)

	_, err := store.Get(ctx, key)
	if err == nil {
		t.Error("删除后 Get 应该返回错误")
	}
}

func TestNewBigCache_GetNonExistentKey(t *testing.T) {
	cfg := newTestConfig()
	ctx := context.Background()
	store := driver.NewBigCache(ctx, cfg)

	_, err := store.Get(ctx, "non_existent_key")
	if err == nil {
		t.Error("不存在的 key 应该返回错误")
	}
}
