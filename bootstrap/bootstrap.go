package bootstrap

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/wuwuseo/cmf/config"
)

// CleanupFunc 定义清理函数的类型
type CleanupFunc func() error

// RouteRegisterFunc 定义路由注册函数的类型
type RouteRegisterFunc func(app *fiber.App, config *config.Config)

// Bootstrap 应用引导程序
type Bootstrap struct {
	Config         *config.Config
	cleanupFuncs   []CleanupFunc
	routeRegisters []RouteRegisterFunc
}

func NewBootstrap() *Bootstrap {
	config.InitConfig()
	return &Bootstrap{
		Config:         config.NewConfig(),
		cleanupFuncs:   []CleanupFunc{},
		routeRegisters: []RouteRegisterFunc{},
	}
}

// RegisterCleanupFunc 注册清理函数，供外部包调用
func (b *Bootstrap) RegisterCleanupFunc(f CleanupFunc) {
	b.cleanupFuncs = append(b.cleanupFuncs, f)
}

// RegisterRoute 注册路由函数，供外部包调用
func (b *Bootstrap) RegisterRoute(f RouteRegisterFunc) {
	b.routeRegisters = append(b.routeRegisters, f)
}

func (b *Bootstrap) Run() error {
	app := fiber.New(fiber.Config{
		IdleTimeout: time.Duration(b.Config.App.IdleTimeout) * time.Second,
	})
	b.setupRoutes(app)

	// Listen from a different goroutine
	go func() {
		if err := app.Listen(":" + fmt.Sprint(b.Config.App.Port)); err != nil {
			log.Panic(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	log.Warnf("Gracefully shutting down...")
	_ = app.Shutdown()

	log.Warnf("Running cleanup tasks...")
	if err := b.cleanup(); err != nil {
		log.Errorf("Cleanup failed: %v", err)
	}
	log.Warnf("Fiber was successful shutdown.")
	return nil
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
	// 注册默认路由
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello world!")
	})

	// 执行所有注册的路由函数
	for _, routeRegister := range b.routeRegisters {
		routeRegister(app, b.Config)
	}
}
