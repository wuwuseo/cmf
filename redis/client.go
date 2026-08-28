package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/wuwuseo/cmf/config"
	"github.com/wuwuseo/cmf/manager"
)

// Options 定义Redis客户端的配置选项
// 这些选项将用于创建Redis连接
type Options struct {
	Addr            string        // Redis服务器地址，格式为"host:port"
	Username        string        // Redis用户名，无用户名时为空字符串
	Password        string        // Redis密码，无密码时为空字符串
	DB              int           // Redis数据库索引
	DialTimeout     time.Duration // 连接超时时间
	ReadTimeout     time.Duration // 读取超时时间
	WriteTimeout    time.Duration // 写入超时时间
	PoolSize        int           // 连接池大小
	MinIdleConns    int           // 最小空闲连接数
	MaxIdleConns    int           // 最大空闲连接数
	ConnMaxIdleTime time.Duration // 连接最大空闲时间
	ConnMaxLifetime time.Duration // 连接最大生命周期
	TLSConfig       *tls.Config   // TLS配置，用于加密连接
}

// 使用 per-config 的 Manager 管理按名创建的 Redis 客户端
// 以 *config.Config 指针为 key：不同配置（如测试用例）各自持有一组客户端实例，避免连接串扰
// ponytail: 以 cfg 指针为 key，天花板是同一配置的多个副本各建一套客户端；升级路径：按连接参数指纹缓存
var managers sync.Map // *config.Config -> *manager.Manager[*redis.Client, config.Redis]

// NewClient 创建一个新的Redis客户端实例
// 该函数封装了go-redis的NewClient函数，提供了更便捷的使用方式
func NewClient(options *Options) *redis.Client {
	redisOptions := &redis.Options{
		Addr:            options.Addr,
		Username:        options.Username,
		Password:        options.Password,
		DB:              options.DB,
		DialTimeout:     options.DialTimeout,
		ReadTimeout:     options.ReadTimeout,
		WriteTimeout:    options.WriteTimeout,
		PoolSize:        options.PoolSize,
		MinIdleConns:    options.MinIdleConns,
		MaxIdleConns:    options.MaxIdleConns,
		ConnMaxIdleTime: options.ConnMaxIdleTime,
		ConnMaxLifetime: options.ConnMaxLifetime,
		TLSConfig:       options.TLSConfig,
	}
	return redis.NewClient(redisOptions)
}

// NewClientFromConfig 从配置对象创建Redis客户端实例
// storeName 指定 redis.connections 中的连接名（不传 = 默认连接），首次访问惰性创建并验证连接，
// 之后返回缓存的同一客户端。配置多个 connections 时可按名切换不同 Redis 实例。
func NewClientFromConfig(ctx context.Context, cfg *config.Config, storeName ...string) (*redis.Client, error) {
	def := cfg.Redis.Default
	if def == "" {
		def = "redis"
	}

	mAny, _ := managers.LoadOrStore(cfg, manager.New[*redis.Client, config.Redis](
		def,
		func() map[string]config.Redis { return cfg.Redis.Connections },
		func(name string, rc config.Redis) (*redis.Client, error) {
			options := &Options{
				Addr:            rc.Addr,
				Username:         rc.Username,
				Password:         rc.Password,
				DB:               rc.DB,
				DialTimeout:      time.Duration(rc.DialTimeout) * time.Second,
				ReadTimeout:      time.Duration(rc.ReadTimeout) * time.Second,
				WriteTimeout:     time.Duration(rc.WriteTimeout) * time.Second,
				PoolSize:         rc.PoolSize,
				MinIdleConns:     rc.MinIdleConns,
				MaxIdleConns:     rc.MaxIdleConns,
				ConnMaxIdleTime:  time.Duration(rc.ConnMaxIdleTime) * time.Minute,
				ConnMaxLifetime:  time.Duration(rc.ConnMaxLifetime) * time.Hour,
			}

			// 如果需要TLS连接，配置TLS
			if rc.UseTLS {
				options.TLSConfig = &tls.Config{}
				// 可以在这里添加更多TLS配置
			}

			client := NewClient(options)

			// 验证连接：失败时关闭客户端并返回错误（Manager 不会缓存失败的实例）
			// ponytail: Ping 固定用 context.Background()，天花板是工厂闭包无法接收调用方 ctx；升级路径：Manager.Get 增加 ctx 透传
			if err := client.Ping(context.Background()).Err(); err != nil {
				client.Close()
				return nil, fmt.Errorf("连接Redis失败: %w", err)
			}
			return client, nil
		},
	))
	m := mAny.(*manager.Manager[*redis.Client, config.Redis])

	return m.Get(storeName...)
}
