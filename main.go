package main

import (
	"context"
	"time"

	fiberlog "github.com/gofiber/fiber/v2/log"
	"github.com/wuwuseo/cmf/bootstrap"
	"github.com/wuwuseo/cmf/cache"
)

func main() {
	// 创建 Bootstrap 实例
	b := bootstrap.NewBootstrap()

	// 创建一个 goroutine 在2秒后打印一些信息，帮助我们确认应用正在运行
	go func() {
		time.Sleep(2 * time.Second)
		cacheService, _ := b.GetService("cache")
		rawCache := cacheService.(*cache.Cache[[]byte])
		// 使用NewTypedCache函数创建一个string类型的缓存包装器
		stringCache := cache.NewTypedCache[string](rawCache)
		ctx := context.Background()
		stringCache.Set(ctx, "test", "hello cache")
		val, err := stringCache.Get(ctx, "test")
		if err != nil {
			fiberlog.Errorf("缓存获取失败: %v", err)
		} else {
			fiberlog.Infof("缓存值: %v", val)
		}
		fiberlog.Infof("应用已成功运行，请按 Ctrl+C 测试优雅关闭和日志输出")
	}()

	// 运行应用
	if err := b.Run(); err != nil {
		fiberlog.Infof("应用运行出错: %v\n", err)
	}
}
