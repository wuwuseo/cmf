package casbin

import "errors"

// 错误定义
var (
	// ErrEnforcerNotFound Enforcer 不存在
	ErrEnforcerNotFound = errors.New("enforcer not found for domain")

	// ErrEnforcerCreateFail 创建 Enforcer 失败
	ErrEnforcerCreateFail = errors.New("failed to create enforcer")

	// ErrPolicyLoadFail 加载策略失败
	ErrPolicyLoadFail = errors.New("failed to load policy")

	// ErrInvalidDomain 无效的域名
	ErrInvalidDomain = errors.New("invalid domain name")

	// ErrEnforcerAlreadyExists Enforcer 已存在
	ErrEnforcerAlreadyExists = errors.New("enforcer already exists for domain")

	// ErrConfigAlreadySet 配置已设置
	ErrConfigAlreadySet = errors.New("config already set for domain with existing enforcer")
)

// 常量定义
const (
	// DefaultDomain 默认域名称
	DefaultDomain = "default"
)
