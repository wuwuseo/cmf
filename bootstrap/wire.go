//go:build wireinject

package bootstrap

import "github.com/google/wire"

func InitBootstrap() *Bootstrap {
	wire.Build(
		WireSet,
	)
	return nil
}
