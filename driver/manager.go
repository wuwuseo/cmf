package driver

import (
	"fmt"
	"sync"
)

// Driver 是驱动工厂函数的泛型接口定义
// T 表示驱动创建函数的返回类型
// C 表示创建驱动时传入的配置类型
type Driver[T any, C any] func(config C) (T, error)

// Manager 是通用的驱动管理器实现
// T 表示驱动创建函数的返回类型
// C 表示创建驱动时传入的配置类型
type Manager[T any, C any] struct {
	drivers sync.Map // 使用sync.Map保证并发安全
}

// NewManager 创建一个新的驱动管理器实例
func NewManager[T any, C any]() *Manager[T, C] {
	return &Manager[T, C]{
		drivers: sync.Map{},
	}
}

// Register 注册一个新的驱动工厂函数
// name 驱动名称
// driver 驱动工厂函数
func (m *Manager[T, C]) Register(name string, driver Driver[T, C]) {
	m.drivers.Store(name, driver)
}

// Get 获取指定名称的驱动工厂函数
// name 驱动名称
// 返回驱动工厂函数或false（如果不存在）
func (m *Manager[T, C]) Get(name string) (Driver[T, C], bool) {
	driver, found := m.drivers.Load(name)
	if !found {
		return nil, false
	}

	typedDriver, ok := driver.(Driver[T, C])
	if !ok {
		return nil, false
	}

	return typedDriver, true
}

// Create 根据配置创建指定名称的驱动实例
// name 驱动名称
// config 驱动配置
// 返回驱动实例或错误
func (m *Manager[T, C]) Create(name string, config C) (T, error) {
	driver, found := m.Get(name)
	if !found {
		var zero T
		return zero, fmt.Errorf("驱动 '%s' 未注册", name)
	}

	return driver(config)
}

// List 列出所有已注册的驱动名称
func (m *Manager[T, C]) List() []string {
	var drivers []string
	m.drivers.Range(func(key, value any) bool {
		drivers = append(drivers, key.(string))
		return true
	})
	return drivers
}

// Has 检查指定名称的驱动是否已注册
// name 驱动名称
// 返回是否已注册
func (m *Manager[T, C]) Has(name string) bool {
	_, found := m.drivers.Load(name)
	return found
}