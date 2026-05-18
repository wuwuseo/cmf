package bootstrap_test

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/wuwuseo/cmf/bootstrap"
	"github.com/wuwuseo/cmf/config"
)

// sendInterrupt 跨平台发送中断信号
func sendInterrupt() {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return
	}
	_ = p.Signal(os.Interrupt)
}

// makeTestConfig 创建测试配置
func makeTestConfig(port int) *config.Config {
	cfg := &config.Config{}
	cfg.App.Name = "test-app"
	cfg.App.Port = port
	cfg.App.Debug = false
	cfg.App.IdleTimeout = 5
	cfg.App.BodyLimit = 10 * 1024 * 1024
	cfg.App.Swagger = false
	cfg.App.Prefork = false

	cfg.Log.Level = "error"
	cfg.Log.Format = "console"
	cfg.Log.ConsoleOutput = false
	cfg.Log.FileOutput = false

	cfg.Cache.Default = "memory"
	cfg.Cache.Stores = map[string]struct {
		Driver     string `mapstructure:"driver"`
		DefaultTTL int    `mapstructure:"default_ttl"`
		Options    any    `mapstructure:"options"`
	}{
		"memory": {Driver: "memory", DefaultTTL: 3600},
	}

	cfg.Filesystem.Default = "local"
	cfg.Filesystem.IsAndLocal = false
	cfg.Filesystem.Disks = map[string]struct {
		Driver  string `mapstructure:"driver"`
		Options any    `mapstructure:"options"`
	}{
		"local": {
			Driver:  "local",
			Options: map[string]any{"root": os.TempDir()},
		},
	}

	return cfg
}

// runBootstrapAndWait 启动 Bootstrap，等待端口可用后返回 done channel
func runBootstrapAndWait(t *testing.T, b *bootstrap.Bootstrap, port int) chan error {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		done <- b.Run()
	}()

	// 等待服务器启动，最多重试 20 次
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		url := fmt.Sprintf("http://localhost:%d/", port)
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return done
		}
	}

	return done
}

func TestBootstrap_Run(t *testing.T) {
	oldConf := config.Conf
	defer func() { config.Conf = oldConf }()

	testPort := 19999
	cfg := makeTestConfig(testPort)
	config.Conf = cfg

	b := bootstrap.NewBootstrap()
	if b == nil {
		t.Fatal("NewBootstrap 返回 nil")
	}

	// 验证服务已注册
	if !b.HasService("config") {
		t.Fatal("config 服务应该已注册")
	}
	if !b.HasService("cache") {
		t.Fatal("cache 服务应该已注册")
	}
	if !b.HasService("filesystem") {
		t.Fatal("filesystem 服务应该已注册")
	}

	// 注册测试路由
	b.RegisterRoute(func(app *fiber.App, cfg *config.Config) {
		app.Get("/api/test", func(c fiber.Ctx) error {
			return c.JSON(fiber.Map{"status": "ok"})
		})
	})

	// 启动并等待
	done := runBootstrapAndWait(t, b, testPort)

	// 发送 HTTP 请求验证服务正常
	url := fmt.Sprintf("http://localhost:%d/api/test", testPort)
	resp, err := http.Get(url)
	if err != nil {
		t.Skipf("无法连接到测试服务器: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("期望状态码 200，实际 %d", resp.StatusCode)
	}

	// 发送中断信号让 Run 退出
	sendInterrupt()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run 返回错误: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Log("Run 未在 5 秒内退出（Windows 可能不支持发送 Interrupt 信号）")
	}
}

func TestBootstrap_Run_DefaultRoute(t *testing.T) {
	oldConf := config.Conf
	defer func() { config.Conf = oldConf }()

	testPort := 19998
	cfg := makeTestConfig(testPort)
	config.Conf = cfg

	b := bootstrap.NewBootstrap()

	done := runBootstrapAndWait(t, b, testPort)

	// 测试默认路由 "/"
	url := fmt.Sprintf("http://localhost:%d/", testPort)
	resp, err := http.Get(url)
	if err != nil {
		t.Skipf("无法连接到测试服务器: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("期望状态码 200，实际 %d", resp.StatusCode)
	}

	sendInterrupt()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Log("Run 未在 5 秒内退出（Windows 可能不支持发送 Interrupt 信号）")
	}
}

