package queue

import (
	"errors"

	"github.com/hibiken/asynq"
)

// 队列包统一错误。asynq 的底层错误在边界处转换为以下错误或其包装，
// 调用方通过 errors.Is 判断。
var (
	ErrQueueFull       = errors.New("queue: 队列已达到容量阈值")
	ErrPayloadTooLarge = errors.New("queue: 任务载荷超过字节上限")
	// ErrRedisConnectionNotFound 配置中不存在指定的 Redis 连接。
	ErrRedisConnectionNotFound = errors.New("queue: redis 连接不存在")

	// ErrUnknownTask 任务名未在任何 Server 注册（执行期不会存在对应处理器）。
	ErrUnknownTask = errors.New("queue: 任务未注册")

	// ErrInvalidTaskState 非法的状态字符串（筛选任务列表时使用）。
	ErrInvalidTaskState = errors.New("queue: 非法的任务状态")

	// ErrTaskNotFound 任务不存在（可能已被删除或过期）。
	ErrTaskNotFound = errors.New("queue: 任务不存在")

	// ErrQueueNotFound 队列不存在。
	ErrQueueNotFound = errors.New("queue: 队列不存在")

	// ErrDuplicateTask 重复入队：启用了 WithUnique 且相同唯一键的任务仍存活。
	ErrDuplicateTask = errors.New("queue: 任务重复入队")

	// ErrTaskStateConflict 任务当前状态不允许该操作（如对 pending 任务执行重试）。
	ErrTaskStateConflict = errors.New("queue: 任务状态不允许该操作")
)

// mapAsynqError 将 asynq 哨兵错误转换为本包错误，其余原样返回。
func mapAsynqError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, asynq.ErrTaskNotFound):
		return ErrTaskNotFound
	case errors.Is(err, asynq.ErrQueueNotFound):
		return ErrQueueNotFound
	case errors.Is(err, asynq.ErrDuplicateTask):
		return ErrDuplicateTask
	case errors.Is(err, asynq.ErrTaskIDConflict):
		return ErrTaskStateConflict
	default:
		return err
	}
}
