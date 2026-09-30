package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/wuwuseo/cmf/config"
)

type stubBackend struct {
	published int
	lastBody  []byte
	deferred  int
}

func (*stubBackend) Capabilities() Capabilities { return Capabilities{} }

func (s *stubBackend) Publish(_ context.Context, _ string, body []byte) error {
	s.published++
	s.lastBody = body
	return nil
}
func (s *stubBackend) PublishDelayed(ctx context.Context, queue string, body []byte, _ time.Duration) error {
	s.deferred++
	return s.Publish(ctx, queue, body)
}
func (s *stubBackend) Consume(context.Context, string, func(context.Context, []byte) error) error {
	return nil
}
func (s *stubBackend) Overview(context.Context, []string) ([]QueueOverview, error) { return nil, nil }
func (s *stubBackend) Close() error                                                { return nil }

func TestRuntimeCapabilitiesMatchOperations(t *testing.T) {
	for _, driver := range []string{"rabbitmq", "nats", "nsq", "kafka", "amqp10"} {
		t.Run(driver, func(t *testing.T) {
			r := &Runtime{cfg: RuntimeConfig{Driver: driver, Queues: map[string]int{"default": 1}}, backend: &stubBackend{}}
			caps := r.Capabilities()
			if caps.TaskList || caps.TaskGet || caps.TaskRetry || caps.TaskDelete || caps.TaskCancel || caps.TaskStats {
				t.Fatalf("%s incorrectly advertises Asynq task operations: %+v", driver, caps)
			}
			if _, _, err := r.ListTasks("pending", "default", 1, 10); !errors.Is(err, ErrUnsupportedCapability) {
				t.Fatalf("list: %v", err)
			}
			if _, err := r.GetTask("default", "id"); !errors.Is(err, ErrUnsupportedCapability) {
				t.Fatalf("get: %v", err)
			}
			if err := r.Retry("default", "id"); !errors.Is(err, ErrUnsupportedCapability) {
				t.Fatalf("retry: %v", err)
			}
			if err := r.Delete("default", "id"); !errors.Is(err, ErrUnsupportedCapability) {
				t.Fatalf("delete: %v", err)
			}
			if err := r.Cancel("id"); !errors.Is(err, ErrUnsupportedCapability) {
				t.Fatalf("cancel: %v", err)
			}
			if _, err := r.AllQueueStats(t.Context()); !errors.Is(err, ErrUnsupportedCapability) {
				t.Fatalf("stats: %v", err)
			}
			if err := r.PauseQueue(t.Context(), "default", true); !errors.Is(err, ErrUnsupportedCapability) {
				t.Fatalf("pause: %v", err)
			}
			if _, err := r.GetRawMessage(t.Context(), "default", 1); !errors.Is(err, ErrUnsupportedCapability) {
				t.Fatalf("raw: %v", err)
			}
		})
	}
}

func TestNativeDriversDeclareOptionalManagement(t *testing.T) {
	natsRuntime := &Runtime{cfg: RuntimeConfig{Driver: "nats"}, backend: &natsBackend{}}
	if caps := natsRuntime.Capabilities(); !caps.RawMessageGet || !caps.MaxRetry {
		t.Fatalf("JetStream capabilities: %+v", caps)
	}
	nsqRuntime := &Runtime{cfg: RuntimeConfig{Driver: "nsq"}, backend: &nsqBackend{cfg: RuntimeConfig{NSQHTTP: "http://localhost:4151"}}}
	if !nsqRuntime.Capabilities().QueuePause {
		t.Fatal("NSQ pause not declared")
	}
}

func TestRuntimeRejectsUnsupportedPublishOptionsBeforeBroker(t *testing.T) {
	backend := &stubBackend{}
	r := &Runtime{cfg: RuntimeConfig{Driver: "rabbitmq", Queues: map[string]int{"default": 1}, MaxPayloadBytes: 1024}, backend: backend}
	for _, opt := range []EnqueueOption{WithDelay(time.Second), WithRetention(time.Minute), WithUnique(time.Minute), WithTaskID("id")} {
		if _, err := r.Enqueue(t.Context(), "test", nil, opt); !errors.Is(err, ErrUnsupportedCapability) {
			t.Fatalf("option error = %v", err)
		}
	}
	if backend.published != 0 {
		t.Fatalf("unsupported options published %d messages", backend.published)
	}
	if _, err := r.Enqueue(t.Context(), "test", json.RawMessage(`{"ok":true}`), WithQueue("default")); err != nil {
		t.Fatal(err)
	}
	if backend.published != 1 {
		t.Fatalf("published = %d, want 1", backend.published)
	}
	if _, err := r.Enqueue(t.Context(), "test", nil, WithMaxRetry(2)); err != nil {
		t.Fatalf("rabbitmq max retry: %v", err)
	}
	var task notification
	if err := json.Unmarshal(backend.lastBody, &task); err != nil || task.MaxRetry != 2 {
		t.Fatalf("published task = %+v, err = %v", task, err)
	}
	r.cfg.Driver = "amqp10"
	if _, err := r.Enqueue(t.Context(), "test", nil, WithMaxRetry(2)); !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("amqp10 max retry error = %v", err)
	}
	r.cfg.Driver = "nsq"
	if _, err := r.Enqueue(t.Context(), "test", nil, WithDelay(time.Second), WithMaxRetry(2)); err != nil || backend.deferred != 1 {
		t.Fatalf("nsq delayed enqueue err = %v, deferred = %d", err, backend.deferred)
	}
}

