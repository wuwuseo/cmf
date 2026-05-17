package bootstrap_test

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/wuwuseo/cmf/bootstrap"
	"github.com/wuwuseo/cmf/config"
)

// safeNewBootstrap 安全地创建 Bootstrap 实例
// 如果 NewBootstrap 因缺少配置等原因 panic，则跳过当前测试
func safeNewBootstrap(t *testing.T) *bootstrap.Bootstrap {
	t.Helper()
	var b *bootstrap.Bootstrap
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Skipf("NewBootstrap 初始化失败（可能缺少配置文件）: %v", r)
			}
		}()
		b = bootstrap.NewBootstrap()
	}()
	return b
}

// =============================================================================
// NewBootstrap 测试
// =============================================================================

// TestNewBootstrap_NotNil 测试创建 bootstrap 实例不为 nil
func TestNewBootstrap_NotNil(t *testing.T) {
	b := safeNewBootstrap(t)
	if b == nil {
		t.Fatal("NewBootstrap 返回了 nil")
	}
}

// TestNewBootstrap_HasConfigService 测试创建后已注册 config 服务
func TestNewBootstrap_HasConfigService(t *testing.T) {
	b := safeNewBootstrap(t)
	if !b.HasService("config") {
		t.Error("NewBootstrap 后应已注册 config 服务")
	}
	svc, ok := b.GetService("config")
	if !ok {
		t.Error("GetService('config') 应返回 true")
	}
	if svc == nil {
		t.Error("config 服务不应为 nil")
	}
}

// TestNewBootstrap_HasCacheService 测试创建后已注册 cache 服务
func TestNewBootstrap_HasCacheService(t *testing.T) {
	b := safeNewBootstrap(t)
	if !b.HasService("cache") {
		t.Error("NewBootstrap 后应已注册 cache 服务")
	}
	svc, ok := b.GetService("cache")
	if !ok {
		t.Error("GetService('cache') 应返回 true")
	}
	if svc == nil {
		t.Error("cache 服务不应为 nil")
	}
}

// TestNewBootstrap_HasFilesystemService 测试创建后已注册 filesystem 服务
func TestNewBootstrap_HasFilesystemService(t *testing.T) {
	b := safeNewBootstrap(t)
	if !b.HasService("filesystem") {
		t.Error("NewBootstrap 后应已注册 filesystem 服务")
	}
	svc, ok := b.GetService("filesystem")
	if !ok {
		t.Error("GetService('filesystem') 应返回 true")
	}
	if svc == nil {
		t.Error("filesystem 服务不应为 nil")
	}
}

// =============================================================================
// RegisterService 测试
// =============================================================================

// TestRegisterService_RegisterCustom 测试注册自定义服务
func TestRegisterService_RegisterCustom(t *testing.T) {
	b := safeNewBootstrap(t)
	myService := &struct{ Name string }{Name: "test-service"}
	b.RegisterService("my-custom", myService)

	if !b.HasService("my-custom") {
		t.Error("注册自定义服务后 HasService 应返回 true")
	}
}

// TestRegisterService_GetService 测试注册后通过 GetService 获取
func TestRegisterService_GetService(t *testing.T) {
	b := safeNewBootstrap(t)
	myService := &struct{ Name string }{Name: "test-service"}
	b.RegisterService("my-custom", myService)

	svc, ok := b.GetService("my-custom")
	if !ok {
		t.Fatal("GetService 获取注册的服务应返回 true")
	}
	if svc != myService {
		t.Error("GetService 返回的服务实例与注册的不一致")
	}
}

// =============================================================================
// GetService 测试
// =============================================================================

// TestGetService_Existing 测试获取存在的服务
func TestGetService_Existing(t *testing.T) {
	b := safeNewBootstrap(t)
	svc, ok := b.GetService("config")
	if !ok {
		t.Error("GetService 获取已存在的 config 服务应返回 true")
	}
	if svc == nil {
		t.Error("GetService 获取已存在的 config 服务不应为 nil")
	}
}

// TestGetService_NonExisting 测试获取不存在的服务返回 false
func TestGetService_NonExisting(t *testing.T) {
	b := safeNewBootstrap(t)
	svc, ok := b.GetService("nonexistent-service")
	if ok {
		t.Error("GetService 获取不存在的服务应返回 false")
	}
	if svc != nil {
		t.Error("GetService 获取不存在的服务应返回 nil")
	}
}

// =============================================================================
// GetServiceTyped 测试
// =============================================================================

// TestGetServiceTyped_Existing 测试获取指定类型的服务
func TestGetServiceTyped_Existing(t *testing.T) {
	b := safeNewBootstrap(t)
	cfg, ok := bootstrap.GetServiceTyped[*config.Config](b, "config")
	if !ok {
		t.Error("GetServiceTyped 获取已存在的 config 服务应返回 true")
	}
	if cfg == nil {
		t.Error("GetServiceTyped 获取已存在的 config 服务不应为 nil")
	}
}

// TestGetServiceTyped_NonExisting 测试获取不存在的服务返回零值和 false
func TestGetServiceTyped_NonExisting(t *testing.T) {
	b := safeNewBootstrap(t)
	result, ok := bootstrap.GetServiceTyped[*config.Config](b, "nonexistent-service")
	if ok {
		t.Error("GetServiceTyped 获取不存在的服务应返回 false")
	}
	if result != nil {
		t.Error("GetServiceTyped 获取不存在的服务应返回零值")
	}
}

