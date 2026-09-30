package queue

import "context"

// TaskManagementAdapter defines Asynq-style task operations for backends that
// can persist and coordinate individual task state. A runtime without this
// adapter returns ErrUnsupportedCapability for these operations.
type TaskManagementAdapter interface {
	ListTasks(state, queue string, page, pageSize int) ([]TaskInfo, int, error)
	GetTask(queue, id string) (TaskInfo, error)
	QueueStats(queue string) (QueueStats, error)
	AllQueueStats(context.Context) ([]QueueStats, error)
	Retry(queue, id string) error
	Delete(queue, id string) error
	Cancel(id string) error
	Close() error
}

// asynqManagementAdapter keeps the existing Inspector behavior and response
// shape while allowing Runtime to own the single management handle.
type asynqManagementAdapter struct{ *Inspector }

var _ TaskManagementAdapter = (*asynqManagementAdapter)(nil)
