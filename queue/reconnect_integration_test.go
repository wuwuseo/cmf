package queue

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

// Set QUEUE_TEST_RECONNECT_DRIVER and QUEUE_TEST_RESTART_CONTAINER to run this
// against a disposable Docker broker. The test restarts that exact container.
func TestNativeBrokerReconnect(t *testing.T) {
	driver := os.Getenv("QUEUE_TEST_RECONNECT_DRIVER")
	container := os.Getenv("QUEUE_TEST_RESTART_CONTAINER")
	if driver == "" || container == "" {
		t.Skip("未配置可重启的临时 broker")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 120*time.Second)
	defer cancel()
	cfg := RuntimeConfig{Driver: driver, Namespace: fmt.Sprintf("qr%d", time.Now().UnixNano()), Queues: map[string]int{"default": 1}, Concurrency: 2, MaxPayloadBytes: 1024}
	switch driver {
	case "rabbitmq":
		cfg.RabbitMQURL, cfg.RabbitMQManagementURL = os.Getenv("QUEUE_TEST_RABBIT_URL"), os.Getenv("QUEUE_TEST_RABBIT_MANAGEMENT_URL")
	case "nats":
		cfg.NATSURL = os.Getenv("QUEUE_TEST_NATS_URL")
	case "nsq":
		cfg.NSQD, cfg.NSQHTTP = os.Getenv("QUEUE_TEST_NSQD"), os.Getenv("QUEUE_TEST_NSQ_HTTP")
	case "kafka":
		cfg.KafkaBrokers = []string{os.Getenv("QUEUE_TEST_KAFKA_BROKER")}
	case "amqp10":
		cfg.AMQP10URL = os.Getenv("QUEUE_TEST_AMQP10_URL")
		if address := os.Getenv("QUEUE_TEST_AMQP10_DECLARE_RABBIT"); address != "" {
			conn, err := amqp091.Dial(address)
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
	default:
		t.Fatalf("unknown driver %q", driver)
	}
	r, err := OpenRuntime(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Shutdown(context.Background())
	done := make(chan string, 8)
	r.Handle("reconnect", func(_ context.Context, task *Task) error { done <- string(task.Payload); return nil })
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	publishAndWait := func(stage string) {
		var published bool
		var lastErr error
		for ctx.Err() == nil {
			attemptCtx, stop := context.WithTimeout(ctx, 4*time.Second)
			_, lastErr = r.Enqueue(attemptCtx, "reconnect", []byte(stage))
			stop()
			if lastErr == nil {
				published = true
				break
			}
			time.Sleep(time.Second)
		}
		if !published {
			t.Fatalf("%s publish: %v (%v)", stage, lastErr, ctx.Err())
		}
		for {
			select {
			case got := <-done:
				if got == stage {
					return
				}
			case <-ctx.Done():
				t.Fatalf("%s consume: %v", stage, ctx.Err())
			}
		}
	}
	publishAndWait("before restart")
	output, err := exec.CommandContext(ctx, "docker", "restart", container).CombinedOutput()
	if err != nil {
		t.Fatalf("restart %s: %v: %s", container, err, output)
	}
	publishAndWait("after restart")
}
