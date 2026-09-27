package queue

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type natsBackend struct {
	cfg    RuntimeConfig
	conn   *nats.Conn
	js     jetstream.JetStream
	stream jetstream.Stream
}

func (*natsBackend) Capabilities() Capabilities { return Capabilities{RawMessageGet: true} }

func openNATS(ctx context.Context, cfg RuntimeConfig) (nativeBackend, error) {
	if cfg.NATSURL == "" {
		return nil, errors.New("queue: nats.url 未配置")
	}
	conn, err := nats.Connect(cfg.NATSURL, nats.MaxReconnects(-1), nats.ReconnectWait(time.Second))
	if err != nil {
		return nil, err
	}
	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	b := &natsBackend{cfg: cfg, conn: conn, js: js}
	stream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name: b.streamName(), Subjects: []string{cfg.Namespace + ".queue.*"},
		Storage: jetstream.FileStorage, Retention: jetstream.WorkQueuePolicy,
		Discard: jetstream.DiscardNew,
	})
	if err != nil {
		conn.Close()
		return nil, err
	}
	b.stream = stream
	for queue := range cfg.Queues {
		if _, err := b.consumer(ctx, queue); err != nil {
			conn.Close()
			return nil, err
		}
	}
	return b, nil
}
func (b *natsBackend) streamName() string               { return strings.ToUpper(b.cfg.Namespace) + "_TASKS" }
func (b *natsBackend) subject(queue string) string      { return b.cfg.Namespace + ".queue." + queue }
func (b *natsBackend) consumerName(queue string) string { return b.cfg.Namespace + "_" + queue }
func (b *natsBackend) consumer(ctx context.Context, queue string) (jetstream.Consumer, error) {
	return b.js.CreateOrUpdateConsumer(ctx, b.streamName(), jetstream.ConsumerConfig{
		Durable: b.consumerName(queue), AckPolicy: jetstream.AckExplicitPolicy,
		AckWait: 60 * time.Second, MaxAckPending: b.cfg.Concurrency,
		FilterSubject: b.subject(queue),
	})
}
func (b *natsBackend) Publish(ctx context.Context, queue string, body []byte) error {
	_, err := b.js.Publish(ctx, b.subject(queue), body)
	return err
}
func (b *natsBackend) Consume(ctx context.Context, queue string, process func(context.Context, []byte) error) error {
	c, err := b.consumer(ctx, queue)
	if err != nil {
		return err
	}
	sem := make(chan struct{}, b.cfg.Concurrency)
	var wg sync.WaitGroup
	defer wg.Wait()
	for ctx.Err() == nil {
		batch, err := c.Fetch(b.cfg.Concurrency, jetstream.FetchMaxWait(time.Second))
		if err != nil {
			return err
		}
		for msg := range batch.Messages() {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return ctx.Err()
			}
			wg.Add(1)
			go func(m jetstream.Msg) {
				defer wg.Done()
				defer func() { <-sem }()
				b.processMessage(ctx, m, process)
			}(msg)
		}
		if err := batch.Error(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
	}
	return ctx.Err()
}

func (b *natsBackend) processMessage(ctx context.Context, msg jetstream.Msg, process func(context.Context, []byte) error) {
	stop := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = msg.InProgress()
			}
		}
	}()
	err := process(ctx, msg.Data())
	close(stop)
	<-stopped
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		_ = msg.NakWithDelay(time.Second)
	} else {
		_ = msg.Ack()
	}
}
func (b *natsBackend) Overview(ctx context.Context, queues []string) ([]QueueOverview, error) {
	out := make([]QueueOverview, 0, len(queues))
	for _, queue := range queues {
		c, err := b.js.Consumer(ctx, b.streamName(), b.consumerName(queue))
		if err != nil {
			out = append(out, QueueOverview{Queue: queue, Source: "jetstream", Error: err.Error()})
			continue
		}
		info, err := c.Info(ctx)
		if err != nil {
			out = append(out, QueueOverview{Queue: queue, Source: "jetstream", Error: err.Error()})
			continue
		}
		ready, active := int64(info.NumPending), int64(info.NumAckPending)
		out = append(out, QueueOverview{Queue: queue, Source: "jetstream", Ready: &ready, Active: &active, Healthy: true})
	}
	return out, nil
}
func (b *natsBackend) GetRaw(ctx context.Context, queue string, sequence uint64) (RawMessage, error) {
	msg, err := b.stream.GetMsg(ctx, sequence)
	if err != nil {
		if errors.Is(err, jetstream.ErrMsgNotFound) {
			return RawMessage{}, ErrMessageNotRetained
		}
		return RawMessage{}, err
	}
	if msg.Subject != b.subject(queue) {
		return RawMessage{}, ErrMessageNotRetained
	}
	return RawMessage{Queue: queue, Sequence: msg.Sequence, Payload: msg.Data}, nil
}
func (b *natsBackend) Close() error {
	if b.conn == nil {
		return nil
	}
	b.conn.Close()
	return nil
}

var _ nativeBackend = (*natsBackend)(nil)
