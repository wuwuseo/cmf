package driver

import (
	"context"
	"time"

	"github.com/allegro/bigcache/v3"
	gostore "github.com/eko/gocache/lib/v4/store"
	bigcachestore "github.com/eko/gocache/store/bigcache/v4"
	"github.com/wuwuseo/cmf/config"
)

func NewBigCache(ctx context.Context, cfg *config.Config) gostore.StoreInterface {
	// 获取默认缓存存储配置
	defaultStore := cfg.Cache.Default
	storeConfig := cfg.Cache.Stores[defaultStore]
	
	bigcacheClient, _ := bigcache.New(ctx, bigcache.DefaultConfig(time.Duration(storeConfig.DefaultTTL)*time.Second))
	return bigcachestore.NewBigcache(bigcacheClient)
}
