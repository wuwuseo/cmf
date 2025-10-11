package filesystem

import (
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/storage"
	"github.com/gofiber/storage/s3/v2"
	"github.com/wuwuseo/cmf/config"
	local "github.com/wuwuseo/cmf/storage/local"
)

// 使用sync.Map存储驱动单例，确保每个磁盘配置只创建一个实例
var driverInstances = sync.Map{}

// DualStorage 复合存储适配器，支持同时存储到主存储和本地存储
type DualStorage struct {
	Primary storage.Storage // 主存储（如S3）
	Local   storage.Storage // 本地存储
}

// Close implements storage.Storage.
func (d *DualStorage) Close() error {
	return d.Primary.Close()
}

// Reset implements storage.Storage.
func (d *DualStorage) Reset() error {
	return d.Primary.Reset()
}

// Get 从主存储获取数据
func (d *DualStorage) Get(key string) ([]byte, error) {
	return d.Primary.Get(key)
}

// Set 同时存储到主存储和本地存储
func (d *DualStorage) Set(key string, value []byte, expiration time.Duration) error {
	// 先存储到主存储
	if err := d.Primary.Set(key, value, expiration); err != nil {
		return err
	}
	// 再存储到本地存储
	return d.Local.Set(key, value, expiration)
}

// Delete 从主存储和本地存储都删除数据
func (d *DualStorage) Delete(key string) error {
	// 从主存储删除
	err1 := d.Primary.Delete(key)
	// 从本地存储删除
	err2 := d.Local.Delete(key)

	// 返回第一个错误（如果有的话）
	if err1 != nil {
		return err1
	}
	return err2
}

type Filesystem struct {
	Config  config.Config
	Adapter storage.Storage
}

func (f *Filesystem) Get(key string) ([]byte, error) {
	return f.Adapter.Get(key)
}

func (f *Filesystem) Set(key string, value []byte, expiration time.Duration) error {
	return f.Adapter.Set(key, value, expiration)
}

func (f *Filesystem) Delete(key string) error {
	return f.Adapter.Delete(key)
}

// NewFilesystem 创建一个新的文件系统实例
// 此方法保持向后兼容性，直接使用提供的适配器
func NewFilesystem(adapter storage.Storage, config config.Config) *Filesystem {
	return &Filesystem{
		Config:  config,
		Adapter: adapter,
	}
}

// NewStorageDriver 根据配置创建存储驱动
// 使用sync.Map确保每个磁盘配置只创建一个实例（单例模式）
func NewStorageDriver(cfg *config.Config, diskName string) (storage.Storage, error) {
	// 构建磁盘配置的唯一标识符
	diskKey := fmt.Sprintf("%s_%s", cfg.App.Name, diskName)

	// 尝试从sync.Map中获取已存在的实例
	if instance, ok := driverInstances.Load(diskKey); ok {
		return instance.(storage.Storage), nil
	}

	// 获取磁盘配置
	disk, exists := cfg.Filesystem.Disks[diskName]
	if !exists {
		return nil, fmt.Errorf("磁盘配置 '%s' 不存在", diskName)
	}

	// 根据驱动类型创建相应的存储实例
	options, ok := disk.Options.(map[string]any)
	if !ok {
		options = make(map[string]any)
	}

	var adapter storage.Storage
	var err error

	switch disk.Driver {
	case "local":
		// 获取本地存储的配置选项
		// 获取根路径配置，默认为./data/storage
		adapter, err = NewLocalStorage(options)

	case "s3":
		// 实现S3存储驱动
		adapter, err = s3.New(s3.Config{
			Credentials: s3.Credentials{
				AccessKey:       options["access_key"].(string),
				SecretAccessKey: options["secret_key"].(string),
			},
			Region:   options["region"].(string),
			Bucket:   options["bucket"].(string),
			Endpoint: options["endpoint"].(string),
		}), nil

	default:
		return nil, fmt.Errorf("不支持的存储驱动类型: %s", disk.Driver)
	}

	if err != nil {
		return nil, err
	}

	// 将新创建的实例存储到sync.Map中
	driverInstances.Store(diskKey, adapter)

	return adapter, nil
}

func NewLocalStorage(options map[string]any) (storage.Storage, error) {
	rootPath, _ := options["root"].(string)
	if rootPath == "" {
		rootPath = "./data/storage"
	}

	// 创建本地存储实例
	return local.New(local.Config{BasePath: rootPath}), nil
}

// NewFilesystemFromConfig 根据配置创建文件系统实例
func NewFilesystemFromConfig(cfg *config.Config) (*Filesystem, error) {
	// 获取默认磁盘名称
	defaultDisk := cfg.Filesystem.Default
	if defaultDisk == "" {
		defaultDisk = "local" // 默认使用本地存储
	}

	// 创建存储驱动
	adapter, err := NewStorageDriver(cfg, defaultDisk)
	if err != nil {
		return nil, fmt.Errorf("创建存储驱动失败: %w", err)
	}

	// 如果启用了IsAndLocal且默认磁盘不是local，则创建复合存储适配器
	if cfg.Filesystem.IsAndLocal && defaultDisk != "local" {
		// 创建本地存储驱动
		localAdapter, err := NewStorageDriver(cfg, "local")
		if err != nil {
			return nil, fmt.Errorf("创建本地存储驱动失败: %w", err)
		}

		// 创建复合存储适配器
		adapter = &DualStorage{
			Primary: adapter,
			Local:   localAdapter,
		}
	}

	// 创建文件系统实例
	return &Filesystem{
		Config:  *cfg,
		Adapter: adapter,
	}, nil
}
