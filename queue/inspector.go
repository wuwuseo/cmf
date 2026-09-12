package queue

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	goredis "github.com/redis/go-redis/v9"
)

// TaskInfo 任务信息（管理/观测视角），字段为 asynq TaskInfo 的稳定子集。
type TaskInfo struct {
	ID            string        // 任务 ID
	Queue         string        // 所属队列
	Name          string        // 处理器名
	Payload       []byte        // 任务载荷
	State         string        // 状态（见 State* 常量）
	MaxRetry      int           // 最大重试次数
	Attempt       int           // 已重试次数
	LastError     string        // 最近一次失败原因
	LastFailedAt  time.Time     // 最近一次失败时间；零值表示无失败记录
	NextProcessAt time.Time     // 下次执行时间；零值表示不适用
	CompletedAt   time.Time     // 成功完成时间；零值表示未完成
	Timeout       time.Duration // 单任务处理超时
	IsOrphaned    bool          // 是否为孤儿任务（worker 崩溃后遗留的 active 任务）
}

// QueueStats 队列统计快照。
type QueueStats struct {
	Queue       string        // 队列名
	Size        int           // 任务总数（各状态之和）
	Pending     int           // 等待执行数
	Active      int           // 执行中数
	Scheduled   int           // 延迟调度数
	Retry       int           // 待重试数
	Archived    int           // 死信数
	Completed   int           // 已完成（保留期内）数
	MemoryUsage int64         // 队列近似内存占用（字节）
	Latency     time.Duration // 队列延迟（最旧 pending 任务的等待时长）
	Processed   int           // 今日已处理数
	Failed      int           // 今日失败数
}

// Inspector 管理/观测端：任务列表、统计、重试、删除、取消等操作，
// 供管理后台 API 使用。并发安全。
type Inspector struct {
	insp  *asynq.Inspector
	redis goredis.UniversalClient
}

// NewInspector 创建管理端实例。使用完毕需调用 Close。
func NewInspector(cfg Config) *Inspector {
	rc := cfg.redisClient()
	return &Inspector{insp: asynq.NewInspectorFromRedisClient(rc), redis: rc}
}

// Close 释放底层连接。
func (i *Inspector) Close() error {
	return i.redis.Close()
}

// Queues 列出当前存在任务的队列名。
func (i *Inspector) Queues() ([]string, error) {
	return i.insp.Queues()
}

// QueueStats 返回指定队列的统计快照。
func (i *Inspector) QueueStats(queue string) (QueueStats, error) {
	info, err := i.insp.GetQueueInfo(queue)
	if err != nil {
		return QueueStats{}, mapAsynqError(err)
	}
	return QueueStats{
		Queue:       info.Queue,
		Size:        info.Size,
		Pending:     info.Pending,
		Active:      info.Active,
		Scheduled:   info.Scheduled,
		Retry:       info.Retry,
		Archived:    info.Archived,
		Completed:   info.Completed,
		MemoryUsage: info.MemoryUsage,
		Latency:     info.Latency,
		Processed:   info.Processed,
		Failed:      info.Failed,
	}, nil
}

