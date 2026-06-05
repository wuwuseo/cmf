package casbin

import (
	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/casbin/casbin/v3/persist"
	"github.com/gofiber/fiber/v3/log"
	"github.com/wuwuseo/cmf/config"
)

type Casbin struct {
	Enforcer *casbin.Enforcer
}

func NewCasbin(adapter persist.Adapter, path string) *Casbin {
	e, err := casbin.NewEnforcer(path, adapter)
	if err != nil {
		panic(err)
	}
	return &Casbin{
		Enforcer: e,
	}
}

func NewCasbinFromString(adapter persist.Adapter, modelString string) *Casbin {
	m, err := model.NewModelFromString(modelString)
	if err != nil {
		log.Fatalf("error: model: %s", err)
	}
	e, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		panic(err)
	}
	return &Casbin{
		Enforcer: e,
	}
}

// InitEnforcerManager 根据配置初始化EnforcerManager
func InitEnforcerManager(adapter persist.Adapter, cfg *config.Config) *EnforcerManager {
	// 创建EnforcerManager
	defaultDomain := cfg.Casbin.DomainsDefault
	if defaultDomain == "" {
		defaultDomain = "default"
	}

	manager := NewEnforcerManager(adapter, defaultDomain)

	// 设置域配置
	for _, domain := range cfg.Casbin.Domains {
		domainConfig := &DomainConfig{
			ModelPath: domain.ModelPath,
			ModelText: domain.ModelText,
		}
		// 只有在配置有效时才设置
		if domainConfig.ModelPath != "" || domainConfig.ModelText != "" {
			if err := manager.SetDomainConfig(domain.Name, domainConfig); err != nil {
				log.Warnf("Failed to set casbin domain config for %s: %v", domain.Name, err)
				continue
			}

			// 如果设置了自动加载，则立即创建Enforcer
			if domain.AutoLoad {
				if _, err := manager.GetEnforcer(domain.Name); err != nil {
					// 记录错误但不中断程序
					log.Warnf("Failed to create enforcer for domain %s: %v", domain.Name, err)
				}
			}
		}
	}

	return manager
}
