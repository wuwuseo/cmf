package queue

import (
	"net/http"

	"github.com/hibiken/asynqmon"
)

// MonitorHandler 返回 asynqmon 监控页的 HTTP 处理器（只读模式），
// 可通过 fiber adaptor 挂载到管理端认证之后，作为管理 API 的可视化补充。
// rootPath 为挂载路径（如 /api/v1/admin/auth/queue-monitor），须与实际路由前缀一致。
//
// 出于安全与审计考虑固定为只读：任务的变更操作（重试/删除等）统一走
// 管理 API，以便记录操作日志。
func MonitorHandler(cfg Config, rootPath string) http.Handler {
	return asynqmon.New(asynqmon.Options{
		RootPath:     rootPath,
		RedisConnOpt: cfg.connOpt(),
		ReadOnly:     true,
	})
}
