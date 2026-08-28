package manager

import (
	"errors"
	"testing"
)

// TestManager_Get_SingletonAndErrors 覆盖：默认名解析、惰性创建、单例缓存、
// 按名获取、未知名称报错、工厂失败不缓存（可重试）、Range 遍历
func TestManager_Get_SingletonAndErrors(t *testing.T) {
	cfgs := map[string]string{"a": "va", "b": "vb"}
	calls := map[string]int{}

	m := New[string, string]("a",
		func() map[string]string { return cfgs },
		func(name string, c string) (string, error) {
			calls[name]++
			return c + "-" + name, nil
		},
	)

	// 默认名
	if m.Default() != "a" {
		t.Fatalf("Default() = %s，期望 a", m.Default())
	}

	// 无参 Get = 默认名，惰性创建
	v1, err := m.Get()
	if err != nil || v1 != "va-a" {
		t.Fatalf("Get() = %s, %v；期望 va-a, nil", v1, err)
	}
	// 同名单例：再次 Get 不重复创建
	v2, _ := m.Get()
	if v1 != v2 || calls["a"] != 1 {
		t.Fatalf("同一名称应只创建一次（实际创建了 %d 次）", calls["a"])
	}

	// 按名获取
	if v, _ := m.Get("b"); v != "vb-b" {
		t.Fatalf("Get(\"b\") = %s，期望 vb-b", v)
	}

	// 不存在的名称必须报错，不静默回退
	if _, err := m.Get("nope"); err == nil {
		t.Fatal("不存在的名称应返回错误（不静默回退）")
	}

	// Range 遍历所有已创建实例
	seen := map[string]bool{}
	if err := m.Range(func(name string, _ string) error {
		seen[name] = true
		return nil
	}); err != nil {
		t.Fatalf("Range 失败: %v", err)
	}
	if !seen["a"] || !seen["b"] {
		t.Fatalf("Range 应遍历到 a 和 b，实际: %v", seen)
	}

	// 工厂失败不缓存：两次 Get 调两次工厂（失败可重试）
	failCalls := 0
	mf := New[int, string]("x",
		func() map[string]string { return map[string]string{"x": "x"} },
		func(_ string, _ string) (int, error) {
			failCalls++
			return 0, errors.New("boom")
		},
	)
	if _, err := mf.Get(); err == nil {
		t.Fatal("工厂失败应返回错误")
	}
	_, _ = mf.Get() // 再次尝试
	if failCalls != 2 {
		t.Fatalf("失败实例不应被缓存（工厂应被再次调用），实际调用 %d 次", failCalls)
	}
}
