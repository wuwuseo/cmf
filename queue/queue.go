package queue

import (
	"context"
	"runtime"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/hibiken/asynq"
	"github.com/wuwuseo/cmf/config"
)

// Task 表示一个队列任务：Name 为处理器名（入队前必须已在 Server 注册），
// Payload 为任务载荷（调用方自行编解码，通常为 JSON）。
type Task struct {
	Name    string
	Payload []byte
}

// Handler 队列任务处理器函数签名。
// 返回非 nil 错误表示执行失败：任务默认按指数退避自动重试，超过 MaxRetry 后进入 archived（死信）。
type Handler func(ctx context.Context, t *Task) error

// Config 任务队列配置：Redis 连接参数 + 运行参数。
// Client、Server、Inspector、Monitor 共用同一份配置，保证指向同一 Redis。
type Config struct {
	Addr     string // Redis 地址，格式 "host:port"
	Username string // Redis 用户名（ACL 认证用，可为空）
	Password string // Redis 密码，可为空
	DB       int    // Redis DB 索引
	PoolSize int    // Redis 连接池大小，0 表示使用 asynq 默认值

	Concurrency    int            // 并发 worker 数，0 表示取 CPU 核数
	Queues         map[string]int // 参与消费的队列及优先级权重；nil 表示仅消费 default 队列
	StrictPriority bool           // 严格优先级模式：高优先级队列非空时不消费低优先级队列

	MaxRetry        int           // 入队时默认最大重试次数
	Timeout         time.Duration // 入队时默认单任务处理超时；0 表示不设置
	Retention       time.Duration // 入队时默认 completed 保留时长（0 表示完成即删除，不保留历史）
	MaxQueueSize    int           // 每队列存量任务准入阈值；<=0 使用 10000
	MaxPayloadBytes int           // 单任务载荷字节上限；<=0 使用 64KiB

	// 空闲轮询间隔（所有队列无任务时检查新任务的周期），0 表示 asynq 默认 1 秒。
	// 仅影响低负载下的响应延迟，通常无需调整；测试可调小以加速断言。
	TaskCheckInterval time.Duration
	RetryDelay        time.Duration // 固定重试间隔；0 使用 asynq 默认指数退避，主要供测试或特殊任务环境使用
}

// DefaultConfig 返回默认配置：本机 Redis、4 个 worker、单 default 队列、最多重试 3 次。
func DefaultConfig() Config {
	return Config{
		Addr:            "127.0.0.1:6379",
		DB:              0,
		Concurrency:     4,
		Queues:          map[string]int{"default": 10},
		MaxRetry:        3,
		MaxQueueSize:    10000,
		MaxPayloadBytes: 64 * 1024,
	}
}

// NewConfigFromApp 从 cmf 全局配置构建队列配置。
// Redis 连接参数取自 cfg.Redis.Connections 中名为 conn 的连接（缺省时依次回退：
// 显式参数 -> cfg.Redis.Default -> "redis"），与 captcha 等模块的 redis_connection 模式一致。
// 未找到指定连接时返回错误，避免以空地址静默创建不可用的队列客户端。
func NewConfigFromApp(cfg *config.Config, conn ...string) (Config, error) {
	name := ""
	if len(conn) > 0 {
		name = conn[0]
	}
	if name == "" {
		name = cfg.Redis.Default
	}
	if name == "" {
		name = "redis"
	}

	rc, ok := cfg.Redis.Connections[name]
	if !ok {
		return Config{}, ErrRedisConnectionNotFound
	}

	c := DefaultConfig()
	c.Addr = rc.Addr
	c.Username = rc.Username
	c.Password = rc.Password
	c.DB = rc.DB
	c.PoolSize = rc.PoolSize
	return c, nil
}

// connOpt 返回 asynq 的 Redis 连接选项，供本包内部各组件共用。
func (c Config) connOpt() asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:     c.Addr,
		Username: c.Username,
		Password: c.Password,
		DB:       c.DB,
		PoolSize: c.PoolSize,
	}
}

func (c Config) redisClient() goredis.UniversalClient {
	return c.connOpt().MakeRedisClient().(goredis.UniversalClient)
}

func (c Config) maxPayloadBytes() int {
	if c.MaxPayloadBytes > 0 {
		return c.MaxPayloadBytes
	}
	return 64 * 1024
}

// 任务的逻辑状态（字符串形态，供外部筛选与展示，与 asynq 状态一一对应）。
const (
	StateActive      = "active"      // 正在执行
	StatePending     = "pending"     // 等待执行
	StateScheduled   = "scheduled"   // 已调度，延迟到未来执行
	StateRetry       = "retry"       // 失败后等待下次重试
	StateArchived    = "archived"    // 重试超限的死信，等待人工处理
	StateCompleted   = "completed"   // 执行成功，按 Retention 保留
	StateAggregating = "aggregating" // 聚合中（本封装未启用聚合，列出仅为完整）
)

// parseState 将字符串状态转换为 asynq 状态枚举。
func parseState(s string) (asynq.TaskState, bool) {
	switch s {
	case StateActive:
		return asynq.TaskStateActive, true
	case StatePending:
		return asynq.TaskStatePending, true
	case StateScheduled:
		return asynq.TaskStateScheduled, true
	case StateRetry:
		return asynq.TaskStateRetry, true
	case StateArchived:
		return asynq.TaskStateArchived, true
	case StateCompleted:
		return asynq.TaskStateCompleted, true
	case StateAggregating:
		return asynq.TaskStateAggregating, true
	}
	return 0, false
}

// GetTaskID 从处理器上下文中取当前任务 ID；不在队列执行环境中时返回空串。
func GetTaskID(ctx context.Context) string {
	if id, ok := ctx.Value(runtimeTaskIDKey{}).(string); ok {
		return id
	}
	id, _ := asynq.GetTaskID(ctx)
	return id
}

// GetAttempt 从处理器上下文中取当前为第几次尝试（首次执行为 0）。
func GetAttempt(ctx context.Context) int {
	if n, ok := ctx.Value(runtimeAttemptKey{}).(int); ok {
		return n
	}
	n, _ := asynq.GetRetryCount(ctx)
	return n
}

// workerCount 返回生效的 worker 并发数。
func (c Config) workerCount() int {
	if c.Concurrency > 0 {
		return c.Concurrency
	}
	return runtime.NumCPU()
}

// queuePriorities 返回生效的队列优先级表；未配置时仅消费 default 队列。
func (c Config) queuePriorities() map[string]int {
	if len(c.Queues) > 0 {
		return c.Queues
	}
	return map[string]int{"default": 10}
}

// Ping 检查配置指向的 Redis 是否可连通。
// 建议在应用启动阶段调用以快速失败：Redis 不可达时尽早暴露配置错误，
// 而不是等到首次入队/消费才发现。
func (c Config) Ping(ctx context.Context) error {
	dialTimeout := 3 * time.Second
	client := goredis.NewClient(&goredis.Options{
		Addr:        c.Addr,
		Username:    c.Username,
		Password:    c.Password,
		DB:          c.DB,
		DialTimeout: dialTimeout,
	})
	defer client.Close()
	return client.Ping(ctx).Err()
}
