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
	bigcacheClient, _ := bigcache.New(ctx, bigcache.DefaultConfig(time.Duration(cfg.Cache.DefaultTTL)))
	return bigcachestore.NewBigcache(bigcacheClient)
}
