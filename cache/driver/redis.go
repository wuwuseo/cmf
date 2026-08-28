package driver

import (
	"context"
	"fmt"
	"time"

	gostore "github.com/eko/gocache/lib/v4/store"
	redisstore "github.com/eko/gocache/store/redis/v4"
	"github.com/wuwuseo/cmf/config"
	"github.com/wuwuseo/cmf/redis"
)

// NewRedisCache 创建 Redis 缓存存储
// storeName 指定要读取的缓存存储配置名（不传 = 默认存储），TTL 取自该配置的 default_ttl
// ponytail: Redis 连接固定使用 redis.default 配置；如需指定连接，可扩展 cache.stores.<name>.options.connection
func NewRedisCache(ctx context.Context, cfg *config.Config, storeName ...string) (gostore.StoreInterface, error) {
	name := cfg.Cache.Default
	if len(storeName) > 0 && storeName[0] != "" {
		name = storeName[0]
	}

	storeConfig, ok := cfg.Cache.Stores[name]
	if !ok {
		return nil, fmt.Errorf("缓存存储配置 '%s' 不存在", name)
	}

	client, err := redis.NewClientFromConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("初始化 Redis 缓存失败: %w", err)
	}

	return redisstore.NewRedis(client, gostore.WithExpiration(time.Duration(storeConfig.DefaultTTL)*time.Second)), nil
}