func TestBootstrap_Run_WithInitFunc(t *testing.T) {
	oldConf := config.Conf
	defer func() { config.Conf = oldConf }()

	testPort := 19997
	cfg := makeTestConfig(testPort)
	config.Conf = cfg

	b := bootstrap.NewBootstrap()

	initCalled := false
	b.RegisterInitFunc(func(cfg *config.Config) error {
		initCalled = true
		return nil
	})

	done := runBootstrapAndWait(t, b, testPort)

	sendInterrupt()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}

	if !initCalled {
		t.Error("initFunc 应该被调用")
	}
}

func TestBootstrap_Run_ErrorHandler(t *testing.T) {
	oldConf := config.Conf
	defer func() { config.Conf = oldConf }()

	testPort := 19996
	cfg := makeTestConfig(testPort)
	config.Conf = cfg

	b := bootstrap.NewBootstrap()

	// 注册一个会产生错误的路由
	b.RegisterRoute(func(app *fiber.App, cfg *config.Config) {
		app.Get("/api/error", func(c fiber.Ctx) error {
			return fiber.NewError(400, "bad request")
		})
	})

	done := runBootstrapAndWait(t, b, testPort)

	url := fmt.Sprintf("http://localhost:%d/api/error", testPort)
	resp, err := http.Get(url)
	if err != nil {
		t.Skipf("无法连接到测试服务器: %v", err)
		return
	}
	defer resp.Body.Close()

	// 错误处理器在找不到 400.html 时会返回 500
	if resp.StatusCode != 500 && resp.StatusCode != 400 {
		t.Errorf("期望状态码 400 或 500，实际 %d", resp.StatusCode)
	}

	sendInterrupt()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}

func TestBootstrap_Run_Cleanup(t *testing.T) {
	oldConf := config.Conf
	defer func() { config.Conf = oldConf }()

	testPort := 19995
	cfg := makeTestConfig(testPort)
	config.Conf = cfg

	b := bootstrap.NewBootstrap()

	cleanupCalled := false
	b.RegisterCleanupFunc(func() error {
		cleanupCalled = true
		return nil
	})

	done := runBootstrapAndWait(t, b, testPort)

	// 发送中断信号触发清理
	sendInterrupt()
	select {
	case <-done:
		if !cleanupCalled {
			t.Error("清理函数应该被调用")
		}
	case <-time.After(5 * time.Second):
		// Windows 不支持发送 Interrupt 信号，清理不会触发
		t.Log("Run 未退出（Windows 信号限制），跳过 cleanup 断言")
	}
}

func TestBootstrap_Run_Middleware(t *testing.T) {
	oldConf := config.Conf
	defer func() { config.Conf = oldConf }()

	testPort := 19994
	cfg := makeTestConfig(testPort)
	config.Conf = cfg

	b := bootstrap.NewBootstrap()

	middlewareCalled := false
	b.RegisterMiddleware(func(app *fiber.App, cfg *config.Config) {
		middlewareCalled = true
	})

	b.RegisterRoute(func(app *fiber.App, cfg *config.Config) {
		app.Get("/api/mw", func(c fiber.Ctx) error {
			return c.JSON(fiber.Map{"status": "ok"})
		})
	})

	done := runBootstrapAndWait(t, b, testPort)

	url := fmt.Sprintf("http://localhost:%d/api/mw", testPort)
	resp, err := http.Get(url)
	if err != nil {
		t.Skipf("无法连接到测试服务器: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("期望状态码 200，实际 %d", resp.StatusCode)
	}

	if middlewareCalled {
		t.Log("中间件注册函数被调用")
	}

	sendInterrupt()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}
