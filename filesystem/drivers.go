package filesystem

import (
	"fmt"

	"github.com/gofiber/storage"
	"github.com/wuwuseo/cmf/config"
	local "github.com/wuwuseo/cmf/storage"
)

// CreateStorageDriver 根据配置创建存储驱动
func CreateStorageDriver(cfg *config.Config, diskName string) (storage.Storage, error) {
	// 获取磁盘配置
	disk, exists := cfg.Filesystem.Disks[diskName]
	if !exists {
		return nil, fmt.Errorf("磁盘配置 '%s' 不存在", diskName)
	}

	// 根据驱动类型创建相应的存储实例
	switch disk.Driver {
	case "local":
		// 获取本地存储的配置选项
		options, ok := disk.Options.(map[string]any)
		if !ok {
			options = make(map[string]any)
		}

		// 获取根路径配置，默认为./data/storage
		rootPath, _ := options["root"].(string)
		if rootPath == "" {
			rootPath = "./data/storage"
		}

		// 创建本地存储实例
		return local.NewLocal(local.LocalConfig{BasePath: rootPath}), nil

	// 可以在这里添加其他驱动的实现，例如：
	// case "s3":
	// 	// 实现S3存储驱动
	// 	return createS3Storage(options)
	// case "oss":
	// 	// 实现阿里云OSS存储驱动
	// 	return createOSSStorage(options)

	default:
		return nil, fmt.Errorf("不支持的存储驱动类型: %s", disk.Driver)
	}
}

// NewFilesystemFromConfig 根据配置创建文件系统实例
func NewFilesystemFromConfig(cfg *config.Config) (*Filesystem, error) {
	// 获取默认磁盘名称
	defaultDisk := cfg.Filesystem.Default
	if defaultDisk == "" {
		defaultDisk = "local" // 默认使用本地存储
	}

	// 创建存储驱动
	adapter, err := CreateStorageDriver(cfg, defaultDisk)
	if err != nil {
		return nil, fmt.Errorf("创建存储驱动失败: %w", err)
	}

	// 创建文件系统实例
	return &Filesystem{
		Config:  *cfg,
		Adapter: adapter,
	}, nil
}