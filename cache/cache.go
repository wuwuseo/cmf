package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/eko/gocache/lib/v4/cache"
	gostore "github.com/eko/gocache/lib/v4/store"
	"github.com/wuwuseo/cmf/cache/driver"
	"github.com/wuwuseo/cmf/config"
)

type Cache[T any] struct {
	ctx context.Context
	*cache.Cache[T]
	cfg      *config.Config
	stores   *sync.Map // 用于存储不同存储类型的缓存实例
	storeKey string    // 当前存储的键
}

// NewCache 创建一个缓存实例，默认存储[]byte类型的数据
func NewCache(ctx context.Context, cfg *config.Config) *Cache[[]byte] {
	// 获取默认缓存存储配置
	defaultStoreName := cfg.Cache.Default
	storeConfig, exists := cfg.Cache.Stores[defaultStoreName]
	if !exists {
		panic("cache driver not found")
	}

	var store gostore.StoreInterface
	switch storeConfig.Driver {
	case "redis":
		store = driver.NewRedisCache(ctx, cfg)

	case "memory":
		store = driver.NewBigCache(ctx, cfg)
	default:
		panic("cache driver not found")
	}

	cacheInstance := &Cache[[]byte]{
		ctx:      ctx,
		Cache:    cache.New[[]byte](store),
		cfg:      cfg,
		stores:   &sync.Map{},
		storeKey: defaultStoreName,
	}

	// 将默认存储实例存储到sync.Map中
	cacheInstance.stores.Store(defaultStoreName, cacheInstance)

	return cacheInstance
}

// Store 切换到指定名称的缓存存储
func (c *Cache[T]) Store(storeName string) (*Cache[T], error) {
	// 首先检查是否已经存在该存储实例
	if store, ok := c.stores.Load(storeName); ok {
		return store.(*Cache[T]), nil
	}

	// 获取指定的缓存存储配置
	storeConfig, exists := c.cfg.Cache.Stores[storeName]
	if !exists {
		return nil, fmt.Errorf("cache store '%s' not found", storeName)
	}

	var store gostore.StoreInterface
	switch storeConfig.Driver {
	case "redis":
		store = driver.NewRedisCache(c.ctx, c.cfg)

	case "memory":
		store = driver.NewBigCache(c.ctx, c.cfg)
	default:
		return nil, fmt.Errorf("unsupported cache driver: %s", storeConfig.Driver)
	}

	cacheInstance := &Cache[T]{
		ctx:      c.ctx,
		Cache:    cache.New[T](store),
		cfg:      c.cfg,
		stores:   c.stores, // 共享同一个sync.Map
		storeKey: storeName,
	}

	// 将新创建的存储实例存储到sync.Map中
	c.stores.Store(storeName, cacheInstance)

	return cacheInstance, nil
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

// Delete 删除缓存中的值
func (tc *TypedCache[T]) Delete(ctx context.Context, key string) error {
	return tc.rawCache.Delete(ctx, key)
}

// Clear 清空所有缓存
func (tc *TypedCache[T]) Clear(ctx context.Context) error {
	return tc.rawCache.Clear(ctx)
}
