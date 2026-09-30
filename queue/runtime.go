package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/wuwuseo/cmf/config"
)

type runtimeTaskIDKey struct{}
type runtimeAttemptKey struct{}
type runtimeQueueKey struct{}

const maxNativeRetry = 100

func nativeRetryLimit(n int) int {
	if n < 0 {
		return 0
	}
	if n > maxNativeRetry {
		return maxNativeRetry
	}
	return n
}

// Capabilities describes the operations which have their documented semantics
// on the selected broker. Unsupported actions must never be silently ignored.
type Capabilities struct {
	Driver         string `json:"driver"`
	TaskList       bool   `json:"task_list"`
	TaskGet        bool   `json:"task_get"`
	TaskStats      bool   `json:"task_stats"`
	TaskRetry      bool   `json:"task_retry"`
	TaskDelete     bool   `json:"task_delete"`
	TaskCancel     bool   `json:"task_cancel"`
	Delay          bool   `json:"delay"`
	Unique         bool   `json:"unique"`
	MaxRetry       bool   `json:"max_retry"`
	Retention      bool   `json:"retention"`
	StrictPriority bool   `json:"strict_priority"`
	QueuePause     bool   `json:"queue_pause"`
	RawMessageGet  bool   `json:"raw_message_get"`
}

// QueueOverview holds broker observations. Nil counts mean unavailable, not 0.
type QueueOverview struct {
	Queue            string            `json:"queue"`
	Source           string            `json:"source"`
	Ready            *int64            `json:"ready"`
	Active           *int64            `json:"active"`
	Consumers        *int64            `json:"consumers"`
	Lag              *int64            `json:"lag"`
	Partitions       *int64            `json:"partitions"`
	ConsumerOffset   *int64            `json:"consumer_offset"`
	EndOffset        *int64            `json:"end_offset"`
	Paused           *bool             `json:"paused"`
	Connected        *bool             `json:"connected"`
	PartitionOffsets []PartitionOffset `json:"partition_offsets"`
	Healthy          bool              `json:"healthy"`
	Error            string            `json:"error,omitempty"`
}

type PartitionOffset struct {
	Partition int32  `json:"partition"`
	Committed *int64 `json:"committed"`
	End       *int64 `json:"end"`
	Lag       *int64 `json:"lag"`
}

func unavailableOverviews(queues []string, source string, err error) []QueueOverview {
	out := make([]QueueOverview, 0, len(queues))
	for _, queue := range queues {
		out = append(out, QueueOverview{Queue: queue, Source: source, Error: err.Error()})
	}
	return out
}

type RawMessage struct {
	Queue    string `json:"queue"`
	Sequence uint64 `json:"sequence"`
	Payload  []byte `json:"payload"`
}

type RuntimeConfig struct {
	Driver, Namespace                                                                 string
	Legacy                                                                            Config
	Queues                                                                            map[string]int
	Concurrency, MaxPayloadBytes                                                      int
	MaxRetry                                                                          int
	Timeout                                                                           time.Duration
	RabbitMQURL, RabbitMQManagementURL, NATSURL, NSQD, NSQLookupd, NSQHTTP, AMQP10URL string
	KafkaBrokers                                                                      []string
	KafkaGroup                                                                        string
	KafkaPartitions                                                                   int32
	KafkaReplicationFactor                                                            int16
}

