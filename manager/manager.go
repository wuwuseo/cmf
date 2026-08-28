// Package manager 提供按名称惰性创建实例的通用管理器，
// 作为 database/cache/redis/filesystem 多驱动切换的统一基座。
package manager

import (
	"fmt"
	"sync"
)

// Manager 按名称管理惰性创建的实例（并发安全单例）
//   - T: 实例类型（如 *sql.DB、*redis.Client、gostore.StoreInterface）
//   - C: 该模块命名配置的类型（如 config.Database、config.Redis）
//
// 各模块只需提供"配置读取函数 + 工厂函数"，即可获得一致的 Get(name)/Range API。
// ponytail: 全局互斥锁，天花板是创建期间同 Manager 内串行；升级路径：按 key 分段锁
type Manager[T any, C any] struct {
	mu      sync.Mutex
	items   map[string]T
	def     string                            // 默认实例名
	configs func() map[string]C               // 从 Config 取命名配置表
	factory func(name string, c C) (T, error) // 工厂：按配置创建实例
}

// New 创建管理器：def 默认实例名；configs 返回命名配置表；factory 按配置创建实例
func New[T any, C any](def string, configs func() map[string]C, factory func(name string, c C) (T, error)) *Manager[T, C] {
	return &Manager[T, C]{
		items:   make(map[string]T),
		def:     def,
		configs: configs,
		factory: factory,
	}
}

// Default 返回默认实例名
func (m *Manager[T, C]) Default() string {
	return m.def
}

// Get 按名获取实例；无参或空名 = 默认名。首次访问惰性创建，之后返回缓存单例。
// 名称不存在时返回错误（不静默回退，避免配错名字连到错误资源）
func (m *Manager[T, C]) Get(name ...string) (T, error) {
	key := m.def
	if len(name) > 0 && name[0] != "" {
		key = name[0]
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if v, ok := m.items[key]; ok {
		return v, nil
	}

	cfgs := m.configs()
	c, ok := cfgs[key]
	if !ok {
		var zero T
		available := make([]string, 0, len(cfgs))
		for k := range cfgs {
			available = append(available, k)
		}
		return zero, fmt.Errorf("配置 '%s' 不存在（可用配置: %v）", key, available)
	}

	v, err := m.factory(key, c)
	if err != nil {
		return v, err
	}
	m.items[key] = v
	return v, nil
}

// MustGet 同 Get，但出错时 panic（用于启动初始化路径，快速失败）
func (m *Manager[T, C]) MustGet(name ...string) T {
	v, err := m.Get(name...)
	if err != nil {
		panic(err)
	}
	return v
}

// Range 遍历已创建的实例（可用于统一关闭资源）
func (m *Manager[T, C]) Range(fn func(name string, v T) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range m.items {
		if err := fn(k, v); err != nil {
			return err
		}
	}
	return nil
}

// Close 关闭所有实现了 io.Closer 的已创建实例（非 Closer 的跳过）
// 返回首个关闭错误，其余仍会尝试关闭
func (m *Manager[T, C]) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var firstErr error
	for k, v := range m.items {
		if closer, ok := any(v).(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("关闭实例 '%s' 失败: %w", k, err)
			}
		}
	}
	return firstErr
}
