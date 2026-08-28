package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/eko/gocache/lib/v4/cache"
	gostore "github.com/eko/gocache/lib/v4/store"
	"github.com/google/wire"
	"github.com/wuwuseo/cmf/cache/driver"
	"github.com/wuwuseo/cmf/config"
	"github.com/wuwuseo/cmf/manager"
)

// ProviderSet 缓存模块的 Wire Provider 集合
var ProviderSet = wire.NewSet(NewCache)

type Cache[T any] struct {
	ctx context.Context
	*cache.Cache[T]
	cfg      *config.Config
	mgr      *manager.Manager[gostore.StoreInterface, config.StoreConfig]
	storeKey string // 当前存储的键
}

// NewCache 创建一个缓存实例，默认存储[]byte类型的数据
func NewCache(ctx context.Context, cfg *config.Config) *Cache[[]byte] {
	// 获取默认缓存存储配置
	defaultStoreName := cfg.Cache.Default
	if defaultStoreName == "" {
		defaultStoreName = "memory"
	}

	mgr := manager.New[gostore.StoreInterface, config.StoreConfig](
		defaultStoreName,
		func() map[string]config.StoreConfig { return cfg.Cache.Stores },
		func(name string, sc config.StoreConfig) (gostore.StoreInterface, error) {
			switch sc.Driver {
			case "redis":
				return driver.NewRedisCache(ctx, cfg, name)
			case "memory":
				return driver.NewBigCache(ctx, cfg, name)
			default:
				return nil, fmt.Errorf("不支持的缓存驱动: %s", sc.Driver)
			}
		},
	)

	return &Cache[[]byte]{
		ctx:      ctx,
		Cache:    cache.New[[]byte](mgr.MustGet()),
		cfg:      cfg,
		mgr:      mgr,
		storeKey: defaultStoreName,
	}
}

// Store 切换到指定名称的缓存存储
// 首次访问惰性创建驱动实例（按该名称的配置初始化），之后返回缓存的同一驱动
// 返回的实例与原实例共享配置，但读写互不影响
func (c *Cache[T]) Store(storeName string) (*Cache[T], error) {
	store, err := c.mgr.Get(storeName)
	if err != nil {
		return nil, err
	}

	return &Cache[T]{
		ctx:      c.ctx,
		Cache:    cache.New[T](store),
		cfg:      c.cfg,
		mgr:      c.mgr,
		storeKey: storeName,
	}, nil
}

// StoreKey 返回当前实例使用的存储名
func (c *Cache[T]) StoreKey() string {
	return c.storeKey
}

// Close 关闭所有已创建的缓存驱动实例（bigcache/redis 驱动均支持 Close）
func (c *Cache[T]) Close() error {
	return c.mgr.Close()
}

// TypedCache 提供类型安全的缓存操作
// 通过JSON序列化和反序列化支持任意类型的数据
type TypedCache[T any] struct {
	rawCache *Cache[[]byte]
}

// NewTypedCache 创建一个指定类型的缓存实例
func NewTypedCache[T any](rawCache *Cache[[]byte]) *TypedCache[T] {
	return &TypedCache[T]{
		rawCache: rawCache,
	}
}

// Get 获取缓存中的值
func (tc *TypedCache[T]) Get(ctx context.Context, key string) (T, error) {
	// 获取原始的[]byte数据
	data, err := tc.rawCache.Get(ctx, key)
	if err != nil {
		var zero T
		return zero, err
	}

	// 将[]byte数据反序列化为目标类型
	var value T
	err = json.Unmarshal(data, &value)
	return value, err
}

// Set 设置缓存值
func (tc *TypedCache[T]) Set(ctx context.Context, key string, value T) error {
	// 将目标类型序列化为[]byte
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	// 存储[]byte数据到原始缓存
	return tc.rawCache.Set(ctx, key, data)
}

// SetWithExpiration 设置带过期时间的缓存值
func (tc *TypedCache[T]) SetWithExpiration(ctx context.Context, key string, value T, ttl time.Duration) error {
	// 将目标类型序列化为[]byte
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	// 存储[]byte数据到原始缓存，带过期时间
	return tc.rawCache.Set(ctx, key, data, gostore.WithExpiration(ttl))
}

// Delete 删除缓存中的值
func (tc *TypedCache[T]) Delete(ctx context.Context, key string) error {
	return tc.rawCache.Delete(ctx, key)
}

// Clear 清空所有缓存
func (tc *TypedCache[T]) Clear(ctx context.Context) error {
	return tc.rawCache.Clear(ctx)
}
