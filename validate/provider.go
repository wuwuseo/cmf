package validate

import "github.com/google/wire"

// ProviderSet 验证模块的 Wire Provider 集合
var ProviderSet = wire.NewSet(NewValidator)
