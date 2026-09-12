package queue

import (
	"context"
	"slices"
	"sort"
	"time"

	"github.com/hibiken/asynq"
	goredis "github.com/redis/go-redis/v9"
)

// enqueueOptions 汇总一次入队调用的可选项。
type enqueueOptions struct {
	queue     string        // 目标队列名；空串表示使用默认队列
	processIn time.Duration // 延迟执行的时长；0 表示立即可执行
	maxRetry  *int          // 最大重试次数；nil 表示使用配置默认值
	timeout   time.Duration // 单任务处理超时；0 表示不设置
	retention time.Duration // completed 保留时长；0 表示使用配置默认值
	uniqueTTL time.Duration // 去重窗口；>0 时窗口内相同任务不重复入队
	taskID    string        // 自定义任务 ID；为空时由 asynq 生成 UUID
}

// EnqueueOption 定制单次入队行为的选项函数。
type EnqueueOption func(*enqueueOptions)

// WithQueue 指定目标队列名（须在 Server 的 Queues 配置内，否则无人消费）。
func WithQueue(name string) EnqueueOption {
	return func(o *enqueueOptions) { o.queue = name }
}

// WithDelay 延迟 delay 后执行（任务入队后处于 scheduled 状态）。
func WithDelay(delay time.Duration) EnqueueOption {
	return func(o *enqueueOptions) { o.processIn = delay }
}

// WithMaxRetry 覆盖本次入队的最大重试次数。
func WithMaxRetry(n int) EnqueueOption {
	return func(o *enqueueOptions) { o.maxRetry = &n }
}

// WithTimeout 设置单任务处理超时，超时按失败处理并触发重试。
func WithTimeout(d time.Duration) EnqueueOption {
	return func(o *enqueueOptions) { o.timeout = d }
}

// WithRetention 设置任务成功完成后在 Redis 中保留的时长，供管理端查看历史。
func WithRetention(d time.Duration) EnqueueOption {
	return func(o *enqueueOptions) { o.retention = d }
}

// WithUnique 设置去重窗口：窗口内 Name+Payload 完全相同的任务拒绝重复入队，
// 返回 ErrDuplicateTask。
func WithUnique(ttl time.Duration) EnqueueOption {
	return func(o *enqueueOptions) { o.uniqueTTL = ttl }
}

// WithTaskID 自定义任务 ID（需保证唯一，冲突时返回 ErrTaskStateConflict）。
func WithTaskID(id string) EnqueueOption {
	return func(o *enqueueOptions) { o.taskID = id }
}

// Client 任务投递端，将任务写入 Redis。并发安全。
type Client struct {
	client          *asynq.Client
	redis           goredis.UniversalClient
	inspector       *asynq.Inspector
	admission       chan struct{}
	maxQueueSize    int
	maxPayloadBytes int
	maxRetry        int
	queue           string
	retention       time.Duration
	timeout         time.Duration
}

// NewClient 创建投递端客户端。使用完毕需调用 Close 释放 Redis 连接。
func NewClient(cfg Config) *Client {
	rc := cfg.redisClient()
	maxQueueSize := cfg.MaxQueueSize
	if maxQueueSize <= 0 {
		maxQueueSize = 10000
	}
	return &Client{
		client:          asynq.NewClientFromRedisClient(rc),
		redis:           rc,
		inspector:       asynq.NewInspectorFromRedisClient(rc),
		admission:       make(chan struct{}, 1),
		maxQueueSize:    maxQueueSize,
		maxPayloadBytes: cfg.maxPayloadBytes(),
		maxRetry:        cfg.MaxRetry,
		queue:           firstQueueName(cfg),
		retention:       cfg.Retention,
		timeout:         cfg.Timeout,
	}
}

// firstQueueName 优先 default，否则取字典序首个队列，保证多队列投递稳定。
func firstQueueName(cfg Config) string {
	queues := cfg.queuePriorities()
	if _, ok := queues["default"]; ok {
		return "default"
	}
	names := make([]string, 0, len(queues))
	for name := range queues {
		names = append(names, name)
	}
	sort.Strings(names)
	return names[0]
}

// Enqueue 入队一个任务并返回其任务 ID。
// name 为处理器名（执行端须已通过 Server.Handle 注册）；
// payload 为任务载荷（通常为 JSON 字节）；
// 未指定的选项依次回退到入队选项 -> Config 默认值。
func (c *Client) Enqueue(ctx context.Context, name string, payload []byte, opts ...EnqueueOption) (string, error) {
	if len(payload) > c.maxPayloadBytes {
		return "", ErrPayloadTooLarge
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	o := &enqueueOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}

	// 基础默认值先落位，再由显式选项覆盖
	maxRetry := c.maxRetry
	if o.maxRetry != nil {
		maxRetry = *o.maxRetry
	}
	queueName := o.queue
	if queueName == "" {
		queueName = c.queue
	}
	retention := o.retention
	if retention == 0 {
		retention = c.retention
	}
	timeout := o.timeout
	if timeout == 0 {
		timeout = c.timeout
	}

	task := asynq.NewTask(name, payload)
	asynqOpts := []asynq.Option{
		asynq.Queue(queueName),
		asynq.MaxRetry(maxRetry),
	}
	if o.processIn > 0 {
		asynqOpts = append(asynqOpts, asynq.ProcessIn(o.processIn))
	}
	if timeout > 0 {
		asynqOpts = append(asynqOpts, asynq.Timeout(timeout))
	}
	if retention > 0 {
		asynqOpts = append(asynqOpts, asynq.Retention(retention))
	}
	if o.uniqueTTL > 0 {
		asynqOpts = append(asynqOpts, asynq.Unique(o.uniqueTTL))
	}
	if o.taskID != "" {
		asynqOpts = append(asynqOpts, asynq.TaskID(o.taskID))
	}

	// ponytail: 同一 Client 串行准入；跨进程的最终内存上限由 Redis maxmemory/noeviction 保证。
	select {
	case c.admission <- struct{}{}:
		defer func() { <-c.admission }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	stats, err := c.inspector.GetQueueInfo(queueName)
	if err != nil {
		// asynq v0.26 的 GetQueueInfo 未包装 ErrQueueNotFound，显式确认队列不存在，
		// 只为首次入队放行；Redis 故障或已存在队列的统计错误必须向上传递。
		names, listErr := c.inspector.Queues()
		if listErr != nil || slices.Contains(names, queueName) {
			return "", mapAsynqError(err)
		}
	}
	if stats != nil && stats.Size >= c.maxQueueSize {
		return "", ErrQueueFull
	}
	info, err := c.client.EnqueueContext(ctx, task, asynqOpts...)
	if err != nil {
		return "", mapAsynqError(err)
	}
	return info.ID, nil
}

// Close 释放底层连接。
func (c *Client) Close() error {
	return c.redis.Close()
}