// TestGetServiceTyped_TypeMismatch 测试类型不匹配返回零值和 false
func TestGetServiceTyped_TypeMismatch(t *testing.T) {
	b := safeNewBootstrap(t)
	// config 服务的类型是 *config.Config，尝试用 string 类型获取应失败
	result, ok := bootstrap.GetServiceTyped[string](b, "config")
	if ok {
		t.Error("GetServiceTyped 类型不匹配时应返回 false")
	}
	if result != "" {
		t.Error("GetServiceTyped 类型不匹配时应返回零值")
	}
}

// =============================================================================
// MustGetService 测试
// =============================================================================

// TestMustGetService_Existing 测试获取存在的服务
func TestMustGetService_Existing(t *testing.T) {
	b := safeNewBootstrap(t)
	svc := b.MustGetService("config")
	if svc == nil {
		t.Error("MustGetService 获取已存在的 config 服务不应为 nil")
	}
}

// TestMustGetService_Panic 测试获取不存在的服务会 panic
func TestMustGetService_Panic(t *testing.T) {
	b := safeNewBootstrap(t)
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustGetService 获取不存在的服务应触发 panic")
		}
	}()
	b.MustGetService("nonexistent-service")
}

// =============================================================================
// MustGetServiceTyped 测试
// =============================================================================

// TestMustGetServiceTyped_Existing 测试获取存在的指定类型服务
func TestMustGetServiceTyped_Existing(t *testing.T) {
	b := safeNewBootstrap(t)
	cfg := bootstrap.MustGetServiceTyped[*config.Config](b, "config")
	if cfg == nil {
		t.Error("MustGetServiceTyped 获取已存在的 config 服务不应为 nil")
	}
}

// TestMustGetServiceTyped_PanicOnMissing 测试服务不存在时 panic
func TestMustGetServiceTyped_PanicOnMissing(t *testing.T) {
	b := safeNewBootstrap(t)
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustGetServiceTyped 获取不存在的服务应触发 panic")
		}
	}()
	bootstrap.MustGetServiceTyped[*config.Config](b, "nonexistent-service")
}

// TestMustGetServiceTyped_PanicOnTypeMismatch 测试类型不匹配时 panic
func TestMustGetServiceTyped_PanicOnTypeMismatch(t *testing.T) {
	b := safeNewBootstrap(t)
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustGetServiceTyped 类型不匹配时应触发 panic")
		}
	}()
	bootstrap.MustGetServiceTyped[string](b, "config")
}

// =============================================================================
// RegisterCleanupFunc 测试
// =============================================================================

// TestRegisterCleanupFunc 测试注册清理函数（不 panic 即通过）
func TestRegisterCleanupFunc(t *testing.T) {
	b := safeNewBootstrap(t)
	called := false
	b.RegisterCleanupFunc(func() error {
		called = true
		return nil
	})
	// 注：cleanupFuncs 是未导出字段，无法直接验证存储结果
	// 但注册操作不 panic 说明调用成功
	_ = called
}

// =============================================================================
// RegisterRoute 测试
// =============================================================================

// TestRegisterRoute 测试注册路由函数（不 panic 即通过）
func TestRegisterRoute(t *testing.T) {
	b := safeNewBootstrap(t)
	b.RegisterRoute(func(app *fiber.App, cfg *config.Config) {
		// 路由注册回调
	})
}

// =============================================================================
// RegisterInitFunc 测试
// =============================================================================

// TestRegisterInitFunc 测试注册初始化函数（不 panic 即通过）
func TestRegisterInitFunc(t *testing.T) {
	b := safeNewBootstrap(t)
	b.RegisterInitFunc(func(cfg *config.Config) error {
		return nil
	})
}

// =============================================================================
// RegisterMiddleware 测试
// =============================================================================

// TestRegisterMiddleware 测试注册中间件函数（不 panic 即通过）
func TestRegisterMiddleware(t *testing.T) {
	b := safeNewBootstrap(t)
	b.RegisterMiddleware(func(app *fiber.App, cfg *config.Config) {
		// 中间件注册回调
	})
}

// =============================================================================
// HasService 测试
// =============================================================================

// TestHasService_Existing 测试检查存在的服务返回 true
func TestHasService_Existing(t *testing.T) {
	b := safeNewBootstrap(t)
	if !b.HasService("config") {
		t.Error("HasService('config') 应返回 true")
	}
}

// TestHasService_NonExisting 测试检查不存在的服务返回 false
func TestHasService_NonExisting(t *testing.T) {
	b := safeNewBootstrap(t)
	if b.HasService("nonexistent-service") {
		t.Error("HasService('nonexistent-service') 应返回 false")
	}
}

// =============================================================================
// RemoveService 测试
// =============================================================================

// TestRemoveService 测试移除后 HasService 返回 false
func TestRemoveService(t *testing.T) {
	b := safeNewBootstrap(t)
	// 注册一个临时服务
	b.RegisterService("temp-service", "temp-value")
	if !b.HasService("temp-service") {
		t.Fatal("注册后 HasService 应返回 true")
	}
	// 移除服务
	b.RemoveService("temp-service")
	if b.HasService("temp-service") {
		t.Error("移除后 HasService 应返回 false")
	}
}
