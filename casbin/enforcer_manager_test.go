package casbin_test

import (
	"os"
	"path/filepath"
	"testing"

	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
	"github.com/wuwuseo/cmf/casbin"
)

// setupEnforcerManager 创建测试用的 EnforcerManager 和临时文件
func setupEnforcerManager(t *testing.T) (*casbin.EnforcerManager, string, *fileadapter.Adapter) {
	t.Helper()

	dir := t.TempDir()

	modelPath := filepath.Join(dir, "rbac_model.conf")
	if err := os.WriteFile(modelPath, []byte(testModel), 0644); err != nil {
		t.Fatalf("创建模型文件失败: %v", err)
	}

	policyPath := filepath.Join(dir, "policy.csv")
	if err := os.WriteFile(policyPath, []byte{}, 0644); err != nil {
		t.Fatalf("创建策略文件失败: %v", err)
	}

	adapter := fileadapter.NewAdapter(policyPath)
	manager := casbin.NewEnforcerManager(adapter, "default")

	return manager, modelPath, adapter
}

// TestNewEnforcerManager 测试创建管理器
func TestNewEnforcerManager(t *testing.T) {
	_, _, adapter := setupEnforcerManager(t)

	manager := casbin.NewEnforcerManager(adapter, "default")
	if manager == nil {
		t.Fatal("NewEnforcerManager 返回 nil")
	}
}

// TestNewEnforcerManager_CustomDomain 测试使用自定义默认域创建管理器
func TestNewEnforcerManager_CustomDomain(t *testing.T) {
	_, _, adapter := setupEnforcerManager(t)

	manager := casbin.NewEnforcerManager(adapter, "myapp")
	if manager == nil {
		t.Fatal("NewEnforcerManager 返回 nil")
	}
}

// TestGetDefaultEnforcer 测试获取默认 enforcer
func TestGetDefaultEnforcer(t *testing.T) {
	manager, modelPath, _ := setupEnforcerManager(t)

	// 设置默认域配置
	config := &casbin.DomainConfig{ModelPath: modelPath}
	err := manager.SetDomainConfig("default", config)
	if err != nil {
		t.Fatalf("设置域配置失败: %v", err)
	}

	enforcer, err := manager.GetDefaultEnforcer()
	if err != nil {
		t.Fatalf("获取默认 enforcer 失败: %v", err)
	}
	if enforcer == nil {
		t.Fatal("默认 enforcer 为 nil")
	}
}

// TestSetDomainConfig 测试设置域配置
func TestSetDomainConfig(t *testing.T) {
	manager, modelPath, _ := setupEnforcerManager(t)

	config := &casbin.DomainConfig{ModelPath: modelPath}
	err := manager.SetDomainConfig("testdomain", config)
	if err != nil {
		t.Fatalf("设置域配置失败: %v", err)
	}

	// 验证配置是否已设置
	dc, exists := manager.GetDomainConfig("testdomain")
	if !exists {
		t.Fatal("域配置未找到")
	}
	if dc.ModelPath != modelPath {
		t.Fatalf("ModelPath 不匹配: got %s, want %s", dc.ModelPath, modelPath)
	}
}

// TestSetDomainConfig_WithModelText 测试使用模型文本设置域配置
func TestSetDomainConfig_WithModelText(t *testing.T) {
	manager, _, _ := setupEnforcerManager(t)

	config := &casbin.DomainConfig{ModelText: testModel}
	err := manager.SetDomainConfig("textdomain", config)
	if err != nil {
		t.Fatalf("设置域配置失败: %v", err)
	}

	dc, exists := manager.GetDomainConfig("textdomain")
	if !exists {
		t.Fatal("域配置未找到")
	}
	if dc.ModelText != testModel {
		t.Fatal("ModelText 不匹配")
	}
}

// TestSetDomainConfig_InvalidDomain 测试设置无效域名的配置
func TestSetDomainConfig_InvalidDomain(t *testing.T) {
	manager, _, _ := setupEnforcerManager(t)

	config := &casbin.DomainConfig{ModelText: testModel}
	err := manager.SetDomainConfig("", config)
	if err == nil {
		t.Fatal("期望返回错误，但返回了 nil")
	}
	if err != casbin.ErrInvalidDomain {
		t.Fatalf("期望 ErrInvalidDomain，但得到: %v", err)
	}
}

// TestSetDomainConfig_NilConfig 测试设置 nil 配置
func TestSetDomainConfig_NilConfig(t *testing.T) {
	manager, _, _ := setupEnforcerManager(t)

	err := manager.SetDomainConfig("testdomain", nil)
	if err == nil {
		t.Fatal("期望返回错误，但返回了 nil")
	}
}

