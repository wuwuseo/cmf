package casbin

import (
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/persist"
	gofibercasbin "github.com/gofiber/contrib/casbin"
	"github.com/wuwuseo/cmf/config"
)

type Casbin struct {
	Enforcer *casbin.Enforcer
}

func NewCasbinMiddleware(config *config.Config, adapter persist.Adapter, path string) *gofibercasbin.Middleware {
	return gofibercasbin.New(gofibercasbin.Config{
		ModelFilePath: path,
		PolicyAdapter: adapter,
	})
}

func NewCasbin(config *config.Config, adapter persist.Adapter, path string) *Casbin {
	e, err := casbin.NewEnforcer(path, adapter)
	if err != nil {
		panic(err)
	}
	return &Casbin{
		Enforcer: e,
	}
}
