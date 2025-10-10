package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	fiberlog "github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/swagger"
	"github.com/wuwuseo/cmf/cache"
	"github.com/wuwuseo/cmf/config"
	"github.com/wuwuseo/cmf/log"
)

// CleanupFunc 定义清理函数的类型
type CleanupFunc func() error

// RouteRegisterFunc 定义路由注册函数的类型
type RouteRegisterFunc func(app *fiber.App, config *config.Config)

// InitFunc 定义初始化函数的类型
type InitFunc func(config *config.Config) error

// MiddlewareFunc 定义中间件注册函数的类型
type MiddlewareFunc func(app *fiber.App, config *config.Config)

// Bootstrap 应用引导程序
type Bootstrap struct {
	ctx             context.Context
	cleanupFuncs    []CleanupFunc
	routeRegisters  []RouteRegisterFunc
	initFuncs       []InitFunc
	middlewareFuncs []MiddlewareFunc
	services        sync.Map // 使用sync.Map保证并发安全
}

func NewBootstrap() *Bootstrap {
	config.InitConfig()

	b := &Bootstrap{
		ctx:            context.Background(),
		cleanupFuncs:   []CleanupFunc{},
		routeRegisters: []RouteRegisterFunc{},
		initFuncs:      []InitFunc{},
	}
	// 将配置注册为服务
	b.RegisterService("config", config.NewConfig())
	configService, _ := b.GetService("config")
	b.RegisterService("cache", cache.NewCache(b.ctx, configService.(*config.Config)))
	return b
}

// RegisterCleanupFunc 注册清理函数，供外部包调用
func (b *Bootstrap) RegisterCleanupFunc(f CleanupFunc) {
	b.cleanupFuncs = append(b.cleanupFuncs, f)
}

// RegisterRoute 注册路由函数，供外部包调用
func (b *Bootstrap) RegisterRoute(f RouteRegisterFunc) {
	b.routeRegisters = append(b.routeRegisters, f)
}

// RegisterInitFunc 注册初始化函数，供外部包调用
func (b *Bootstrap) RegisterInitFunc(f InitFunc) {
	b.initFuncs = append(b.initFuncs, f)
}

// RegisterMiddleware 注册中间件函数，供外部包调用
func (b *Bootstrap) RegisterMiddleware(f MiddlewareFunc) {
	b.middlewareFuncs = append(b.middlewareFuncs, f)
}

// RegisterService 注册服务实例到容器中（单例模式）
func (b *Bootstrap) RegisterService(name string, service any) {
	b.services.Store(name, service)
}

// GetService 从容器中获取服务实例
// 如果服务不存在，返回nil和false
func (b *Bootstrap) GetService(name string) (any, bool) {
	service, exists := b.services.Load(name)
	return service, exists
}

// GetServiceTyped 从容器中获取指定类型的服务实例
// 提供类型安全的服务获取，使用泛型
func GetServiceTyped[T any](b *Bootstrap, name string) (T, bool) {
	service, exists := b.services.Load(name)
	if !exists {
		var zero T
		return zero, false
	}

	typedService, ok := service.(T)
	if !ok {
		var zero T
		return zero, false
	}

	return typedService, true
}

// MustGetService 从容器中获取服务实例，如果服务不存在则panic
// 适用于必须依赖该服务的场景
func (b *Bootstrap) MustGetService(name string) any {
	service, exists := b.services.Load(name)
	if !exists {
		panic(fmt.Sprintf("服务 '%s' 未注册", name))
	}
	return service
}

// MustGetServiceTyped 从容器中获取指定类型的服务实例，如果服务不存在或类型不匹配则panic
func MustGetServiceTyped[T any](b *Bootstrap, name string) T {
	service, exists := b.services.Load(name)
	if !exists {
		panic(fmt.Sprintf("服务 '%s' 未注册", name))
	}

	typedService, ok := service.(T)
	if !ok {
		panic(fmt.Sprintf("服务 '%s' 的类型与请求的类型不匹配", name))
	}

	return typedService
}

