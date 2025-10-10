package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/eko/gocache/lib/v4/cache"
	bigcachestore "github.com/eko/gocache/store/bigcache/v4"
	"github.com/wuwuseo/cmf/config"
	"github.com/wuwuseo/cmf/driver"
)

// CacheInterface 定义通用的缓存接口
// 这里使用泛型来支持不同类型的值
// T 表示缓存值的类型
// 注意：为了简化，这里暂时只支持字符串类型的键和默认的缓存选项
// 完整实现可以进一步扩展
type CacheInterface[T any] interface {
	Get(ctx context.Context, key string) (T, error)
	Set(ctx context.Context, key string, value T) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
}

// CacheConfig 缓存配置结构体
type CacheConfig struct {
	Driver          string        // 缓存驱动类型
	DefaultTTL      int           // 默认缓存过期时间（秒）
	Size            int           // 缓存大小
	CleanWindow     int           // 清理窗口（秒）
	HardMaxCacheSize int          // 最大缓存大小（MB）
}

// 全局缓存驱动管理器
var manager = driver.NewManager[CacheInterface[[]byte], *CacheConfig]()

// Cache 缓存结构体，实现CacheInterface接口
type Cache[T any] struct {
	*cache.Cache[T]
}

// 实现CacheInterface接口的方法
func (c *Cache[T]) Get(ctx context.Context, key string) (T, error) {
	return c.Cache.Get(ctx, key)
}

func (c *Cache[T]) Set(ctx context.Context, key string, value T) error {
	return c.Cache.Set(ctx, key, value)
}

func (c *Cache[T]) Delete(ctx context.Context, key string) error {
	return c.Cache.Delete(ctx, key)
}

func (c *Cache[T]) Clear(ctx context.Context) error {
	return c.Cache.Clear(ctx)
}

// NewCache 根据配置创建一个缓存实例
// ctx 上下文
// cfg 缓存配置
// 返回缓存实例或错误
func NewCache(ctx context.Context, cfg *CacheConfig) (CacheInterface[[]byte], error) {
	return manager.Create(cfg.Driver, cfg)
}

// NewCacheFromConfig 根据配置创建缓存实例
func NewCacheFromConfig(ctx context.Context, config *config.Config) (CacheInterface[[]byte], error) {
	// 从配置中提取缓存配置
	cacheConfig := &CacheConfig{
		Driver:          config.Cache.Driver,
		DefaultTTL:      config.Cache.DefaultTTL,
		Size:            config.Cache.Size,
		CleanWindow:     config.Cache.CleanWindow,
		HardMaxCacheSize: config.Cache.HardMaxCacheSize,
	}

	// 使用驱动管理器创建缓存实例
	cacheInstance, err := manager.Create(cacheConfig.Driver, cacheConfig)
	if err != nil {
		return nil, fmt.Errorf("创建缓存实例失败: %w", err)
	}

	return cacheInstance, nil
}

// 初始化函数，注册内置缓存驱动
func init() {
	// 注册bigcache驱动
	manager.Register("bigcache", func(cfg *CacheConfig) (CacheInterface[[]byte], error) {
		// 创建bigcache配置
		bigcacheConfig := bigcache.DefaultConfig(time.Duration(cfg.DefaultTTL) * time.Second)
		if cfg.CleanWindow > 0 {
			bigcacheConfig.CleanWindow = time.Duration(cfg.CleanWindow) * time.Second
		}
		if cfg.HardMaxCacheSize > 0 {
			bigcacheConfig.HardMaxCacheSize = cfg.HardMaxCacheSize
		}

		// 创建bigcache客户端
		ctx := context.Background()
		bigcacheClient, err := bigcache.New(ctx, bigcacheConfig)
		if err != nil {
			return nil, fmt.Errorf("创建bigcache客户端失败: %w", err)
		}

		// 创建缓存实例
		bigcacheStore := bigcachestore.NewBigcache(bigcacheClient)
		return &Cache[[]byte]{
			Cache: cache.New[[]byte](bigcacheStore),
		}, nil
	})
}
