package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/wuwuseo/cmf/config"
)

// Options 定义Redis客户端的配置选项
// 这些选项将用于创建Redis连接
type Options struct {
	Addr            string        // Redis服务器地址，格式为"host:port"
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

// NewClient 创建一个新的Redis客户端实例
// 该函数封装了go-redis的NewClient函数，提供了更便捷的使用方式
func NewClient(options *Options) *redis.Client {
	redisOptions := &redis.Options{
		Addr:            options.Addr,
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
// 该函数使用应用的全局配置来初始化Redis客户端
func NewClientFromConfig(ctx context.Context, config *config.Config) (*redis.Client, error) {
	// 从配置中获取Redis相关配置
	redisConfig := config.Redis

	// 创建选项对象，使用配置中的值
	options := &Options{
		Addr:            redisConfig.Addr,
		Password:        redisConfig.Password,
		DB:              redisConfig.DB,
		DialTimeout:     time.Duration(redisConfig.DialTimeout) * time.Second,
		ReadTimeout:     time.Duration(redisConfig.ReadTimeout) * time.Second,
		WriteTimeout:    time.Duration(redisConfig.WriteTimeout) * time.Second,
		PoolSize:        redisConfig.PoolSize,
		MinIdleConns:    redisConfig.MinIdleConns,
		MaxIdleConns:    redisConfig.MaxIdleConns,
		ConnMaxIdleTime: time.Duration(redisConfig.ConnMaxIdleTime) * time.Minute,
		ConnMaxLifetime: time.Duration(redisConfig.ConnMaxLifetime) * time.Hour,
	}

	// 如果需要TLS连接，配置TLS
	if redisConfig.UseTLS {
		options.TLSConfig = &tls.Config{}
		// 可以在这里添加更多TLS配置
	}

	// 创建客户端
	client := NewClient(options)

	// 测试连接
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("连接Redis失败: %w", err)
	}

	return client, nil
}
