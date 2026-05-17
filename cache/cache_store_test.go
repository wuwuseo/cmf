package cache_test

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/wuwuseo/cmf/cache"
	"github.com/wuwuseo/cmf/config"
)

func TestCache_Store(t *testing.T) {
	ctx := context.Background()

	redisContainer, err := redis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Skip("Docker not available")
	}
	defer func() { _ = redisContainer.Terminate(ctx) }()

	redisAddr, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		t.Skip("获取 Redis 连接字符串失败")
	}

	cfg := &config.Config{}
	cfg.Cache.Default = "memory"
	cfg.Cache.Stores = map[string]struct {
		Driver     string `mapstructure:"driver"`
		DefaultTTL int    `mapstructure:"default_ttl"`
		Options    any    `mapstructure:"options"`
	}{
		"memory": {
			Driver:     "memory",
			DefaultTTL: 3600,
		},
		"redis": {
			Driver:     "redis",
			DefaultTTL: 3600,
		},
	}
	cfg.Redis.Default = "test"
	cfg.Redis.Connections = map[string]config.Redis{
		"test": {
			Addr:             redisAddr,
			Username:         "",
			Password:         "",
			DB:                 0,
			DialTimeout:   5,
			ReadTimeout:    3,
			WriteTimeout:   3,
			PoolSize:          10,
			MinIdleConns:   5,
			MaxIdleConns:  10,
		},
	}

	c := cache.NewCache(ctx, cfg)

	// 测试切换到 redis 存储
	redisCache, err := c.Store("redis")
	if err != nil {
		t.Errorf("切换到 redis 存储失败: %v", err)
	}
	if redisCache == nil {
		t.Error("redis 存储应该非 nil")
	}

	// 测试再次切换返回同一个存储
	redisCache2, err := c.Store("redis")
	if err != nil {
		t.Errorf("切换到 redis 存储失败: %v", err)
	}
	if redisCache2 != redisCache {
		t.Error("同一个存储应该返回同一个实例")
	}

	// 测试切换到不存在的存储
	_, err = c.Store("non-existent")
	if err == nil {
		t.Error("不存在的存储应该返回错误")
	}
}
