package local

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Storage 本地文件存储实现
// 实现了 Storage 接口，用于在本地文件系统中直接存储原始文件数据
type Storage struct {
	BasePath string // 存储文件的基础目录
}

// Config 本地存储配置参数
type Config struct {
	BasePath string // 存储文件的基础目录
}

// configOrDefault 获取配置参数，如果未提供则使用默认值
func configOrDefault(cfg ...Config) Config {
	if len(cfg) > 0 {
		return cfg[0]
	}
	return Config{
		BasePath: "./data/storage", // 默认存储路径
	}
}

// New 创建一个新的本地存储实例
// 参数 cfg 可选，用于自定义配置
func New(cfg ...Config) *Storage {
	config := configOrDefault(cfg...)

	// 确保基础目录存在
	if err := os.MkdirAll(config.BasePath, 0755); err != nil {
		// 记录错误但继续执行
	}

	return &Storage{
		BasePath: config.BasePath,
	}
}

// getFilePath 将键转换为文件路径
func (s *Storage) getFilePath(key string) string {
	return filepath.Join(s.BasePath, key)
}

// getMetaFilePath 将键转换为元数据文件路径
func (s *Storage) getMetaFilePath(key string) string {
	return filepath.Join(s.BasePath, key+".meta")
}

// GetWithContext 获取给定键的原始文件数据（带上下文）
// 当键不存在时返回 `nil, nil`
func (s *Storage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 检查元数据文件，确认是否过期
	metaPath := s.getMetaFilePath(key)
	metaData, err := os.ReadFile(metaPath)
	if err == nil {
		// 解析过期时间
		expTimeStr := string(metaData)
		if expTimeStr != "" {
			expTimeUnix, parseErr := strconv.ParseInt(expTimeStr, 10, 64)
			if parseErr == nil && expTimeUnix > 0 {
				// 检查是否过期
				if time.Now().Unix() > expTimeUnix {
					// 已过期，删除数据文件和元数据文件
					go func() {
						s.DeleteWithContext(ctx, key)
					}()
					return nil, nil
				}
			}
		}
	}

	// 读取原始数据文件
	filePath := s.getFilePath(key)
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 键不存在
		}
		return nil, err
	}

	return fileData, nil
}

// Get 获取给定键的原始文件数据（不使用上下文）
// 当键不存在时返回 `nil, nil`
func (s *Storage) Get(key string) ([]byte, error) {
	return s.GetWithContext(context.Background(), key)
}

// SetWithContext 存储给定键的原始文件数据（带上下文）
// exp 为过期时间，0 表示永不过期
func (s *Storage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 检查键或值是否为空
	if key == "" || val == nil {
		return nil // 忽略空键或空值
	}

	// 写入原始数据文件
	filePath := s.getFilePath(key)
	if err := os.WriteFile(filePath, val, 0644); err != nil {
		return err
	}

	// 处理过期时间
	metaPath := s.getMetaFilePath(key)
	if exp > 0 {
		// 设置过期时间（Unix时间戳）
		expTime := time.Now().Add(exp).Unix()
		return os.WriteFile(metaPath, []byte(strconv.FormatInt(expTime, 10)), 0644)
	}
	return nil
}

// Set 存储给定键的原始文件数据（不使用上下文）
// exp 为过期时间，0 表示永不过期
// 空键或空值将被忽略而不报错
func (s *Storage) Set(key string, val []byte, exp time.Duration) error {
	return s.SetWithContext(context.Background(), key, val, exp)
}

// DeleteWithContext 删除给定键的原始文件数据（带上下文）
// 如果存储中不包含该键，不会返回错误
func (s *Storage) DeleteWithContext(ctx context.Context, key string) error {
	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 删除数据文件
	filePath := s.getFilePath(key)
	err1 := os.Remove(filePath)

	// 删除元数据文件
	metaPath := s.getMetaFilePath(key)
	err2 := os.Remove(metaPath)

	// 只有当两个删除操作都失败且都不是因为文件不存在时才返回错误
	if err1 != nil && !os.IsNotExist(err1) {
		return err1
	}
	if err2 != nil && !os.IsNotExist(err2) {
		return err2
	}

	return nil
}

// Delete 删除给定键的原始文件数据（不使用上下文）
// 如果存储中不包含该键，不会返回错误
func (s *Storage) Delete(key string) error {
	return s.DeleteWithContext(context.Background(), key)
}

// ResetWithContext 重置存储并删除所有键（带上下文）
func (s *Storage) ResetWithContext(ctx context.Context) error {
	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 读取目录内容
	entries, err := os.ReadDir(s.BasePath)
	if err != nil {
		return err
	}

	// 删除所有文件（包括数据文件和元数据文件）
	for _, entry := range entries {
		if !entry.IsDir() {
			filePath := filepath.Join(s.BasePath, entry.Name())
			if err := os.Remove(filePath); err != nil {
				return err
			}
		}
	}

	return nil
}

// Reset 重置存储并删除所有键（不使用上下文）
func (s *Storage) Reset() error {
	return s.ResetWithContext(context.Background())
}

// Close 关闭存储，停止任何运行的垃圾收集器和打开的连接
// 对于本地文件存储，此方法实际上不需要执行任何操作
func (s *Storage) Close() error {
	// 本地文件存储不需要特别的关闭操作
	return nil
}
