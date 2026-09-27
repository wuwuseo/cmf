package queue

import "context"

// TaskManagementAdapter defines the Asynq-style task operations. A runtime
// without this adapter returns ErrUnsupportedCapability for these operations.
// Native broker metrics and operations are exposed separately by Runtime.
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
