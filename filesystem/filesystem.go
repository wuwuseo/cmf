package filesystem

import (
	"fmt"
	"time"

	"github.com/gofiber/storage"
	"github.com/wuwuseo/cmf/config"
)

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
