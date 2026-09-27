package queue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"
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
