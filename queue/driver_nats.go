package queue

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type natsBackend struct {
	cfg         RuntimeConfig
	conn        *nats.Conn
	js          jetstream.JetStream
	stream      jetstream.Stream
	stateStream jetstream.Stream
}

func (*natsBackend) Capabilities() Capabilities {
	return Capabilities{RawMessageGet: true, MaxRetry: true, Delay: true}
}

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
		Name: b.streamName(), Subjects: []string{cfg.Namespace + ".queue.*", cfg.Namespace + ".dead.*"},
		Storage: jetstream.FileStorage, Retention: jetstream.WorkQueuePolicy,
		Discard: jetstream.DiscardNew,
	})
	if err != nil {
		conn.Close()
		return nil, err
	}
	b.stream = stream
	stateStream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name: b.stateStreamName(), Subjects: []string{b.stateSubject()},
		Storage: jetstream.FileStorage, Retention: jetstream.LimitsPolicy,
		Discard: jetstream.DiscardNew,
	})
	if err != nil {
		conn.Close()
		return nil, err
	}
	b.stateStream = stateStream
	for queue := range cfg.Queues {
		if _, err := b.consumer(ctx, queue); err != nil {
			conn.Close()
			return nil, err
		}
	}
	return b, nil
}
func (b *natsBackend) streamName() string { return strings.ToUpper(b.cfg.Namespace) + "_TASKS" }
func (b *natsBackend) stateStreamName() string {
	return strings.ToUpper(b.cfg.Namespace) + "_TASK_STATE"
}
func (b *natsBackend) stateSubject() string             { return b.cfg.Namespace + ".task.state" }
func (b *natsBackend) subject(queue string) string      { return b.cfg.Namespace + ".queue." + queue }
func (b *natsBackend) deadSubject(queue string) string  { return b.cfg.Namespace + ".dead." + queue }
func (b *natsBackend) consumerName(queue string) string { return b.cfg.Namespace + "_" + queue }
func (b *natsBackend) consumer(ctx context.Context, queue string) (jetstream.Consumer, error) {
	return b.js.CreateOrUpdateConsumer(ctx, b.streamName(), jetstream.ConsumerConfig{
		Durable: b.consumerName(queue), AckPolicy: jetstream.AckExplicitPolicy,
		AckWait: 60 * time.Second, MaxAckPending: b.cfg.Concurrency,
		FilterSubject: b.subject(queue),
	})
}
func (b *natsBackend) Publish(ctx context.Context, queue string, body []byte) error {
	headers := nats.Header{}
	headers.Set("X-CMF-Attempt", "0")
	_, err := b.js.PublishMsg(ctx, &nats.Msg{Subject: b.subject(queue), Data: body, Header: headers})
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
	meta, metaErr := msg.Metadata()
	if metaErr != nil || meta == nil || meta.NumDelivered == 0 {
		close(stop)
		<-stopped
		_ = msg.NakWithDelay(time.Second)
		return
	}
	attempt := int(meta.NumDelivered - 1)
	if raw := msg.Headers().Get("X-CMF-Attempt"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			attempt = parsed
		}
	}
	err := process(context.WithValue(ctx, runtimeAttemptKey{}, attempt), msg.Data())
	close(stop)
	<-stopped
	if errors.Is(err, errNativeTaskBusy) {
		_ = msg.NakWithDelay(time.Second)
		return
	}
	if ctx.Err() != nil {
		return
	}
	ackCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err != nil {
		task, _ := nativeNotification(msg.Data(), b.cfg.MaxRetry)
		if attempt < nativeRetryLimit(task.MaxRetry) {
			headers := nats.Header{}
			headers.Set("X-CMF-Attempt", strconv.Itoa(attempt+1))
			queue := strings.TrimPrefix(msg.Subject(), b.cfg.Namespace+".queue.")
			_, pubErr := b.js.PublishMsg(ctx, &nats.Msg{Subject: b.subject(queue), Data: msg.Data(), Header: headers})
			if pubErr != nil {
				_ = msg.NakWithDelay(time.Second)
				return
			}
			_ = msg.DoubleAck(ackCtx)
			return
		}
		headers := nats.Header{}
		headers.Set("X-CMF-Attempt", strconv.Itoa(attempt))
		lastError := err.Error()
		if len(lastError) > 1024 {
			lastError = lastError[:1024]
		}
		headers.Set("X-CMF-Last-Error", lastError)
		queue := strings.TrimPrefix(msg.Subject(), b.cfg.Namespace+".queue.")
		_, pubErr := b.js.PublishMsg(ctx, &nats.Msg{Subject: b.deadSubject(queue), Data: msg.Data(), Header: headers})
		if pubErr != nil {
			_ = msg.NakWithDelay(time.Second)
			return
		}
		_ = msg.DoubleAck(ackCtx)
	} else {
		_ = msg.DoubleAck(ackCtx)
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
func (b *natsBackend) AppendTaskEvent(ctx context.Context, data []byte) error {
	_, err := b.js.Publish(ctx, b.stateSubject(), data)
	return err
}
func (b *natsBackend) TailTaskOffset(ctx context.Context) (uint64, error) {
	info, err := b.stateStream.Info(ctx)
	if err != nil {
		return 0, err
	}
	if info.State.Msgs == 0 {
		if info.State.LastSeq != 0 {
			return 0, errors.New("queue: JetStream 任务状态日志已丢失")
		}
		return 0, nil
	}
	return info.State.LastSeq + 1, nil
}
func (b *natsBackend) ReplayTaskEvents(ctx context.Context, from, stop uint64, marker string, visit func([]byte) error) (uint64, error) {
	info, err := b.stateStream.Info(ctx)
	if err != nil {
		return 0, err
	}
	if from == 0 {
		if info.State.FirstSeq != 1 {
			return 0, errors.New("queue: JetStream 任务状态日志起始序号不为 1")
		}
		from = info.State.FirstSeq
	}
	if from < info.State.FirstSeq {
		return 0, errors.New("queue: JetStream 任务状态日志存在缺口")
	}
	end := info.State.LastSeq + 1
	if marker == "" {
		end = stop
	}
	for seq := from; seq < end; seq++ {
		msg, err := b.stateStream.GetMsg(ctx, seq)
		if errors.Is(err, jetstream.ErrMsgNotFound) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if err := visit(msg.Data); err != nil {
			return 0, err
		}
		var event nativeTaskEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return 0, err
		}
		if event.EventID == marker {
			return seq + 1, nil
		}
	}
	if marker == "" {
		return end, nil
	}
	return 0, errors.New("queue: JetStream 任务状态日志缺少确认事件")
}
func (b *natsBackend) Close() error {
	if b.conn == nil {
		return nil
	}
	b.conn.Close()
	return nil
}

var _ nativeBackend = (*natsBackend)(nil)
