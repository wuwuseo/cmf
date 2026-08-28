package driver

import (
	"context"
	"fmt"
	"time"

	"github.com/allegro/bigcache/v3"
	gostore "github.com/eko/gocache/lib/v4/store"
	bigcachestore "github.com/eko/gocache/store/bigcache/v4"
	"github.com/wuwuseo/cmf/config"
)

// NewBigCache 创建内存缓存存储
// storeName 指定要读取的缓存存储配置名（不传 = 默认存储），TTL 取自该配置的 default_ttl
func NewBigCache(ctx context.Context, cfg *config.Config, storeName ...string) (gostore.StoreInterface, error) {
	name := cfg.Cache.Default
	if len(storeName) > 0 && storeName[0] != "" {
		name = storeName[0]
	}

	storeConfig, ok := cfg.Cache.Stores[name]
	if !ok {
		return nil, fmt.Errorf("缓存存储配置 '%s' 不存在", name)
	}

	bigcacheClient, err := bigcache.New(ctx, bigcache.DefaultConfig(time.Duration(storeConfig.DefaultTTL)*time.Second))
	if err != nil {
		return nil, fmt.Errorf("创建 bigcache 实例失败: %w", err)
	}
	return bigcachestore.NewBigcache(bigcacheClient), nil
}
