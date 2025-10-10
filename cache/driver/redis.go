package driver

import (
	"context"
	"time"

	gostore "github.com/eko/gocache/lib/v4/store"
	redisstore "github.com/eko/gocache/store/redis/v4"
	"github.com/wuwuseo/cmf/config"
	"github.com/wuwuseo/cmf/redis"
)

func NewRedisCache(ctx context.Context, cfg *config.Config) gostore.StoreInterface {
	client, err := redis.NewClientFromConfig(ctx, cfg)
	if err != nil {
		panic(err)
	}
	return redisstore.NewRedis(client, gostore.WithExpiration(time.Duration(cfg.Cache.DefaultTTL)))
}
