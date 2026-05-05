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

	"github.com/gofiber/contrib/v3/swaggerui"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/wuwuseo/cmf/cache"
	"github.com/wuwuseo/cmf/config"
	"github.com/wuwuseo/cmf/filesystem"
	"github.com/wuwuseo/cmf/log"
	"go.uber.org/zap"
)

// CleanupFunc 定义清理函数的类型
type CleanupFunc func() error

// RouteRegisterFunc 定义路由注册函数的类型
type RouteRegisterFunc func(app *fiber.App, cfg *config.Config)

// InitFunc 定义初始化函数的类型
type InitFunc func(config *config.Config) error

// MiddlewareFunc 定义中间件注册函数的类型
type MiddlewareFunc func(app *fiber.App, cfg *config.Config)

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
	b := &Bootstrap{
		ctx:            context.Background(),
		cleanupFuncs:   []CleanupFunc{},
		routeRegisters: []RouteRegisterFunc{},
		initFuncs:      []InitFunc{},
	}
	// 将配置注册为服务
	b.RegisterService("config", config.Conf)
	configService, _ := b.GetService("config")
	cfg := configService.(*config.Config)

	// 初始化缓存服务
	b.RegisterService("cache", cache.NewCache(b.ctx, cfg))
	// 初始化文件系统服务
	filesystem, err := filesystem.NewFilesystemFromConfig(cfg)
	if err != nil {
		log.Fatal("文件系统初始化失败", zap.Error(err))
	}
	b.RegisterService("filesystem", filesystem)
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
	log.Info("应用启动中...",
		zap.String("program", Config.App.Name),
		zap.Int("port", Config.App.Port),
		zap.Bool("debug", Config.App.Debug),
	)

	// 构建 Fiber 服务列表，注册到 cfg.Services 以启用生命周期管理
	serviceList := b.buildServiceList(Config)

	app := fiber.New(fiber.Config{
		IdleTimeout: time.Duration(Config.App.IdleTimeout) * time.Second,
		BodyLimit:   Config.App.BodyLimit,
		ErrorHandler: func(ctx fiber.Ctx, err error) error {
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
		// 注册 Fiber v3 服务，框架自动调用 Start/Terminate
		Services: serviceList,
	})

	// 将 CMF 内部服务同步到 Fiber State，支持 fiber.GetService/MustGetService
	b.syncServicesToState(app)

	app.Use(
		recover.New(),
		compress.New(),
		requestid.New(),
	)
	b.loadMiddlewares(app)
	b.setupRoutes(app)

	// 注册 Fiber v3 Hooks 进行生命周期管理
	app.Hooks().OnPreShutdown(func() error {
		log.Warn("应用准备关闭，执行清理任务...")
		return b.cleanup()
	})

	app.Hooks().OnPostShutdown(func(err error) error {
		if err != nil {
			log.Error("应用关闭失败: " + err.Error())
		} else {
			log.Warn("Fiber 已成功关闭")
		}
		return nil
	})

	// v3 要求在 goroutine 中运行 Listen，以支持 Hooks
	go func() {
		listenAddr := ":" + fmt.Sprint(Config.App.Port)
		listenCfg := fiber.ListenConfig{
			EnablePrefork: Config.App.Prefork,
		}
		if err := app.Listen(listenAddr, listenCfg); err != nil {
			log.Fatal("监听端口失败", zap.Error(err))
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	// 触发 Fiber 优雅关闭（会依次调用 OnPreShutdown → 服务 Terminate → OnPostShutdown）
	if err := app.Shutdown(); err != nil {
		log.Error("关闭失败: " + err.Error())
	}
	return nil
}

// loadMiddlewares 加载所有注册的中间件
func (b *Bootstrap) loadMiddlewares(app *fiber.App) {
	// 从服务中获取配置
	Config := MustGetServiceTyped[*config.Config](b, "config")
	log.Info("加载中间件...")
	for _, middlewareFunc := range b.middlewareFuncs {
		middlewareFunc(app, Config)
	}
	log.Info("所有中间件加载完成")
}

// init 执行所有注册的初始化函数
func (b *Bootstrap) init() {
	// 从服务中获取配置
	Config := MustGetServiceTyped[*config.Config](b, "config")
	log.Info("执行初始化函数...")

	// 执行所有注册的初始化函数
	for _, initFunc := range b.initFuncs {
		if err := initFunc(Config); err != nil {
			log.Fatal("初始化失败", zap.Error(err))
		}
	}
	log.Info("所有初始化函数执行完成")
}

// buildServiceList 构建 Fiber v3 Service 列表，注册需要生命周期管理的服务
func (b *Bootstrap) buildServiceList(cfg *config.Config) []fiber.Service {
	var services []fiber.Service

	// 从 sync.Map 中提取所有注册的服务，包装为 fiber.Service
	b.services.Range(func(key, value any) bool {
		name := key.(string)
		services = append(services, &cmfServiceWrapper{
			name:    name,
			service: value,
		})
		return true
	})

	return services
}

// cmfServiceWrapper 将 CMF 服务包装为 fiber.Service 接口
// 提供统一的 Start / String / State / Terminate 实现
type cmfServiceWrapper struct {
	name    string
	service any
}

func (w *cmfServiceWrapper) Start(ctx context.Context) error {
	log.Info("服务启动: " + w.name)
	return nil
}

func (w *cmfServiceWrapper) String() string {
	return w.name
}

func (w *cmfServiceWrapper) State(ctx context.Context) (string, error) {
	return "running", nil
}

func (w *cmfServiceWrapper) Terminate(ctx context.Context) error {
	log.Info("服务终止: " + w.name)
	return nil
}

// syncServicesToState 将 CMF 内部服务同步到 Fiber 的 State，
// 以便在中间件/handler 中通过 app.State().Get(name) 或 c.App().State().Get(name) 检索
func (b *Bootstrap) syncServicesToState(app *fiber.App) {
	b.services.Range(func(key, value any) bool {
		name := key.(string)
		app.State().Set(name, value)
		return true
	})
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

	// 执行所有注册的路由函数
	for _, routeRegister := range b.routeRegisters {
		routeRegister(app, Config)
	}
	// 注册默认路由
	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello world! cmf!")
	})

	// Swagger UI（v3 使用 swaggerui 中间件）
	if Config.App.Swagger || Config.App.Debug {
		app.Use(swaggerui.New(swaggerui.Config{
			BasePath: "/",
			FilePath: "./docs/swagger.json",
			Path:     "docs",
		}))
	}

}
