package queue

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/hibiken/asynq"
	"github.com/wuwuseo/cmf/log"
	"go.uber.org/zap"
)

// Server 任务执行端：常驻 worker 从 Redis 认领任务并调用注册的 Handler。
type Server struct {
	srv             *asynq.Server
	mux             *asynq.ServeMux
	mu              sync.RWMutex
	names           []string // 已注册的任务名（按注册顺序去重，供 RegisteredTasks 输出）
	maxPayloadBytes int
}

// NewServer 创建执行端。与 Client 使用同一份 Config，保证指向同一 Redis。
func NewServer(cfg Config) *Server {
	asynqCfg := asynq.Config{
		Concurrency:       cfg.workerCount(),
		Queues:            cfg.queuePriorities(),
		StrictPriority:    cfg.StrictPriority,
		TaskCheckInterval: cfg.TaskCheckInterval,
		// 处理器返回错误时记录日志；任务的重试/死信流转由 asynq 自行管理
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			log.Error("队列任务执行失败",
				zap.String("task_id", GetTaskID(ctx)),
				zap.String("task_name", task.Type()),
				zap.Int("attempt", GetAttempt(ctx)),
				zap.Error(err),
			)
		}),
	}
	return &Server{
		srv:             asynq.NewServer(cfg.connOpt(), asynqCfg),
		mux:             asynq.NewServeMux(),
		maxPayloadBytes: cfg.maxPayloadBytes(),
	}
}

// Handle 注册任务处理器。重复注册同名任务时后者覆盖前者。
// Start 之后注册依然生效（ServeMux 内部有锁保护）。
func (s *Server) Handle(name string, h Handler) {
	if name == "" || h == nil {
		return
	}
	s.mu.Lock()
	if _, dup := s.index(name); !dup {
		s.names = append(s.names, name)
	}
	s.mu.Unlock()

	s.mux.HandleFunc(name, func(ctx context.Context, t *asynq.Task) error {
		if len(t.Payload()) > s.maxPayloadBytes {
			return fmt.Errorf("%w: %v", asynq.SkipRetry, ErrPayloadTooLarge)
		}
		return h(ctx, &Task{Name: t.Type(), Payload: t.Payload()})
	})
}

// index 返回任务名在注册表中的下标（调用方须持有 mu）。
func (s *Server) index(name string) (int, bool) {
	for i, n := range s.names {
		if n == name {
			return i, true
		}
	}
	return -1, false
}

// Has 查询任务名是否已注册。
func (s *Server) Has(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.index(name)
	return ok
}

// RegisteredTasks 返回所有已注册的任务名（字典序）。
func (s *Server) RegisteredTasks() []string {
	s.mu.RLock()
	names := append([]string(nil), s.names...)
	s.mu.RUnlock()
	sort.Strings(names)
	return names
}

// Start 非阻塞启动 worker。重复调用返回错误。
func (s *Server) Start() error {
	return s.srv.Start(s.mux)
}

// Shutdown 停止认领并等待在途任务，等待预算由 asynq 的 ShutdownTimeout 控制。
func (s *Server) Shutdown() {
	s.srv.Shutdown()
}

// Ping 检查执行端与 Redis 的连通性。
func (s *Server) Ping() error {
	return s.srv.Ping()
}
