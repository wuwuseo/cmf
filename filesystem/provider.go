package filesystem

import "github.com/google/wire"

// ProviderSet 文件系统模块的 Wire Provider 集合
var ProviderSet = wire.NewSet(NewFilesystemFromConfig)