// TestSetDomainConfig_EnforcerAlreadyExists 测试 enforcer 已存在时设置配置
func TestSetDomainConfig_EnforcerAlreadyExists(t *testing.T) {
	manager, modelPath, _ := setupEnforcerManager(t)

	// 先设置配置并创建 enforcer
	config := &casbin.DomainConfig{ModelPath: modelPath}
	err := manager.SetDomainConfig("testdomain", config)
	if err != nil {
		t.Fatalf("设置域配置失败: %v", err)
	}

	// 获取 enforcer 使其被创建
	_, err = manager.GetEnforcer("testdomain")
	if err != nil {
		t.Fatalf("获取 enforcer 失败: %v", err)
	}

	// 再次设置配置应该失败
	err = manager.SetDomainConfig("testdomain", config)
	if err == nil {
		t.Fatal("期望返回错误，但返回了 nil")
	}
	if err != casbin.ErrConfigAlreadySet {
		t.Fatalf("期望 ErrConfigAlreadySet，但得到: %v", err)
	}
}

// TestGetDomainConfig 测试获取域配置
func TestGetDomainConfig(t *testing.T) {
	manager, modelPath, _ := setupEnforcerManager(t)

	config := &casbin.DomainConfig{ModelPath: modelPath}
	err := manager.SetDomainConfig("testdomain", config)
	if err != nil {
		t.Fatalf("设置域配置失败: %v", err)
	}

	dc, exists := manager.GetDomainConfig("testdomain")
	if !exists {
		t.Fatal("域配置未找到")
	}
	if dc == nil {
		t.Fatal("返回的域配置为 nil")
	}
}

// TestGetDomainConfig_NotFound 测试获取不存在的域配置
func TestGetDomainConfig_NotFound(t *testing.T) {
	manager, _, _ := setupEnforcerManager(t)

	_, exists := manager.GetDomainConfig("nonexistent")
	if exists {
		t.Fatal("不存在的域不应该返回配置")
	}
}

// TestGetEnforcer 测试获取 enforcer
func TestGetEnforcer(t *testing.T) {
	manager, modelPath, _ := setupEnforcerManager(t)

	// 设置配置
	config := &casbin.DomainConfig{ModelPath: modelPath}
	err := manager.SetDomainConfig("testdomain", config)
	if err != nil {
		t.Fatalf("设置域配置失败: %v", err)
	}

	enforcer, err := manager.GetEnforcer("testdomain")
	if err != nil {
		t.Fatalf("获取 enforcer 失败: %v", err)
	}
	if enforcer == nil {
		t.Fatal("enforcer 为 nil")
	}
}

// TestGetEnforcer_CachedReturn 测试已缓存的 enforcer 直接返回
func TestGetEnforcer_CachedReturn(t *testing.T) {
	manager, modelPath, _ := setupEnforcerManager(t)

	config := &casbin.DomainConfig{ModelPath: modelPath}
	manager.SetDomainConfig("testdomain", config)

	// 第一次获取，创建 enforcer
	e1, err := manager.GetEnforcer("testdomain")
	if err != nil {
		t.Fatalf("第一次获取 enforcer 失败: %v", err)
	}

	// 第二次获取，应该返回缓存的
	e2, err := manager.GetEnforcer("testdomain")
	if err != nil {
		t.Fatalf("第二次获取 enforcer 失败: %v", err)
	}

	if e1 != e2 {
		t.Fatal("两次获取的 enforcer 不是同一个实例")
	}
}

// TestGetEnforcer_InvalidDomain 测试获取无效域名的 enforcer
func TestGetEnforcer_InvalidDomain(t *testing.T) {
	manager, _, _ := setupEnforcerManager(t)

	_, err := manager.GetEnforcer("")
	if err == nil {
		t.Fatal("期望返回错误，但返回了 nil")
	}
	if err != casbin.ErrInvalidDomain {
		t.Fatalf("期望 ErrInvalidDomain，但得到: %v", err)
	}
}

// TestGetEnforcer_NoConfig 测试不存在配置的域
func TestGetEnforcer_NoConfig(t *testing.T) {
	manager, _, _ := setupEnforcerManager(t)

	_, err := manager.GetEnforcer("noconfigdomain")
	if err == nil {
		t.Fatal("期望返回错误，但返回了 nil")
	}
}

