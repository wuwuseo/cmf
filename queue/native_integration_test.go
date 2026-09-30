package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"
	"github.com/twmb/franz-go/pkg/kgo"
)

// These tests run only against explicitly configured local brokers. Each run
// creates a unique namespace and verifies publish acknowledgement and retry.
func TestNativeBrokers(t *testing.T) {
	for _, tc := range []struct {
		name, env string
		configure func(*RuntimeConfig, string)
	}{
		{"rabbitmq", "QUEUE_TEST_RABBIT_URL", func(c *RuntimeConfig, address string) {
			c.RabbitMQURL = address
			c.RabbitMQManagementURL = os.Getenv("QUEUE_TEST_RABBIT_MANAGEMENT_URL")
		}},
		{"nats", "QUEUE_TEST_NATS_URL", func(c *RuntimeConfig, address string) { c.NATSURL = address }},
		{"nsq", "QUEUE_TEST_NSQD", func(c *RuntimeConfig, address string) { c.NSQD = address; c.NSQHTTP = os.Getenv("QUEUE_TEST_NSQ_HTTP") }},
		{"kafka", "QUEUE_TEST_KAFKA_BROKER", func(c *RuntimeConfig, address string) { c.KafkaBrokers = []string{address} }},
		{"amqp10", "QUEUE_TEST_AMQP10_URL", func(c *RuntimeConfig, address string) { c.AMQP10URL = address }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			address := os.Getenv(tc.env)
			if address == "" {
				t.Skipf("%s 未设置", tc.env)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
			defer cancel()
			cfg := RuntimeConfig{Driver: tc.name, Namespace: fmt.Sprintf("qt%d", time.Now().UnixNano()), Queues: map[string]int{"default": 1}, Concurrency: 2, MaxPayloadBytes: 1024}
			if tc.name == "rabbitmq" || tc.name == "nats" || tc.name == "nsq" || tc.name == "kafka" {
				cfg.MaxRetry = 1
			}
			tc.configure(&cfg, address)
			if tc.name == "amqp10" && os.Getenv("QUEUE_TEST_AMQP10_DECLARE_RABBIT") != "" {
				conn, err := amqp091.Dial(os.Getenv("QUEUE_TEST_AMQP10_DECLARE_RABBIT"))
				if err != nil {
					t.Fatal(err)
				}
				ch, err := conn.Channel()
				if err != nil {
					conn.Close()
					t.Fatal(err)
				}
				_, err = ch.QueueDeclare(cfg.Namespace+".default", true, false, false, false, nil)
				ch.Close()
				conn.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			r, err := OpenRuntime(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Shutdown(context.Background())
			var attempts atomic.Int32
			done := make(chan struct{}, 1)
			r.Handle("probe", func(context.Context, *Task) error {
				if attempts.Add(1) == 1 {
					return errors.New("retry probe")
				}
				select {
				case done <- struct{}{}:
				default:
				}
				return nil
			})
			if _, err := r.Enqueue(ctx, "probe", []byte(`{"ok":true}`)); err != nil {
				t.Fatalf("publish confirmation: %v", err)
			}
			if tc.name == "rabbitmq" || tc.name == "nats" {
				var view []QueueOverview
				for ctx.Err() == nil {
					view, err = r.Overview(ctx)
					if err == nil && len(view) == 1 && view[0].Ready != nil && *view[0].Ready == 1 {
						break
					}
					time.Sleep(100 * time.Millisecond)
				}
				if err != nil || len(view) != 1 || view[0].Ready == nil || *view[0].Ready != 1 {
					t.Fatalf("published message depth: %+v, %v", view, err)
				}
			}
			if tc.name == "nats" {
				msg, err := r.GetRawMessage(ctx, "default", 1)
				if err != nil || len(msg.Payload) == 0 {
					t.Fatalf("GetRawMessage: %+v, %v", msg, err)
				}
			}
			if err := r.Start(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatalf("redelivery: attempts=%d, err=%v", attempts.Load(), ctx.Err())
			}
			if attempts.Load() < 2 {
				t.Fatalf("attempts=%d, want redelivery", attempts.Load())
			}
			if tc.name == "rabbitmq" {
				r.Handle("poison", func(context.Context, *Task) error { return errors.New("permanent failure") })
				poisonID, err := r.Enqueue(ctx, "poison", nil, WithMaxRetry(0))
				if err != nil {
					t.Fatal(err)
				}
				conn, err := amqp091.Dial(address)
				if err != nil {
					t.Fatal(err)
				}
				ch, err := conn.Channel()
				if err != nil {
					conn.Close()
					t.Fatal(err)
				}
				found := false
				for ctx.Err() == nil {
					msg, ok, getErr := ch.Get(cfg.Namespace+".default.dead", false)
					if getErr != nil {
						t.Fatal(getErr)
					}
					if ok {
						var task notification
						if err := json.Unmarshal(msg.Body, &task); err != nil || task.ID != poisonID {
							t.Fatalf("dead task = %+v, err = %v", task, err)
						}
						if err := msg.Ack(false); err != nil {
							t.Fatal(err)
						}
						found = true
						break
					}
					time.Sleep(100 * time.Millisecond)
				}
				ch.Close()
				conn.Close()
				if !found {
					t.Fatalf("poison task %s never reached dead-letter queue: %v", poisonID, ctx.Err())
				}
			}
			if tc.name == "nats" {
				r.Handle("poison", func(context.Context, *Task) error { return errors.New("permanent failure") })
				poisonID, err := r.Enqueue(ctx, "poison", nil, WithMaxRetry(0))
				if err != nil {
					t.Fatal(err)
				}
				b := r.backend.(*natsBackend)
				found := false
				for ctx.Err() == nil {
					msg, lookupErr := b.stream.GetLastMsgForSubject(ctx, b.deadSubject("default"))
					if lookupErr == nil {
						var task notification
						if err := json.Unmarshal(msg.Data, &task); err != nil || task.ID != poisonID {
							t.Fatalf("JetStream dead task = %+v, err = %v", task, err)
						}
						found = true
						break
					}
					time.Sleep(100 * time.Millisecond)
				}
				if !found {
					t.Fatalf("JetStream poison task %s never reached dead subject: %v", poisonID, ctx.Err())
				}
			}
			if tc.name == "nsq" {
				r.Handle("poison", func(context.Context, *Task) error { return errors.New("permanent failure") })
				if _, err := r.Enqueue(ctx, "poison", nil, WithMaxRetry(0)); err != nil {
					t.Fatal(err)
				}
				b := r.backend.(*nsqBackend)
				found := false
				for ctx.Err() == nil {
					req, err := http.NewRequestWithContext(ctx, http.MethodGet, os.Getenv("QUEUE_TEST_NSQ_HTTP")+"/stats?format=json", nil)
					if err != nil {
						t.Fatal(err)
					}
					resp, err := b.http.Do(req)
					if err == nil {
						var stats nsqStats
						err = json.NewDecoder(resp.Body).Decode(&stats)
						resp.Body.Close()
						if err == nil {
							for _, topic := range stats.Topics {
								if topic.TopicName != b.deadTopic("default") {
									continue
								}
								for _, channel := range topic.Channels {
									if channel.ChannelName == b.deadChannel() && channel.Depth > 0 {
										found = true
									}
								}
							}
						}
					}
					if found {
						break
					}
					time.Sleep(100 * time.Millisecond)
				}
				if !found {
					t.Fatalf("NSQ poison task never reached dead topic: %v", ctx.Err())
				}
			}
			if tc.name == "kafka" {
				r.Handle("poison", func(context.Context, *Task) error { return errors.New("permanent failure") })
				poisonID, err := r.Enqueue(ctx, "poison", nil, WithMaxRetry(0))
				if err != nil {
					t.Fatal(err)
				}
				b := r.backend.(*kafkaBackend)
				consumer, err := kgo.NewClient(kgo.SeedBrokers(address), kgo.ConsumeTopics(b.deadTopic("default")))
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for ctx.Err() == nil && !found {
					fetches := consumer.PollRecords(ctx, 1)
					if errs := fetches.Errors(); len(errs) > 0 {
						t.Fatalf("Kafka dead fetch: %v", errs[0].Err)
					}
					fetches.EachRecord(func(record *kgo.Record) {
						var task notification
						if err := json.Unmarshal(record.Value, &task); err != nil || task.ID != poisonID {
							t.Errorf("Kafka dead task = %+v, err = %v", task, err)
						} else {
							found = true
						}
					})
				}
				consumer.Close()
				if !found {
					t.Fatalf("Kafka poison task %s never reached dead topic: %v", poisonID, ctx.Err())
				}
			}
			if tc.name == "nats" {
				for ctx.Err() == nil {
					_, err = r.GetRawMessage(ctx, "default", 1)
					if errors.Is(err, ErrMessageNotRetained) {
						break
					}
					time.Sleep(100 * time.Millisecond)
				}
				if !errors.Is(err, ErrMessageNotRetained) {
					t.Fatalf("acked work-queue message still retained: %v", err)
				}
			}
			overview, err := r.Overview(ctx)
			if err != nil || len(overview) != 1 {
				t.Fatalf("overview=%+v, err=%v", overview, err)
			}
			if tc.name == "nsq" && (overview[0].Ready == nil || overview[0].Active == nil || overview[0].Consumers == nil) {
				t.Fatalf("NSQ channel metrics missing: %+v", overview[0])
			}
			if (tc.name == "rabbitmq" || tc.name == "amqp10") && (overview[0].Connected == nil || !*overview[0].Connected) {
				t.Fatalf("broker connection state missing: %+v", overview[0])
			}
			if tc.name == "kafka" {
				for (overview[0].Lag == nil || overview[0].ConsumerOffset == nil || overview[0].EndOffset == nil) && ctx.Err() == nil {
					time.Sleep(300 * time.Millisecond)
					overview, err = r.Overview(ctx)
					if err != nil {
						t.Fatal(err)
					}
				}
				if overview[0].Lag == nil || overview[0].Partitions == nil || overview[0].ConsumerOffset == nil || overview[0].EndOffset == nil || len(overview[0].PartitionOffsets) == 0 {
					t.Fatalf("Kafka offset/lag metrics missing: %+v", overview[0])
				}
			}
			if tc.name == "nsq" {
				if err := r.PauseQueue(ctx, "default", true); err != nil {
					t.Fatal(err)
				}
				paused, err := r.Overview(ctx)
				if err != nil || paused[0].Paused == nil || !*paused[0].Paused {
					t.Fatalf("NSQ pause status: %+v, %v", paused, err)
				}
				if err := r.PauseQueue(ctx, "default", false); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// A second runtime must observe and control tasks through the broker alone.
func TestNativeTaskManagement(t *testing.T) {
	for _, tc := range []struct {
		name, env string
		configure func(*RuntimeConfig, string)
	}{
		{"rabbitmq", "QUEUE_TEST_RABBIT_URL", func(c *RuntimeConfig, address string) {
			c.RabbitMQURL = address
			c.RabbitMQManagementURL = os.Getenv("QUEUE_TEST_RABBIT_MANAGEMENT_URL")
		}},
		{"nats", "QUEUE_TEST_NATS_URL", func(c *RuntimeConfig, address string) { c.NATSURL = address }},
		{"kafka", "QUEUE_TEST_KAFKA_BROKER", func(c *RuntimeConfig, address string) { c.KafkaBrokers = []string{address} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			address := os.Getenv(tc.env)
			if address == "" {
				t.Skipf("%s 未设置", tc.env)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 70*time.Second)
			defer cancel()
			cfg := RuntimeConfig{Driver: tc.name, Namespace: fmt.Sprintf("qm%d", time.Now().UnixNano()), Queues: map[string]int{"default": 1}, Concurrency: 1, MaxPayloadBytes: 1024}
			tc.configure(&cfg, address)
			worker, err := OpenRuntime(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer worker.Shutdown(context.Background())
			admin, err := OpenRuntime(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer admin.Shutdown(context.Background())
			caps := admin.Capabilities()
			if !caps.TaskList || !caps.TaskGet || !caps.TaskStats || !caps.TaskRetry || !caps.TaskDelete || !caps.TaskCancel {
				t.Fatalf("management capabilities: %+v", caps)
			}
			deletedID, err := worker.Enqueue(ctx, "probe", []byte(`{"kind":"deleted"}`), WithMaxRetry(0))
			if err != nil {
				t.Fatal(err)
			}
			info, err := admin.GetTask("default", deletedID)
			if err != nil || info.State != StatePending || string(info.Payload) != `{"kind":"deleted"}` {
				t.Fatalf("cross-process get: %+v, %v", info, err)
			}
			items, total, err := admin.ListTasks(StatePending, "default", 1, 20)
			if err != nil || total != 1 || len(items) != 1 || items[0].ID != deletedID {
				t.Fatalf("cross-process list: items=%+v total=%d err=%v", items, total, err)
			}
			if err := admin.Delete("default", deletedID); err != nil {
				t.Fatal(err)
			}
			if _, err := worker.GetTask("default", deletedID); !errors.Is(err, ErrTaskNotFound) {
				t.Fatalf("deleted task remains visible: %v", err)
			}
			started := make(chan struct{}, 1)
			stopped := make(chan struct{}, 1)
			worker.Handle("probe", func(ctx context.Context, task *Task) error {
				if string(task.Payload) == `{"kind":"deleted"}` {
					t.Error("deleted task executed")
					return nil
				}
				started <- struct{}{}
				<-ctx.Done()
				stopped <- struct{}{}
				return ctx.Err()
			})
			activeID, err := worker.Enqueue(ctx, "probe", []byte(`{"kind":"cancel"}`), WithMaxRetry(0))
			if err != nil {
				t.Fatal(err)
			}
			if err := worker.Start(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal("task did not start")
			}
			if err := admin.Delete("default", activeID); !errors.Is(err, ErrTaskStateConflict) {
				t.Fatalf("active task delete = %v, want state conflict", err)
			}
			if err := admin.Cancel(activeID); err != nil {
				t.Fatalf("cross-process cancel: %v", err)
			}
			select {
			case <-stopped:
			case <-ctx.Done():
				t.Fatal("handler did not observe cancellation")
			}
			for ctx.Err() == nil {
				info, err = admin.GetTask("default", activeID)
				if err == nil && info.State == StateArchived {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if err != nil || info.State != StateArchived {
				t.Fatalf("cancel result: %+v, %v", info, err)
			}
			completed := make(chan struct{}, 1)
			var completedCount atomic.Int32
			worker.Handle("probe", func(context.Context, *Task) error {
				completedCount.Add(1)
				select {
				case completed <- struct{}{}:
				default:
				}
				return nil
			})
			if err := admin.Retry("default", activeID); err != nil {
				t.Fatalf("cross-process retry: %v", err)
			}
			select {
			case <-completed:
			case <-ctx.Done():
				t.Fatal("retry did not execute")
			}
			for ctx.Err() == nil {
				info, err = admin.GetTask("default", activeID)
				if err == nil && info.State == StateCompleted {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if err != nil || info.State != StateCompleted {
				t.Fatalf("retry result: %+v, %v", info, err)
			}
			duplicate, err := nativeTaskBody(info)
			if err != nil {
				t.Fatal(err)
			}
			if err := worker.backend.Publish(ctx, "default", duplicate); err != nil {
				t.Fatal(err)
			}
			time.Sleep(1200 * time.Millisecond)
			if n := completedCount.Load(); n != 1 {
				t.Fatalf("duplicate broker delivery executed %d times", n)
			}
			stats, err := admin.AllQueueStats(ctx)
			if err != nil || len(stats) != 1 || stats[0].Completed != 1 || stats[0].Size != 1 || stats[0].Processed != 2 || stats[0].Failed != 1 {
				t.Fatalf("cross-process stats: %+v, %v", stats, err)
			}
			if err := admin.Delete("default", activeID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNativeTaskRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, env string
		configure func(*RuntimeConfig, string)
	}{
		{"rabbitmq", "QUEUE_TEST_RABBIT_URL", func(c *RuntimeConfig, address string) {
			c.RabbitMQURL = address
			c.RabbitMQManagementURL = os.Getenv("QUEUE_TEST_RABBIT_MANAGEMENT_URL")
		}},
		{"nats", "QUEUE_TEST_NATS_URL", func(c *RuntimeConfig, address string) { c.NATSURL = address }},
		{"kafka", "QUEUE_TEST_KAFKA_BROKER", func(c *RuntimeConfig, address string) { c.KafkaBrokers = []string{address} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			address := os.Getenv(tc.env)
			if address == "" {
				t.Skipf("%s 未设置", tc.env)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
			defer cancel()
			cfg := RuntimeConfig{Driver: tc.name, Namespace: fmt.Sprintf("qr%d", time.Now().UnixNano()), Queues: map[string]int{"default": 1}, Concurrency: 1, MaxPayloadBytes: 1024}
			tc.configure(&cfg, address)
			worker, err := OpenRuntime(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer worker.Shutdown(context.Background())
			admin, err := OpenRuntime(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer admin.Shutdown(context.Background())
			id := fmt.Sprintf("orphan-%d", time.Now().UnixNano())
			event := nativeTaskEvent{
				EventID: id + "-create", Kind: "create", Queue: "default", ID: id,
				Task: notification{ID: id, Name: "recovered", Payload: []byte(`{"ok":true}`)},
				At:   time.Now().Add(-time.Minute),
			}
			data, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			if err := admin.backend.(nativeTaskJournal).AppendTaskEvent(ctx, data); err != nil {
				t.Fatal(err)
			}
			done := make(chan struct{}, 1)
			worker.Handle("recovered", func(context.Context, *Task) error { done <- struct{}{}; return nil })
			if err := worker.Start(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("orphan task was not republished from broker state")
			}
			for ctx.Err() == nil {
				info, err := admin.GetTask("default", id)
				if err == nil && info.State == StateCompleted {
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
			t.Fatal("recovered task did not complete")
		})
	}
}

func TestNativeTaskScheduling(t *testing.T) {
	for _, tc := range []struct {
		name, env string
		configure func(*RuntimeConfig, string)
	}{
		{"rabbitmq", "QUEUE_TEST_RABBIT_URL", func(c *RuntimeConfig, address string) {
			c.RabbitMQURL = address
			c.RabbitMQManagementURL = os.Getenv("QUEUE_TEST_RABBIT_MANAGEMENT_URL")
		}},
		{"nats", "QUEUE_TEST_NATS_URL", func(c *RuntimeConfig, address string) { c.NATSURL = address }},
		{"kafka", "QUEUE_TEST_KAFKA_BROKER", func(c *RuntimeConfig, address string) { c.KafkaBrokers = []string{address} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			address := os.Getenv(tc.env)
			if address == "" {
				t.Skipf("%s 未设置", tc.env)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			cfg := RuntimeConfig{Driver: tc.name, Namespace: fmt.Sprintf("qs%d", time.Now().UnixNano()), Queues: map[string]int{"default": 1}, Concurrency: 1, MaxPayloadBytes: 1024}
			tc.configure(&cfg, address)
			worker, err := OpenRuntime(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer worker.Shutdown(context.Background())
			admin, err := OpenRuntime(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer admin.Shutdown(context.Background())
			if !admin.Capabilities().Delay {
				t.Fatal("delay capability missing")
			}
			var executions atomic.Int32
			done := make(chan struct{}, 1)
			worker.Handle("scheduled", func(_ context.Context, task *Task) error {
				executions.Add(1)
				if string(task.Payload) == "deleted" {
					t.Error("deleted scheduled task executed")
				}
				select {
				case done <- struct{}{}:
				default:
				}
				return nil
			})
			id, err := worker.Enqueue(ctx, "scheduled", []byte("run"), WithDelay(3*time.Second))
			if err != nil {
				t.Fatal(err)
			}
			deletedID, err := worker.Enqueue(ctx, "scheduled", []byte("deleted"), WithDelay(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			info, err := admin.GetTask("default", id)
			if err != nil || info.State != StateScheduled || info.NextProcessAt.IsZero() {
				t.Fatalf("scheduled state: %+v, %v", info, err)
			}
			if err := admin.Delete("default", deletedID); err != nil {
				t.Fatal(err)
			}
			if err := worker.Start(); err != nil {
				t.Fatal(err)
			}
			beforeDue := time.Until(info.NextProcessAt.Add(-500 * time.Millisecond))
			if beforeDue > 0 {
				select {
				case <-done:
					t.Fatal("scheduled task executed before due time")
				case <-time.After(beforeDue):
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("scheduled task was not executed")
			}
			time.Sleep(time.Second)
			if n := executions.Load(); n != 1 {
				t.Fatalf("scheduled executions = %d, want 1", n)
			}
			if _, err := admin.GetTask("default", deletedID); !errors.Is(err, ErrTaskNotFound) {
				t.Fatalf("deleted scheduled task remains: %v", err)
			}
		})
	}
}

func TestNativeLegacyMessageImport(t *testing.T) {
	for _, tc := range []struct {
		name, env string
		configure func(*RuntimeConfig, string)
	}{
		{"rabbitmq", "QUEUE_TEST_RABBIT_URL", func(c *RuntimeConfig, address string) {
			c.RabbitMQURL = address
			c.RabbitMQManagementURL = os.Getenv("QUEUE_TEST_RABBIT_MANAGEMENT_URL")
		}},
		{"nats", "QUEUE_TEST_NATS_URL", func(c *RuntimeConfig, address string) { c.NATSURL = address }},
		{"kafka", "QUEUE_TEST_KAFKA_BROKER", func(c *RuntimeConfig, address string) { c.KafkaBrokers = []string{address} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			address := os.Getenv(tc.env)
			if address == "" {
				t.Skipf("%s 未设置", tc.env)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			cfg := RuntimeConfig{Driver: tc.name, Namespace: fmt.Sprintf("ql%d", time.Now().UnixNano()), Queues: map[string]int{"default": 1}, Concurrency: 1, MaxPayloadBytes: 1024}
			tc.configure(&cfg, address)
			worker, err := OpenRuntime(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer worker.Shutdown(context.Background())
			admin, err := OpenRuntime(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer admin.Shutdown(context.Background())
			id := fmt.Sprintf("legacy-%d", time.Now().UnixNano())
			body, err := json.Marshal(notification{ID: id, Name: "legacy", Payload: []byte(`{"old":true}`)})
			if err != nil {
				t.Fatal(err)
			}
			var executions atomic.Int32
			done := make(chan struct{}, 1)
			worker.Handle("legacy", func(context.Context, *Task) error {
				executions.Add(1)
				select {
				case done <- struct{}{}:
				default:
				}
				return nil
			})
			if err := worker.backend.Publish(ctx, "default", body); err != nil {
				t.Fatal(err)
			}
			if err := worker.Start(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("legacy message was not imported")
			}
			for ctx.Err() == nil {
				info, err := admin.GetTask("default", id)
				if err == nil && info.State == StateCompleted {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if err := admin.Delete("default", id); err != nil {
				t.Fatal(err)
			}
			if err := worker.backend.Publish(ctx, "default", body); err != nil {
				t.Fatal(err)
			}
			time.Sleep(1200 * time.Millisecond)
			if n := executions.Load(); n != 1 {
				t.Fatalf("deleted legacy task resurrected: executions=%d", n)
			}
			if _, err := admin.GetTask("default", id); !errors.Is(err, ErrTaskNotFound) {
				t.Fatalf("deleted legacy task visible: %v", err)
			}
		})
	}
}

func TestNativeDistributedWorkers(t *testing.T) {
	for _, tc := range []struct {
		name, env string
		configure func(*RuntimeConfig, string)
	}{
		{"rabbitmq", "QUEUE_TEST_RABBIT_URL", func(c *RuntimeConfig, address string) {
			c.RabbitMQURL = address
			c.RabbitMQManagementURL = os.Getenv("QUEUE_TEST_RABBIT_MANAGEMENT_URL")
		}},
		{"nats", "QUEUE_TEST_NATS_URL", func(c *RuntimeConfig, address string) { c.NATSURL = address }},
		{"kafka", "QUEUE_TEST_KAFKA_BROKER", func(c *RuntimeConfig, address string) { c.KafkaBrokers = []string{address}; c.KafkaPartitions = 2 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			address := os.Getenv(tc.env)
			if address == "" {
				t.Skipf("%s 未设置", tc.env)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
			defer cancel()
			cfg := RuntimeConfig{Driver: tc.name, Namespace: fmt.Sprintf("qw%d", time.Now().UnixNano()), Queues: map[string]int{"default": 1}, Concurrency: 2, MaxPayloadBytes: 1024}
			tc.configure(&cfg, address)
			first, err := OpenRuntime(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer first.Shutdown(context.Background())
			second, err := OpenRuntime(ctx, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer second.Shutdown(context.Background())
			done := make(chan string, 10)
			handle := func(ctx context.Context, _ *Task) error {
				time.Sleep(100 * time.Millisecond)
				done <- GetTaskID(ctx)
				return nil
			}
			first.Handle("shared", handle)
			second.Handle("shared", handle)
			if err := first.Start(); err != nil {
				t.Fatal(err)
			}
			if err := second.Start(); err != nil {
				t.Fatal(err)
			}
			want := make(map[string]bool)
			for i := 0; i < 5; i++ {
				id, err := first.Enqueue(ctx, "shared", []byte(fmt.Sprintf(`{"i":%d}`, i)))
				if err != nil {
					t.Fatal(err)
				}
				want[id] = true
			}
			for i := 0; i < 5; i++ {
				select {
				case id := <-done:
					if !want[id] {
						t.Fatalf("unexpected or repeated task ID %q", id)
					}
					delete(want, id)
				case <-ctx.Done():
					t.Fatalf("distributed workers did not finish: remaining=%d", len(want))
				}
			}
			time.Sleep(time.Second)
			select {
			case id := <-done:
				t.Fatalf("duplicate task execution: %s", id)
			default:
			}
		})
	}
}