// HasService 检查服务是否已注册
func (b *Bootstrap) HasService(name string) bool {
	_, exists := b.services.Load(name)
	return exists
}

// RemoveService 从容器中移除服务（谨慎使用）
// 注意：单例模式下通常不建议移除服务，但在某些特殊场景可能有用
func (b *Bootstrap) RemoveService(name string) {
	b.services.Delete(name)
}

func (b *Bootstrap) Run() error {

	b.init()
	// 从服务中获取配置
	Config := MustGetServiceTyped[*config.Config](b, "config")
	// 记录应用启动信息
	fiberlog.Infof("应用启动中... 程序：%s 端口：%d (Debug: %v)",
		Config.App.Name,
		Config.App.Port,
		Config.App.Debug)

	app := fiber.New(fiber.Config{
		IdleTimeout: time.Duration(Config.App.IdleTimeout) * time.Second,
		ErrorHandler: func(ctx *fiber.Ctx, err error) error {
			// Status code defaults to 500
			code := fiber.StatusInternalServerError

			// Retrieve the custom status code if it's a *fiber.Error
			var e *fiber.Error
			if errors.As(err, &e) {
				code = e.Code
			}

			// Send custom error page
			err = ctx.Status(code).SendFile(fmt.Sprintf("./%d.html", code))
			if err != nil {
				// In case the SendFile fails
				return ctx.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
			}

			// Return from handler
			return nil
		},
	})
	app.Use(
		recover.New(),
		logger.New(),
		requestid.New(),
	)
	b.loadMiddlewares(app)
	b.setupRoutes(app)

	// Listen from a different goroutine
	go func() {
		if err := app.Listen(":" + fmt.Sprint(Config.App.Port)); err != nil {
			fiberlog.Panic(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	// 添加一些调试日志，验证日志是否正常工作
	fiberlog.Warn("Running cleanup tasks...")
	if err := b.cleanup(); err != nil {
		fiberlog.Error("Cleanup failed: " + err.Error())
	}
	fiberlog.Warn("Fiber was successful shutdown.")
	return nil
}

// loadMiddlewares 加载所有注册的中间件
func (b *Bootstrap) loadMiddlewares(app *fiber.App) {
	// 从服务中获取配置
	Config := MustGetServiceTyped[*config.Config](b, "config")
	fiberlog.Info("加载中间件...")
	for _, middlewareFunc := range b.middlewareFuncs {
		middlewareFunc(app, Config)
	}
	fiberlog.Info("所有中间件加载完成")
}

// init 执行所有注册的初始化函数
func (b *Bootstrap) init() {
	// 从服务中获取配置
	Config := MustGetServiceTyped[*config.Config](b, "config")
	log.InitDefaultLogger(Config)
	fiberlog.Info("执行初始化函数...")
	// 执行所有注册的初始化函数
	for _, initFunc := range b.initFuncs {
		if err := initFunc(Config); err != nil {
			fiberlog.Fatalf("初始化失败: %v", err)
		}
	}
	fiberlog.Info("所有初始化函数执行完成")
}

/**
 * @description: 清理资源
 * @return {*}
 */
func (b *Bootstrap) cleanup() error {
	// 执行所有注册的清理函数
	for _, cleanupFunc := range b.cleanupFuncs {
		if err := cleanupFunc(); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bootstrap) setupRoutes(app *fiber.App) {
	// 从服务中获取配置
	Config := MustGetServiceTyped[*config.Config](b, "config")
	// 注册默认路由
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello world!")
	})
	//swagger
	if Config.App.Swagger || Config.App.Debug {
		app.Get("/swagger/*", swagger.HandlerDefault)
	}

	// 执行所有注册的路由函数
	for _, routeRegister := range b.routeRegisters {
		routeRegister(app, Config)
	}
}
