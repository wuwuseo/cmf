package cache

import (
	"context"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/eko/gocache/lib/v4/cache"
	bigcachestore "github.com/eko/gocache/store/bigcache/v4"
)

type Cache[T any] struct {
	*cache.Cache[T]
}

func NewCache(ctx context.Context, defaultCacheTime time.Duration) *Cache[[]byte] {
	bigcacheClient, _ := bigcache.New(ctx, bigcache.DefaultConfig(defaultCacheTime))
	bigcacheStore := bigcachestore.NewBigcache(bigcacheClient)
	return &Cache[[]byte]{
		Cache: cache.New[[]byte](bigcacheStore),
	}
}
