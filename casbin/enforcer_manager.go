package casbin

import (
	"fmt"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	fiberlog "github.com/gofiber/fiber/v2/log"
)

// DomainConfig 域配置
type DomainConfig struct {
	ModelPath string // 模型文件路径
	ModelText string // 模型文本内容（优先级高于 ModelPath）
}

// EnforcerManager 管理多个 Casbin Enforcer 实例
type EnforcerManager struct {
	enforcers     map[string]*casbin.Enforcer // 存储域名到 Enforcer 的映射
	domainConfigs map[string]*DomainConfig    // 存储每个域的模型配置
	mu            sync.RWMutex                // 保护 enforcers 和 domainConfigs map 的并发访问
	adapter       persist.Adapter             // Casbin 策略适配器（共享）
	defaultDomain string                      // 默认域名称
}

// NewEnforcerManager 创建新的 EnforcerManager
func NewEnforcerManager(adapter persist.Adapter, defaultDomain string) *EnforcerManager {
	return &EnforcerManager{
		enforcers:     make(map[string]*casbin.Enforcer),
		domainConfigs: make(map[string]*DomainConfig),
		adapter:       adapter,
		defaultDomain: defaultDomain,
	}
}

// validateDomain 验证域名是否有效
func (em *EnforcerManager) validateDomain(domain string) error {
	if domain == "" {
		return ErrInvalidDomain
	}
	// 可以添加更多的域名格式验证
	return nil
}

// createEnforcerWithConfig 使用指定配置创建 Enforcer
// 这是一个私有方法，调用前必须持有写锁
func (em *EnforcerManager) createEnforcerWithConfig(domain string, config *DomainConfig) (*casbin.Enforcer, error) {
	// 检查配置是否存在
	if config == nil {
		return nil, fmt.Errorf("domain config is required for domain: %s", domain)
	}

	// 检查 ModelPath 或 ModelText 至少有一个不为空
	if config.ModelPath == "" && config.ModelText == "" {
		return nil, fmt.Errorf("either ModelPath or ModelText must be set for domain: %s", domain)
	}

	var enforcer *casbin.Enforcer
	var err error

	// 优先使用 ModelText，其次使用 ModelPath
	if config.ModelText != "" {
		// 使用模型文本创建
		fiberlog.Infof("creating enforcer for domain %s with model text", domain)
		m, parseErr := model.NewModelFromString(config.ModelText)
		if parseErr != nil {
			fiberlog.Errorf("failed to parse model text for domain %s: %v", domain, parseErr)
			return nil, fmt.Errorf("failed to parse model text: %w", parseErr)
		}
		enforcer, err = casbin.NewEnforcer(m, em.adapter)
	} else {
		// 使用模型文件创建
		fiberlog.Infof("creating enforcer for domain %s with model path: %s", domain, config.ModelPath)
		enforcer, err = casbin.NewEnforcer(config.ModelPath, em.adapter)
	}

	if err != nil {
		fiberlog.Errorf("failed to create enforcer for domain %s: %v", domain, err)
		return nil, fmt.Errorf("failed to create enforcer: %w", err)
	}

	// 加载策略
	if err := enforcer.LoadPolicy(); err != nil {
		fiberlog.Errorf("failed to load policy for domain %s: %v", domain, err)
		return nil, fmt.Errorf("failed to load policy: %w", err)
	}

	// 保存到 map 中
	em.enforcers[domain] = enforcer
	fiberlog.Infof("enforcer created successfully for domain: %s", domain)

	return enforcer, nil
}

// GetEnforcer 获取指定域的 Enforcer，如果不存在则使用配置创建
// 使用双重检查锁定模式确保并发安全
func (em *EnforcerManager) GetEnforcer(domain string) (*casbin.Enforcer, error) {
	// 验证域名
	if err := em.validateDomain(domain); err != nil {
		fiberlog.Errorf("invalid domain name: %s", domain)
		return nil, err
	}

	// 第一次检查：使用读锁尝试获取
	em.mu.RLock()
	enforcer, exists := em.enforcers[domain]
	em.mu.RUnlock()

	if exists {
		fiberlog.Debugf("enforcer found for domain: %s", domain)
		return enforcer, nil
	}

	// 不存在，升级为写锁创建
	em.mu.Lock()
	defer em.mu.Unlock()

	// 双重检查：再次确认是否已被其他 goroutine 创建
	if enforcer, exists = em.enforcers[domain]; exists {
		fiberlog.Debugf("enforcer already created by another goroutine for domain: %s", domain)
		return enforcer, nil
	}

	// 查找域配置
	config, hasConfig := em.domainConfigs[domain]
	if !hasConfig {
		// 如果没有配置，返回错误
		fiberlog.Errorf("no config found for domain: %s", domain)
		return nil, fmt.Errorf("no config found for domain: %s", domain)
	}

	// 创建新的 Enforcer
	fiberlog.Infof("creating new enforcer for domain: %s", domain)
	enforcer, err := em.createEnforcerWithConfig(domain, config)
	if err != nil {
		fiberlog.Errorf("failed to create enforcer for domain %s: %v", domain, err)
		return nil, err
	}

	return enforcer, nil
}

// GetDefaultEnforcer 获取默认 Enforcer
func (em *EnforcerManager) GetDefaultEnforcer() (*casbin.Enforcer, error) {
	return em.GetEnforcer(em.defaultDomain)
}

// GetEnforcerWithConfig 使用自定义配置获取或创建 Enforcer
// 如果 Enforcer 已存在，返回错误（不允许覆盖）
func (em *EnforcerManager) GetEnforcerWithConfig(domain string, config *DomainConfig) (*casbin.Enforcer, error) {
	// 验证域名
	if err := em.validateDomain(domain); err != nil {
		fiberlog.Errorf("invalid domain name: %s", domain)
		return nil, err
	}

	// 验证配置
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	em.mu.Lock()
	defer em.mu.Unlock()

	// 检查 Enforcer 是否已存在
	if _, exists := em.enforcers[domain]; exists {
		fiberlog.Errorf("enforcer already exists for domain: %s", domain)
		return nil, ErrEnforcerAlreadyExists
	}

	// 保存配置到 domainConfigs
	em.domainConfigs[domain] = config

	// 创建新的 Enforcer
	fiberlog.Infof("creating new enforcer with custom config for domain: %s", domain)
	enforcer, err := em.createEnforcerWithConfig(domain, config)
	if err != nil {
		// 创建失败，清理配置
		delete(em.domainConfigs, domain)
		fiberlog.Errorf("failed to create enforcer for domain %s: %v", domain, err)
		return nil, err
	}

	return enforcer, nil
}

// SetDomainConfig 设置域的模型配置（在创建 Enforcer 前调用）
func (em *EnforcerManager) SetDomainConfig(domain string, config *DomainConfig) error {
	// 验证域名
	if err := em.validateDomain(domain); err != nil {
		fiberlog.Errorf("invalid domain name: %s", domain)
		return err
	}

	// 验证配置
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	em.mu.Lock()
	defer em.mu.Unlock()

	// 检查 Enforcer 是否已创建
	if _, exists := em.enforcers[domain]; exists {
		fiberlog.Errorf("cannot set config for domain %s: enforcer already exists", domain)
		return ErrConfigAlreadySet
	}

	// 保存配置
	em.domainConfigs[domain] = config
	fiberlog.Infof("domain config set for domain: %s", domain)

	return nil
}

// GetDomainConfig 获取域的模型配置
func (em *EnforcerManager) GetDomainConfig(domain string) (*DomainConfig, bool) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	config, exists := em.domainConfigs[domain]
	return config, exists
}
