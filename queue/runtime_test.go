package queue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/wuwuseo/cmf/config"
)

type stubBackend struct{ published int }

func (*stubBackend) Capabilities() Capabilities { return Capabilities{} }

func (s *stubBackend) Publish(context.Context, string, []byte) error { s.published++; return nil }
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
	if !natsRuntime.Capabilities().RawMessageGet {
		t.Fatal("JetStream raw lookup not declared")
	}
	nsqRuntime := &Runtime{cfg: RuntimeConfig{Driver: "nsq"}, backend: &nsqBackend{cfg: RuntimeConfig{NSQHTTP: "http://localhost:4151"}}}
	if !nsqRuntime.Capabilities().QueuePause {
		t.Fatal("NSQ pause not declared")
	}
}

func TestRuntimeRejectsUnsupportedPublishOptionsBeforeBroker(t *testing.T) {
	backend := &stubBackend{}
	r := &Runtime{cfg: RuntimeConfig{Driver: "rabbitmq", Queues: map[string]int{"default": 1}, MaxPayloadBytes: 1024}, backend: backend}
	for _, opt := range []EnqueueOption{WithDelay(time.Second), WithMaxRetry(2), WithRetention(time.Minute), WithUnique(time.Minute), WithTaskID("id")} {
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
	if _, err := RuntimeConfigFromApp(cfg); !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("config option error = %v", err)
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