func RuntimeConfigFromApp(cfg *config.Config) (RuntimeConfig, error) {
	driver := canonicalDriver(cfg.Queue.Driver)
	if driver == "" {
		driver = "asynq"
	}
	rc := RuntimeConfig{
		Driver: driver, Namespace: cfg.Queue.Namespace, Queues: cfg.Queue.Queues,
		Concurrency: cfg.Queue.Concurrency, MaxPayloadBytes: cfg.Queue.MaxPayloadBytes, MaxRetry: cfg.Queue.MaxRetry,
		Timeout:     time.Duration(cfg.Queue.Timeout) * time.Second,
		RabbitMQURL: cfg.Queue.RabbitMQ.URL, RabbitMQManagementURL: cfg.Queue.RabbitMQ.ManagementURL, NATSURL: cfg.Queue.NATS.URL,
		NSQD: cfg.Queue.NSQ.NSQD, NSQLookupd: cfg.Queue.NSQ.Lookupd, NSQHTTP: cfg.Queue.NSQ.HTTP,
		KafkaBrokers: cfg.Queue.Kafka.Brokers, KafkaGroup: cfg.Queue.Kafka.Group,
		KafkaPartitions: cfg.Queue.Kafka.Partitions, KafkaReplicationFactor: cfg.Queue.Kafka.ReplicationFactor,
		AMQP10URL: cfg.Queue.AMQP10.URL,
	}
	if rc.Namespace == "" {
		rc.Namespace = "admin"
	}
	if len(rc.Queues) == 0 {
		rc.Queues = map[string]int{"default": 10}
	}
	if rc.Concurrency <= 0 {
		rc.Concurrency = 4
	}
	if rc.MaxPayloadBytes <= 0 {
		rc.MaxPayloadBytes = 64 * 1024
	}
	if rc.MaxRetry < 0 || (driver != "asynq" && rc.MaxRetry > maxNativeRetry) {
		return RuntimeConfig{}, fmt.Errorf("queue: max_retry 必须在 0 到 %d 之间", maxNativeRetry)
	}
	if driver == "asynq" {
		legacy, err := NewConfigFromApp(cfg, cfg.Queue.RedisConnection)
		if err != nil {
			return RuntimeConfig{}, err
		}
		legacy.Concurrency = rc.Concurrency
		legacy.Queues = rc.Queues
		legacy.StrictPriority = cfg.Queue.StrictPriority
		if cfg.Queue.MaxRetry > 0 {
			legacy.MaxRetry = cfg.Queue.MaxRetry
		}
		if rc.Timeout > 0 {
			legacy.Timeout = rc.Timeout
		}
		if cfg.Queue.Retention > 0 {
			legacy.Retention = time.Duration(cfg.Queue.Retention) * time.Second
		}
		if cfg.Queue.MaxQueueSize > 0 {
			legacy.MaxQueueSize = cfg.Queue.MaxQueueSize
		}
		legacy.MaxPayloadBytes = rc.MaxPayloadBytes
		rc.Legacy = legacy
	} else if cfg.Queue.StrictPriority || cfg.Queue.Retention > 0 || (driver == "amqp10" && cfg.Queue.MaxRetry > 0) || cfg.Queue.MaxQueueSize > 0 || cfg.Queue.EnableMonitor {
		return RuntimeConfig{}, fmt.Errorf("%w: strict_priority、retention、max_queue_size、enable_monitor 仅适用于 asynq；amqp10 不支持 max_retry", ErrUnsupportedCapability)
	}
	return rc, nil
}

type notification struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Payload  []byte `json:"payload"`
	Timeout  int64  `json:"timeout_ns,omitempty"`
	MaxRetry int    `json:"max_retry"`
}

func nativeNotification(body []byte, defaultRetry int) (notification, error) {
	var n notification
	if err := json.Unmarshal(body, &n); err != nil {
		return notification{}, err
	}
	var retryField struct {
		MaxRetry *int `json:"max_retry"`
	}
	if err := json.Unmarshal(body, &retryField); err != nil {
		return notification{}, err
	}
	if retryField.MaxRetry == nil {
		n.MaxRetry = defaultRetry
	}
	return n, nil
}

type nativeBackend interface {
	Capabilities() Capabilities
	Publish(context.Context, string, []byte) error
	Consume(context.Context, string, func(context.Context, []byte) error) error
	Overview(context.Context, []string) ([]QueueOverview, error)
	Close() error
}

// Runtime owns exactly one queue backend and the handlers registered for it.
// Legacy Asynq constructors remain available for applications using them.
type Runtime struct {
	cfg       RuntimeConfig
	client    *Client
	server    *Server
	inspector TaskManagementAdapter
	backend   nativeBackend
	mu        sync.RWMutex
	handlers  map[string]Handler
	started   bool
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	stop      sync.Once
}

func OpenRuntime(ctx context.Context, cfg RuntimeConfig) (*Runtime, error) {
	cfg.Driver = canonicalDriver(cfg.Driver)
	if cfg.Driver == "" {
		cfg.Driver = "asynq"
	}
	if cfg.Namespace == "" {
		cfg.Namespace = "admin"
	}
	if len(cfg.Queues) == 0 {
		cfg.Queues = map[string]int{"default": 10}
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 4
	}
	if cfg.MaxPayloadBytes <= 0 {
		cfg.MaxPayloadBytes = 64 * 1024
	}
	r := &Runtime{cfg: cfg, handlers: make(map[string]Handler)}
	if cfg.Driver == "asynq" {
		if err := cfg.Legacy.Ping(ctx); err != nil {
			return nil, err
		}
		return NewLegacyRuntime(cfg.Legacy), nil
	}
	var err error
	switch cfg.Driver {
	case "rabbitmq":
		r.backend, err = openRabbitMQ(ctx, cfg)
	case "nats":
		r.backend, err = openNATS(ctx, cfg)
	case "nsq":
		r.backend, err = openNSQ(ctx, cfg)
	case "kafka":
		r.backend, err = openKafka(ctx, cfg)
	case "amqp10":
		r.backend, err = openAMQP10(ctx, cfg)
	default:
		err = fmt.Errorf("queue: 未知后端 %q", cfg.Driver)
	}
	if err != nil {
		return nil, err
	}
	if journal, ok := r.backend.(nativeTaskJournal); ok {
		r.inspector = &nativeTaskManagement{journal: journal, queues: cfg.Queues, publish: r.backend.Publish}
	}
	return r, nil
}

