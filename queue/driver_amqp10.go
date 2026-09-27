package queue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/Azure/go-amqp"
)

type amqp10Backend struct {
	cfg       RuntimeConfig
	mu        sync.Mutex
	consuming map[string]bool
}

func (*amqp10Backend) Capabilities() Capabilities { return Capabilities{} }

func openAMQP10(ctx context.Context, cfg RuntimeConfig) (nativeBackend, error) {
	if cfg.AMQP10URL == "" {
		return nil, errors.New("queue: amqp10.url 未配置")
	}
	conn, err := amqp.Dial(ctx, cfg.AMQP10URL, nil)
	if err != nil {
		return nil, err
	}
	_ = conn.Close()
	return &amqp10Backend{cfg: cfg, consuming: make(map[string]bool)}, nil
}
func (b *amqp10Backend) address(queue string) string { return b.cfg.Namespace + "." + queue }
func (b *amqp10Backend) Publish(ctx context.Context, queue string, body []byte) error {
	conn, err := amqp.Dial(ctx, b.cfg.AMQP10URL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	session, err := conn.NewSession(ctx, nil)
	if err != nil {
		return err
	}
	defer session.Close(context.Background())
	sender, err := session.NewSender(ctx, b.address(queue), nil)
	if err != nil {
		return err
	}
	defer sender.Close(context.Background())
	msg := amqp.NewMessage(body)
	msg.Header = &amqp.MessageHeader{Durable: true}
	receipt, err := sender.SendWithReceipt(ctx, msg, nil)
	if err != nil {
		return err
	}
	state, err := receipt.Wait(ctx)
	if err != nil {
		return err
	}
	if _, ok := state.(*amqp.StateAccepted); !ok {
		return fmt.Errorf("queue: AMQP 1.0 发布未获 accepted 确认: %T", state)
	}
	return nil
}
func (b *amqp10Backend) Consume(ctx context.Context, queue string, process func(context.Context, []byte) error) error {
	conn, err := amqp.Dial(ctx, b.cfg.AMQP10URL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	session, err := conn.NewSession(ctx, nil)
	if err != nil {
		return err
	}
	defer session.Close(context.Background())
	receiver, err := session.NewReceiver(ctx, b.address(queue), &amqp.ReceiverOptions{Credit: int32(b.cfg.Concurrency)})
	if err != nil {
		return err
	}
	defer receiver.Close(context.Background())
	b.mu.Lock()
	b.consuming[queue] = true
	b.mu.Unlock()
	defer func() { b.mu.Lock(); delete(b.consuming, queue); b.mu.Unlock() }()
	for ctx.Err() == nil {
		msg, err := receiver.Receive(ctx, nil)
		if err != nil {
			return err
		}
		if err := process(ctx, msg.GetData()); err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
			if err := receiver.ReleaseMessage(ctx, msg); err != nil {
				return err
			}
		} else if err := receiver.AcceptMessage(ctx, msg); err != nil {
			return err
		}
	}
	return ctx.Err()
}
func (b *amqp10Backend) Overview(ctx context.Context, queues []string) ([]QueueOverview, error) {
	conn, err := amqp.Dial(ctx, b.cfg.AMQP10URL, nil)
	if err != nil {
		items := unavailableOverviews(queues, "amqp10:connection", err)
		connected := false
		for i := range items {
			items[i].Connected = &connected
		}
		return items, nil
	}
	defer conn.Close()
	out := make([]QueueOverview, 0, len(queues))
	for _, queue := range queues {
		connected := true
		b.mu.Lock()
		active := b.consuming[queue]
		b.mu.Unlock()
		consumers := int64(0)
		if active {
			consumers = 1
		}
		out = append(out, QueueOverview{Queue: queue, Source: "amqp10:connection", Connected: &connected, Consumers: &consumers, Healthy: active})
	}
	return out, nil
}
func (b *amqp10Backend) Close() error { return nil }
