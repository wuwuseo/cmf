package queue_test

import (
	"context"
	"time"

	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

// runRedisContainer 尝试启动一个用于测试的 Redis 容器，返回连接地址。
// Docker 环境不可用时返回错误（调用方据此跳过测试）。
func runRedisContainer() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	container, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		return "", err
	}
	return container.ConnectionString(ctx)
}
