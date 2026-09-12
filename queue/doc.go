// Package queue 基于 asynq 提供任务队列能力。
//
// 任务队列用于异步/延迟执行、失败自动重试与任务状态管理，与 cron（定时触发）、
// tasks（进程内注册表）互补：cron 管"何时触发"，队列管"如何可靠执行"。
//
// 组件划分：
//   - Client：投递端，将任务写入 Redis；
//   - Server：执行端，常驻 worker 从 Redis 认领任务并调用注册的 Handler；
//   - Inspector：管理/观测端，提供任务列表、统计、重试、删除、取消等操作，供管理后台 API 使用；
//   - MonitorHandler：挂载 asynqmon 监控页（只读），作为管理 API 的可视化补充。
//
// 使用示例：
//
//	cfg := queue.DefaultConfig()
//	cfg.Addr = "127.0.0.1:6379"
//	client := queue.NewClient(cfg)
//	taskID, err := client.Enqueue(ctx, "email:send", payload, queue.WithQueue("mail"), queue.WithMaxRetry(5))
//
//	server := queue.NewServer(cfg)
//	server.Handle("email:send", func(ctx context.Context, t *queue.Task) error {
//	    // 业务逻辑；返回错误将触发指数退避重试，超过 MaxRetry 进入 archived（死信）
//	    return nil
//	})
//	if err := server.Start(); err != nil { ... }
//	defer server.Shutdown()
package queue
