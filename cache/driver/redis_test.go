package driver_test

import (
	"context"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/wuwuseo/cmf/cache/driver"
	"github.com/wuwuseo/cmf/config"
)

var redisContainer *redis.RedisContainer
var testRedisAddr string

func TestMain(m *testing.M) {
	ctx := context.Background()

	c, err := redis.Run(ctx, "redis:7-alpine")
	if err != nil {
		os.Exit(0)
	}
	redisContainer = c

	connStr, err := c.ConnectionString(ctx)
	if err != nil {
		c.Terminate(ctx)
		os.Exit(0)
	}
	testRedisAddr = connStr

	code := m.Run()

	c.Terminate(ctx)
	os.Exit(code)
}

func newRedisConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Redis.Default = "test"
	cfg.Redis.Connections = map[string]config.Redis{
		"test": {
			Addr:         testRedisAddr,
			Username:     "",
			Password:     "",
			DB:           0,
			DialTimeout:  5,
			ReadTimeout:  3,
			WriteTimeout: 3,
			PoolSize:     10,
			MinIdleConns: 5,
			MaxIdleConns: 10,
		},
	}
	cfg.Cache.Default = "redis"
	cfg.Cache.Stores = map[string]struct {
		Driver     string `mapstructure:"driver"`
		DefaultTTL int    `mapstructure:"default_ttl"`
		Options    any    `mapstructure:"options"`
	}{
		"redis": {Driver: "redis", DefaultTTL: 3600, Options: nil},
	}
	return cfg
}

func TestNewRedisCache_Create(t *testing.T) {
	cfg := newRedisConfig()
	ctx := context.Background()
	store := driver.NewRedisCache(ctx, cfg)
	if store == nil {
		t.Error("NewRedisCache 返回值不应为 nil")
	}
}

func TestNewRedisCache_GetSet(t *testing.T) {
	cfg := newRedisConfig()
	ctx := context.Background()
	store := driver.NewRedisCache(ctx, cfg)

	key := "test_redis_key"
	value := []byte("test_redis_value")

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

	_ = store.Delete(ctx, key)
}

func TestNewRedisCache_Delete(t *testing.T) {
	cfg := newRedisConfig()
	ctx := context.Background()
	store := driver.NewRedisCache(ctx, cfg)

	key := "test_redis_delete"
	value := []byte("test_redis_delete_value")

	_ = store.Set(ctx, key, value)
	_ = store.Delete(ctx, key)

	_, err := store.Get(ctx, key)
	if err == nil {
		t.Error("删除后 Get 应该返回错误")
	}
}

func TestNewRedisCache_GetNonExistentKey(t *testing.T) {
	cfg := newRedisConfig()
	ctx := context.Background()
	store := driver.NewRedisCache(ctx, cfg)

	_, err := store.Get(ctx, "non_existent_redis_key")
	if err == nil {
		t.Error("不存在的 key 应该返回错误")
	}
}
