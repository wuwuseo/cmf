package bootstrap

import (
	"github.com/google/wire"
	"github.com/wuwuseo/cmf/config"
)

type Bootstrap struct {
}

func NewBootstrap() *Bootstrap {
	return &Bootstrap{}
}

func (b *Bootstrap) Init() {
	// 初始化配置
	config.InitConfig()
}

var WireSet = wire.NewSet(
	NewBootstrap,
)