func TestRuntimeNativeConfigNeedsNoMySQL(t *testing.T) {
	cfg := &config.Config{}
	cfg.Queue.Driver = "jetstream"
	cfg.Queue.NATS.URL = "nats://127.0.0.1:4222"
	rc, err := RuntimeConfigFromApp(cfg)
	if err != nil || rc.Driver != "nats" {
		t.Fatalf("config = %+v, err = %v", rc, err)
	}
	cfg.Queue.MaxRetry = 3
	if rc, err := RuntimeConfigFromApp(cfg); err != nil || rc.MaxRetry != 3 {
		t.Fatalf("nats retry config = %+v, err = %v", rc, err)
	}
	cfg.Queue.Driver = "nsq"
	if rc, err := RuntimeConfigFromApp(cfg); err != nil || rc.MaxRetry != 3 {
		t.Fatalf("nsq retry config = %+v, err = %v", rc, err)
	}
	cfg.Queue.Driver = "kafka"
	if rc, err := RuntimeConfigFromApp(cfg); err != nil || rc.MaxRetry != 3 {
		t.Fatalf("kafka retry config = %+v, err = %v", rc, err)
	}
	cfg.Queue.Driver = "amqp10"
	if _, err := RuntimeConfigFromApp(cfg); !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("amqp10 retry config error = %v", err)
	}
	cfg.Queue.Driver = "rabbitmq"
	if rc, err := RuntimeConfigFromApp(cfg); err != nil || rc.MaxRetry != 3 {
		t.Fatalf("rabbitmq retry config = %+v, err = %v", rc, err)
	}
}

func TestRabbitRetryDestination(t *testing.T) {
	b := &rabbitBackend{cfg: RuntimeConfig{Namespace: "admin"}}
	for _, tc := range []struct {
		attempt, maxRetry int
		want              string
	}{
		{0, 0, "admin.default.dead"},
		{0, 2, "admin.default"},
		{1, 2, "admin.default"},
		{2, 2, "admin.default.dead"},
	} {
		if got := b.retryDestination("default", tc.attempt, tc.maxRetry); got != tc.want {
			t.Errorf("attempt=%d maxRetry=%d: got %s, want %s", tc.attempt, tc.maxRetry, got, tc.want)
		}
	}
}

func TestRuntimeNativeAttemptInHandlerContext(t *testing.T) {
	r := &Runtime{cfg: RuntimeConfig{MaxPayloadBytes: 1024}, handlers: make(map[string]Handler)}
	called := false
	r.Handle("probe", func(ctx context.Context, _ *Task) error {
		called = true
		if got := GetAttempt(ctx); got != 2 {
			t.Errorf("attempt = %d, want 2", got)
		}
		return nil
	})
	body, err := json.Marshal(notification{ID: "task-1", Name: "probe"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(t.Context(), runtimeAttemptKey{}, 2)
	if err := r.execute(ctx, body); err != nil || !called {
		t.Fatalf("execute err = %v, called = %v", err, called)
	}
}

func TestKafkaAttemptHeader(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int
	}{
		{"", 0}, {"2", 2}, {"-1", 0}, {"invalid", 0},
	} {
		got := kafkaAttempt([]kgo.RecordHeader{{Key: "x-cmf-attempt", Value: []byte(tc.raw)}})
		if got != tc.want {
			t.Errorf("header %q: got %d, want %d", tc.raw, got, tc.want)
		}
	}
}

func TestQueueOverviewUnknownCountsRemainNull(t *testing.T) {
	data, err := json.Marshal(QueueOverview{Queue: "default", Source: "amqp10", Healthy: true})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ready", "active", "consumers", "lag", "partitions", "consumer_offset", "end_offset", "partition_offsets", "paused", "connected"} {
		if got[key] != nil {
			t.Fatalf("%s = %v, want null", key, got[key])
		}
	}
}