// AllQueueStats 返回当前所有队列的统计快照（按队列名遍历）。
func (i *Inspector) AllQueueStats(ctx context.Context) ([]QueueStats, error) {
	names, err := i.insp.Queues()
	if err != nil {
		return nil, mapAsynqError(err)
	}
	stats := make([]QueueStats, 0, len(names))
	for _, name := range names {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}
		s, err := i.QueueStats(name)
		if err != nil {
			continue // 单个队列查询失败不阻断整体快照
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// ListTasks 按状态分页列出任务。queue 为空表示 default 队列；
// state 取 State* 常量之一；page 从 1 开始，pageSize <= 0 时取 20。
// 返回的总数取自队列统计中该状态的计数。
func (i *Inspector) ListTasks(state, queue string, page, pageSize int) ([]TaskInfo, int, error) {
	if queue == "" {
		queue = "default"
	}
	st, ok := parseState(state)
	if !ok {
		return nil, 0, ErrInvalidTaskState
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// 总数从队列统计中取该状态的计数，避免为计数而全量拉取
	total := 0
	if qs, err := i.QueueStats(queue); err == nil {
		total = stateCount(qs, st)
	}

	opts := []asynq.ListOption{asynq.Page(page), asynq.PageSize(pageSize)}
	var (
		infos []*asynq.TaskInfo
		err   error
	)
	switch st {
	case asynq.TaskStatePending:
		infos, err = i.insp.ListPendingTasks(queue, opts...)
	case asynq.TaskStateActive:
		infos, err = i.insp.ListActiveTasks(queue, opts...)
	case asynq.TaskStateScheduled:
		infos, err = i.insp.ListScheduledTasks(queue, opts...)
	case asynq.TaskStateRetry:
		infos, err = i.insp.ListRetryTasks(queue, opts...)
	case asynq.TaskStateArchived:
		infos, err = i.insp.ListArchivedTasks(queue, opts...)
	case asynq.TaskStateCompleted:
		infos, err = i.insp.ListCompletedTasks(queue, opts...)
	default:
		return nil, 0, ErrInvalidTaskState
	}
	if err != nil {
		// 队列尚不存在时视为空列表，而非错误
		if errorsIsQueueNotFound(err) {
			return []TaskInfo{}, 0, nil
		}
		return nil, 0, mapAsynqError(err)
	}

	items := make([]TaskInfo, 0, len(infos))
	for _, info := range infos {
		items = append(items, newTaskInfo(info))
	}
	return items, total, nil
}

// stateCount 从队列统计中取指定状态的计数。
func stateCount(qs QueueStats, st asynq.TaskState) int {
	switch st {
	case asynq.TaskStatePending:
		return qs.Pending
	case asynq.TaskStateActive:
		return qs.Active
	case asynq.TaskStateScheduled:
		return qs.Scheduled
	case asynq.TaskStateRetry:
		return qs.Retry
	case asynq.TaskStateArchived:
		return qs.Archived
	case asynq.TaskStateCompleted:
		return qs.Completed
	default:
		return 0
	}
}

// errorsIsQueueNotFound 判断错误是否为"队列不存在"（队列未初始化时视为空）。
func errorsIsQueueNotFound(err error) bool {
	return mapAsynqError(err) == ErrQueueNotFound
}

// newTaskInfo 将 asynq TaskInfo 转换为本包 TaskInfo。
func newTaskInfo(info *asynq.TaskInfo) TaskInfo {
	return TaskInfo{
		ID:            info.ID,
		Queue:         info.Queue,
		Name:          info.Type,
		Payload:       info.Payload,
		State:         info.State.String(),
		MaxRetry:      info.MaxRetry,
		Attempt:       info.Retried,
		LastError:     info.LastErr,
		LastFailedAt:  info.LastFailedAt,
		NextProcessAt: info.NextProcessAt,
		CompletedAt:   info.CompletedAt,
		Timeout:       info.Timeout,
		IsOrphaned:    info.IsOrphaned,
	}
}

// GetTask 查询单个任务详情（任意状态）。
func (i *Inspector) GetTask(queue, id string) (TaskInfo, error) {
	info, err := i.insp.GetTaskInfo(queue, id)
	if err != nil {
		return TaskInfo{}, mapAsynqError(err)
	}
	return newTaskInfo(info), nil
}

// Retry 立即执行一个处于 scheduled/retry/archived 状态的任务；
// 对 pending 任务无意义，返回 ErrTaskStateConflict。
func (i *Inspector) Retry(queue, id string) error {
	if err := i.insp.RunTask(queue, id); err != nil {
		return mapAsynqError(err)
	}
	return nil
}

// Delete 从队列中删除任务（仅限 pending/scheduled/retry/archived/completed，
// 执行中的任务不可删除）。
func (i *Inspector) Delete(queue, id string) error {
	return mapAsynqError(i.insp.DeleteTask(queue, id))
}

// Cancel 取消执行中的任务（发出取消信号，处理器可通过 ctx 感知并提前退出；
// 任务随后按失败处理进入重试/死信流转）。
func (i *Inspector) Cancel(id string) error {
	return mapAsynqError(i.insp.CancelProcessing(id))
}

// PruneArchived 按归档时间清理过期死信，返回清理数量。
func (i *Inspector) PruneArchived(queue string, cutoff time.Time) (int, error) {
	return i.PruneArchivedContext(context.Background(), queue, cutoff)
}

// asynq v0.26 使用 archived zset 的 score 记录归档时间。限定每批 100 条，
// 在 Redis 内原子选择并删除，避免偏移分页漏删及误删并发重试后的任务；不读取载荷。
var pruneArchivedScript = goredis.NewScript(`
local ids = redis.call("ZRANGEBYSCORE", KEYS[1], "-inf", ARGV[1], "LIMIT", 0, 100)
for _, id in ipairs(ids) do
    local task = ARGV[2] .. id
    if redis.call("HGET", task, "state") == "archived" then
        local unique = redis.call("HGET", task, "unique_key")
        if unique and unique ~= "" and redis.call("GET", unique) == id then
            redis.call("DEL", unique)
        end
        redis.call("DEL", task)
    end
    redis.call("ZREM", KEYS[1], id)
end
return #ids
`)

// PruneArchivedContext 允许关闭流程中断清理；不存在的队列视为空队列。
func (i *Inspector) PruneArchivedContext(ctx context.Context, queue string, cutoff time.Time) (int, error) {
	if strings.TrimSpace(queue) == "" {
		return 0, fmt.Errorf("queue: 队列名不能为空")
	}
	prefix := "asynq:{" + queue + "}:"
	removed := 0
	for {
		if err := ctx.Err(); err != nil {
			return removed, err
		}
		n, err := pruneArchivedScript.Run(ctx, i.redis, []string{prefix + "archived"}, cutoff.Unix(), prefix+"t:").Int()
		if err != nil {
			return removed, err
		}
		removed += n
		if n < 100 {
			return removed, nil
		}
	}
}