// TestGetEnforcerWithConfig 测试使用自定义配置获取 enforcer
func TestGetEnforcerWithConfig(t *testing.T) {
	manager, _, _ := setupEnforcerManager(t)

	config := &casbin.DomainConfig{ModelText: testModel}
	enforcer, err := manager.GetEnforcerWithConfig("customdomain", config)
	if err != nil {
		t.Fatalf("获取 enforcer 失败: %v", err)
	}
	if enforcer == nil {
		t.Fatal("enforcer 为 nil")
	}

	// 验证配置也被保存了
	dc, exists := manager.GetDomainConfig("customdomain")
	if !exists {
		t.Fatal("域配置未保存")
	}
	if dc.ModelText != testModel {
		t.Fatal("ModelText 不匹配")
	}
}

// TestGetEnforcerWithConfig_ModelPath 测试使用模型路径配置获取 enforcer
func TestGetEnforcerWithConfig_ModelPath(t *testing.T) {
	manager, modelPath, _ := setupEnforcerManager(t)

	config := &casbin.DomainConfig{ModelPath: modelPath}
	enforcer, err := manager.GetEnforcerWithConfig("pathdomain", config)
	if err != nil {
		t.Fatalf("获取 enforcer 失败: %v", err)
	}
	if enforcer == nil {
		t.Fatal("enforcer 为 nil")
	}
}

// TestGetEnforcerWithConfig_InvalidDomain 测试无效域名
func TestGetEnforcerWithConfig_InvalidDomain(t *testing.T) {
	manager, _, _ := setupEnforcerManager(t)

	config := &casbin.DomainConfig{ModelText: testModel}
	_, err := manager.GetEnforcerWithConfig("", config)
	if err == nil {
		t.Fatal("期望返回错误，但返回了 nil")
	}
	if err != casbin.ErrInvalidDomain {
		t.Fatalf("期望 ErrInvalidDomain，但得到: %v", err)
	}
}

// TestGetEnforcerWithConfig_NilConfig 测试 nil 配置
func TestGetEnforcerWithConfig_NilConfig(t *testing.T) {
	manager, _, _ := setupEnforcerManager(t)

	_, err := manager.GetEnforcerWithConfig("testdomain", nil)
	if err == nil {
		t.Fatal("期望返回错误，但返回了 nil")
	}
}

// TestGetEnforcerWithConfig_EnforcerAlreadyExists 测试 enforcer 已存在
func TestGetEnforcerWithConfig_EnforcerAlreadyExists(t *testing.T) {
	manager, _, _ := setupEnforcerManager(t)

	config := &casbin.DomainConfig{ModelText: testModel}

	// 第一次创建
	_, err := manager.GetEnforcerWithConfig("testdomain", config)
	if err != nil {
		t.Fatalf("第一次创建 enforcer 失败: %v", err)
	}

	// 第二次使用相同域名创建应失败
	_, err = manager.GetEnforcerWithConfig("testdomain", config)
	if err == nil {
		t.Fatal("期望返回错误，但返回了 nil")
	}
	if err != casbin.ErrEnforcerAlreadyExists {
		t.Fatalf("期望 ErrEnforcerAlreadyExists，但得到: %v", err)
	}
}

// TestGetEnforcerWithConfig_EmptyConfig 测试空配置（ModelPath 和 ModelText 都为空）
func TestGetEnforcerWithConfig_EmptyConfig(t *testing.T) {
	manager, _, _ := setupEnforcerManager(t)

	config := &casbin.DomainConfig{}
	_, err := manager.GetEnforcerWithConfig("testdomain", config)
	if err == nil {
		t.Fatal("期望返回错误，但返回了 nil")
	}
}

// TestValidateDomain_Empty 测试空域名验证
func TestValidateDomain_Empty(t *testing.T) {
	manager, _, _ := setupEnforcerManager(t)

	// 通过 SetDomainConfig 间接测试 validateDomain
	err := manager.SetDomainConfig("", &casbin.DomainConfig{ModelText: testModel})
	if err == nil {
		t.Fatal("空域名应该返回错误")
	}
	if err != casbin.ErrInvalidDomain {
		t.Fatalf("期望 ErrInvalidDomain，但得到: %v", err)
	}
}

// TestValidateDomain_Valid 测试有效域名验证
func TestValidateDomain_Valid(t *testing.T) {
	manager, _, _ := setupEnforcerManager(t)

	// 有效域名应该可以正常设置配置
	err := manager.SetDomainConfig("valid-domain", &casbin.DomainConfig{ModelText: testModel})
	if err != nil {
		t.Fatalf("有效域名设置失败: %v", err)
	}
}