func canonicalDriver(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "jetstream":
		return "nats"
	case "amqp1", "amqp1.0", "amqp-1.0":
		return "amqp10"
	default:
		return strings.ToLower(strings.TrimSpace(name))
	}
}

// NewLegacyRuntime wraps the existing Asynq constructors without checking the
// connection. Callers that need startup validation should use OpenRuntime.
func NewLegacyRuntime(cfg Config) *Runtime {
	return &Runtime{
		cfg: RuntimeConfig{Driver: "asynq", Legacy: cfg, Queues: cfg.queuePriorities(),
			Concurrency: cfg.workerCount(), MaxPayloadBytes: cfg.maxPayloadBytes()},
		client: NewClient(cfg), server: NewServer(cfg), inspector: &asynqManagementAdapter{NewInspector(cfg)},
		handlers: make(map[string]Handler),
	}
}

func (r *Runtime) Driver() string { return r.cfg.Driver }
func (r *Runtime) Queues() []string {
	names := make([]string, 0, len(r.cfg.Queues))
	for name := range r.cfg.Queues {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
func (r *Runtime) LegacyConfig() (Config, bool) { return r.cfg.Legacy, r.cfg.Driver == "asynq" }
func (r *Runtime) Capabilities() Capabilities {
	if r.backend != nil {
		c := r.backend.Capabilities()
		c.Driver = r.cfg.Driver
		if r.inspector != nil {
			c.TaskList, c.TaskGet, c.TaskStats = true, true, true
			c.TaskRetry, c.TaskDelete, c.TaskCancel = true, true, true
		}
		return c
	}
	c := Capabilities{Driver: r.cfg.Driver}
	if r.cfg.Driver == "asynq" {
		c.TaskList, c.TaskGet, c.TaskStats = true, true, true
		c.TaskRetry, c.TaskDelete, c.TaskCancel = true, true, true
		c.Delay, c.Unique, c.MaxRetry, c.Retention, c.StrictPriority = true, true, true, true, true
	}
	return c
}
func (r *Runtime) Handle(name string, h Handler) {
	if name == "" || h == nil {
		return
	}
	r.mu.Lock()
	r.handlers[name] = h
	r.mu.Unlock()
	if r.server != nil {
		r.server.Handle(name, h)
	}
}
func (r *Runtime) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.handlers[name]
	return ok
}
func (r *Runtime) RegisteredTasks() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
func (r *Runtime) Start() error {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return errors.New("queue: runtime 已启动")
	}
	r.started = true
	r.mu.Unlock()
	if r.server != nil {
		return r.server.Start()
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	if management, ok := r.inspector.(*nativeTaskManagement); ok {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for ctx.Err() == nil {
				recoveryCtx, done := context.WithTimeout(ctx, 20*time.Second)
				_ = management.Recover(recoveryCtx)
				done()
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	}
	for _, queue := range r.Queues() {
		r.wg.Add(1)
		go func(name string) {
			defer r.wg.Done()
			for ctx.Err() == nil {
				_ = r.backend.Consume(ctx, name, func(taskCtx context.Context, body []byte) error {
					return r.execute(context.WithValue(taskCtx, runtimeQueueKey{}, name), body)
				})
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
				}
			}
		}(queue)
	}
	return nil
}
func (r *Runtime) execute(ctx context.Context, body []byte) error {
	n, err := nativeNotification(body, r.cfg.MaxRetry)
	if err != nil {
		return err
	}
	if n.ID == "" || n.Name == "" || len(n.Payload) > r.cfg.MaxPayloadBytes {
		return errors.New("queue: 非法通知")
	}
	queue, _ := ctx.Value(runtimeQueueKey{}).(string)
	var token string
	management, managed := r.inspector.(*nativeTaskManagement)
	if managed {
		attempt, _ := ctx.Value(runtimeAttemptKey{}).(int)
		var accepted bool
		var err error
		token, accepted, err = management.Claim(ctx, queue, n, attempt)
		if err != nil {
			return err
		}
		if !accepted {
			return nil // deleted, completed, or a duplicate broker delivery
		}
	}
	r.mu.RLock()
	h := r.handlers[n.Name]
	r.mu.RUnlock()
	var handlerErr error
	if h == nil {
		handlerErr = ErrUnknownTask
	}
	ctx = context.WithValue(ctx, runtimeTaskIDKey{}, n.ID)
	if _, ok := ctx.Value(runtimeAttemptKey{}).(int); !ok {
		ctx = context.WithValue(ctx, runtimeAttemptKey{}, 0)
	}
	brokerCtx := ctx
	if n.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(n.Timeout))
		defer cancel()
	}
	var wasCanceled atomic.Bool
	if managed {
		var cancelTask context.CancelFunc
		ctx, cancelTask = context.WithCancel(ctx)
		defer cancelTask()
		pollCtx, stopPoll := context.WithCancel(ctx)
		pollDone := make(chan struct{})
		go func() {
			defer close(pollDone)
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			heartbeatAt := time.Now()
			for {
				select {
				case <-pollCtx.Done():
					return
				case <-ticker.C:
					checkCtx, done := context.WithTimeout(pollCtx, 10*time.Second)
					canceled, err := management.canceled(checkCtx, queue, n.ID, token)
					done()
					if err == nil && canceled {
						wasCanceled.Store(true)
						cancelTask()
						return
					}
					if time.Since(heartbeatAt) >= 30*time.Second {
						hbCtx, done := context.WithTimeout(pollCtx, 10*time.Second)
						_ = management.Heartbeat(hbCtx, queue, n.ID, token)
						done()
						heartbeatAt = time.Now()
					}
				}
			}
		}()
		defer func() { stopPoll(); <-pollDone }()
	}
	if handlerErr == nil {
		handlerErr = h(ctx, &Task{Name: n.Name, Payload: n.Payload})
	}
	if handlerErr == nil && ctx.Err() != nil {
		handlerErr = ctx.Err()
	}
	if !managed {
		return handlerErr
	}
	if brokerCtx.Err() != nil {
		return brokerCtx.Err()
	}
	checkCtx, checkDone := context.WithTimeout(context.Background(), 15*time.Second)
	canceled, checkErr := management.canceled(checkCtx, queue, n.ID, token)
	checkDone()
	if checkErr != nil {
		return checkErr
	}
	if canceled {
		wasCanceled.Store(true)
	}
	if wasCanceled.Load() {
		handlerErr = context.Canceled
	}
	finishCtx, done := context.WithTimeout(context.Background(), 15*time.Second)
	defer done()
	attempt, _ := ctx.Value(runtimeAttemptKey{}).(int)
	if err := management.Finish(finishCtx, queue, n.ID, token, attempt, handlerErr); err != nil {
		return err
	}
	return handlerErr
}
func (r *Runtime) Enqueue(ctx context.Context, name string, payload []byte, opts ...EnqueueOption) (string, error) {
	if r.client != nil {
		return r.client.Enqueue(ctx, name, payload, opts...)
	}
	if len(payload) > r.cfg.MaxPayloadBytes {
		return "", ErrPayloadTooLarge
	}
	o := &enqueueOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
	if o.processIn < 0 || (o.processIn != 0 && r.cfg.Driver != "nsq" && !r.Capabilities().Delay) || (r.cfg.Driver == "amqp10" && o.maxRetry != nil) || o.retention != 0 || o.uniqueTTL != 0 || o.taskID != "" {
		return "", ErrUnsupportedCapability
	}
	maxRetry := r.cfg.MaxRetry
	if o.maxRetry != nil {
		maxRetry = *o.maxRetry
	}
	if maxRetry < 0 || maxRetry > maxNativeRetry {
		return "", fmt.Errorf("queue: max_retry 必须在 0 到 %d 之间", maxNativeRetry)
	}
	queue := o.queue
	if queue == "" {
		if _, ok := r.cfg.Queues["default"]; ok {
			queue = "default"
		} else {
			queue = r.Queues()[0]
		}
	}
	if _, ok := r.cfg.Queues[queue]; !ok {
		return "", ErrQueueNotFound
	}
	timeout := o.timeout
	if timeout == 0 {
		timeout = r.cfg.Timeout
	}
	id := uuid.NewString()
	n := notification{ID: id, Name: name, Payload: payload, Timeout: int64(timeout), MaxRetry: maxRetry}
	body, err := json.Marshal(n)
	if err != nil {
		return "", err
	}
	management, managed := r.inspector.(*nativeTaskManagement)
	if managed {
		var due time.Time
		if o.processIn > 0 {
			due = time.Now().Add(o.processIn)
		}
		if err := management.Create(ctx, queue, n, due); err != nil {
			return "", err
		}
		if !due.IsZero() {
			return id, nil
		}
	}
	var publishErr error
	if o.processIn > 0 {
		deferred, ok := r.backend.(interface {
			PublishDelayed(context.Context, string, []byte, time.Duration) error
		})
		if !ok {
			return "", ErrUnsupportedCapability
		}
		publishErr = deferred.PublishDelayed(ctx, queue, body, o.processIn)
	} else {
		publishErr = r.backend.Publish(ctx, queue, body)
	}
	if publishErr != nil {
		if managed {
			_ = management.Delete(queue, id)
		}
		return "", publishErr
	}
	if managed {
		// The broker has confirmed the delivery. A failed dispatch marker is
		// recovered by the periodic replay and can only create a duplicate.
		_ = management.MarkDispatched(ctx, queue, id)
	}
	return id, nil
}
func (r *Runtime) Overview(ctx context.Context) ([]QueueOverview, error) {
	if r.backend != nil {
		return r.backend.Overview(ctx, r.Queues())
	}
	out := make([]QueueOverview, 0, len(r.cfg.Queues))
	for _, name := range r.Queues() {
		info, err := r.inspector.QueueStats(name)
		if err != nil {
			out = append(out, QueueOverview{Queue: name, Source: "asynq", Error: err.Error()})
			continue
		}
		ready, active := int64(info.Pending), int64(info.Active)
		out = append(out, QueueOverview{Queue: name, Source: "asynq", Ready: &ready, Active: &active, Healthy: true})
	}
	return out, nil
}
func (r *Runtime) ListTasks(state, queue string, page, pageSize int) ([]TaskInfo, int, error) {
	if r.inspector == nil {
		return nil, 0, ErrUnsupportedCapability
	}
	return r.inspector.ListTasks(state, queue, page, pageSize)
}
func (r *Runtime) GetTask(queue, id string) (TaskInfo, error) {
	if r.inspector == nil {
		return TaskInfo{}, ErrUnsupportedCapability
	}
	return r.inspector.GetTask(queue, id)
}
func (r *Runtime) Retry(queue, id string) error {
	if r.inspector == nil {
		return ErrUnsupportedCapability
	}
	return r.inspector.Retry(queue, id)
}
func (r *Runtime) Delete(queue, id string) error {
	if r.inspector == nil {
		return ErrUnsupportedCapability
	}
	return r.inspector.Delete(queue, id)
}
func (r *Runtime) Cancel(id string) error {
	if r.inspector == nil {
		return ErrUnsupportedCapability
	}
	return r.inspector.Cancel(id)
}
func (r *Runtime) AllQueueStats(ctx context.Context) ([]QueueStats, error) {
	if r.inspector == nil {
		return nil, ErrUnsupportedCapability
	}
	return r.inspector.AllQueueStats(ctx)
}
func (r *Runtime) PauseQueue(ctx context.Context, queue string, pause bool) error {
	if !r.Capabilities().QueuePause {
		return ErrUnsupportedCapability
	}
	p, ok := r.backend.(interface {
		Pause(context.Context, string, bool) error
	})
	if !ok {
		return ErrUnsupportedCapability
	}
	if _, exists := r.cfg.Queues[queue]; !exists {
		return ErrQueueNotFound
	}
	return p.Pause(ctx, queue, pause)
}
func (r *Runtime) GetRawMessage(ctx context.Context, queue string, sequence uint64) (RawMessage, error) {
	if !r.Capabilities().RawMessageGet {
		return RawMessage{}, ErrUnsupportedCapability
	}
	g, ok := r.backend.(interface {
		GetRaw(context.Context, string, uint64) (RawMessage, error)
	})
	if !ok {
		return RawMessage{}, ErrUnsupportedCapability
	}
	if _, exists := r.cfg.Queues[queue]; !exists {
		return RawMessage{}, ErrQueueNotFound
	}
	return g.GetRaw(ctx, queue, sequence)
}
func (r *Runtime) Shutdown(ctx context.Context) {
	r.stop.Do(func() {
		if r.cancel != nil {
			r.cancel()
		}
		if r.server != nil {
			done := make(chan struct{})
			go func() { r.server.Shutdown(); close(done) }()
			select {
			case <-done:
			case <-ctx.Done():
			}
		}
		done := make(chan struct{})
		go func() { r.wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-ctx.Done():
		}
		if r.backend != nil {
			_ = r.backend.Close()
		}
		if r.client != nil {
			_ = r.client.Close()
		}
		if r.inspector != nil {
			_ = r.inspector.Close()
		}
	})
}
