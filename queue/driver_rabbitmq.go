package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type rabbitBackend struct {
	cfg  RuntimeConfig
	conn *amqp.Connection
	mu   sync.Mutex
	http *http.Client
}

func (*rabbitBackend) Capabilities() Capabilities { return Capabilities{MaxRetry: true, Delay: true} }

func openRabbitMQ(ctx context.Context, cfg RuntimeConfig) (nativeBackend, error) {
	if cfg.RabbitMQURL == "" || cfg.RabbitMQManagementURL == "" {
		return nil, errors.New("queue: rabbitmq.url 和 rabbitmq.management_url 必须配置")
	}
	mgmt, err := url.Parse(cfg.RabbitMQManagementURL)
	if err != nil || (mgmt.Scheme != "http" && mgmt.Scheme != "https") {
		return nil, errors.New("queue: rabbitmq.management_url 必须是 HTTP(S) URL")
	}
	conn, err := amqp.Dial(cfg.RabbitMQURL)
	if err != nil {
		return nil, err
	}
	b := &rabbitBackend{cfg: cfg, conn: conn, http: &http.Client{Timeout: 5 * time.Second}}
	stateCh, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}
	_, err = stateCh.QueueDeclare(b.stateName(), true, false, false, false, amqp.Table{"x-queue-type": "stream"})
	stateCh.Close()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("queue: RabbitMQ 任务状态 stream 不可用: %w", err)
	}
	bootstrap, _ := json.Marshal(nativeTaskEvent{EventID: "bootstrap", Kind: "bootstrap", At: time.Now().UTC()})
	if err := b.publish(ctx, b.stateName(), bootstrap, nil); err != nil {
		conn.Close()
		return nil, err
	}
	for queue := range cfg.Queues {
		ch, e := conn.Channel()
		if e != nil {
			conn.Close()
			return nil, e
		}
		_, e = ch.QueueDeclare(b.name(queue), true, false, false, false, nil)
		if e == nil {
			_, e = ch.QueueDeclare(b.deadName(queue), true, false, false, false, nil)
		}
		ch.Close()
		if e != nil {
			conn.Close()
			return nil, e
		}
	}
	return b, nil
}

func (b *rabbitBackend) name(queue string) string     { return b.cfg.Namespace + "." + queue }
func (b *rabbitBackend) deadName(queue string) string { return b.name(queue) + ".dead" }
func (b *rabbitBackend) stateName() string            { return b.cfg.Namespace + ".task-state" }
func (b *rabbitBackend) connection() (*amqp.Connection, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.conn != nil && !b.conn.IsClosed() {
		return b.conn, nil
	}
	conn, err := amqp.Dial(b.cfg.RabbitMQURL)
	if err != nil {
		return nil, err
	}
	b.conn = conn
	return conn, nil
}
func (b *rabbitBackend) Publish(ctx context.Context, queue string, body []byte) error {
	return b.publish(ctx, b.name(queue), body, nil)
}
func (b *rabbitBackend) publish(ctx context.Context, routingKey string, body []byte, headers amqp.Table) error {
	conn, err := b.connection()
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	if err = ch.Confirm(false); err != nil {
		return err
	}
	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	returns := ch.NotifyReturn(make(chan amqp.Return, 1))
	err = ch.PublishWithContext(ctx, "", routingKey, true, false, amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: body, Headers: headers,
	})
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c, ok := <-confirms:
		if !ok || !c.Ack {
			return errors.New("queue: RabbitMQ 发布未获确认")
		}
		select {
		case ret := <-returns:
			return fmt.Errorf("queue: RabbitMQ 消息不可路由: %s", ret.ReplyText)
		default:
			return nil
		}
	}
}
func (b *rabbitBackend) Consume(ctx context.Context, queue string, process func(context.Context, []byte) error) error {
	conn, err := b.connection()
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	if _, err = ch.QueueDeclare(b.name(queue), true, false, false, false, nil); err != nil {
		return err
	}
	if err = ch.Qos(b.cfg.Concurrency, 0, false); err != nil {
		return err
	}
	deliveries, err := ch.Consume(b.name(queue), "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	sem := make(chan struct{}, b.cfg.Concurrency)
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("queue: RabbitMQ 消费连接中断")
			}
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return ctx.Err()
			}
			wg.Add(1)
			go func(d amqp.Delivery) {
				defer wg.Done()
				defer func() { <-sem }()
				attempt := rabbitAttempt(d.Headers)
				err := process(context.WithValue(ctx, runtimeAttemptKey{}, attempt), d.Body)
				if errors.Is(err, errNativeTaskBusy) {
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Second):
					}
					_ = d.Nack(false, true)
					return
				}
				if err == nil {
					_ = d.Ack(false)
					return
				}
				if ctx.Err() != nil {
					return // closing the channel returns the unacked delivery to the queue
				}
				task, _ := nativeNotification(d.Body, b.cfg.MaxRetry)
				routingKey := b.retryDestination(queue, attempt, nativeRetryLimit(task.MaxRetry))
				lastError := err.Error()
				if len(lastError) > 1024 {
					lastError = lastError[:1024]
				}
				headers := amqp.Table{"x-cmf-attempt": int32(attempt), "x-cmf-last-error": lastError, "x-cmf-failed-at": time.Now().UTC().Format(time.RFC3339Nano)}
				if routingKey == b.name(queue) {
					headers = amqp.Table{"x-cmf-attempt": int32(attempt + 1)}
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Second):
					}
				}
				// Confirm the replacement before acknowledging the original. A crash
				// between these steps can duplicate work, as with other at-least-once queues.
				if pubErr := b.publish(ctx, routingKey, d.Body, headers); pubErr != nil {
					_ = d.Nack(false, true)
					return
				}
				_ = d.Ack(false)
			}(d)
		}
	}
}
func (b *rabbitBackend) retryDestination(queue string, attempt, maxRetry int) string {
	if attempt < maxRetry {
		return b.name(queue)
	}
	return b.deadName(queue)
}
func rabbitAttempt(headers amqp.Table) int {
	switch n := headers["x-cmf-attempt"].(type) {
	case int32:
		if n >= 0 {
			return int(n)
		}
	case int64:
		if n >= 0 && n <= 1<<31-1 {
			return int(n)
		}
	}
	return 0
}
func (b *rabbitBackend) Overview(ctx context.Context, queues []string) ([]QueueOverview, error) {
	conn, err := b.connection()
	if err != nil {
		items := unavailableOverviews(queues, "rabbitmq:management", err)
		connected := false
		for i := range items {
			items[i].Connected = &connected
		}
		return items, nil
	}
	management, _ := url.Parse(b.cfg.RabbitMQManagementURL)
	amqpURL, _ := url.Parse(b.cfg.RabbitMQURL)
	vhost := strings.TrimPrefix(amqpURL.Path, "/")
	if vhost == "" {
		vhost = "/"
	}
	credentials := management.User
	if credentials == nil {
		credentials = amqpURL.User
	}
	out := make([]QueueOverview, 0, len(queues))
	for _, queue := range queues {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		connected := !conn.IsClosed()
		item := QueueOverview{Queue: queue, Source: "rabbitmq:management", Connected: &connected}
		endpoint := strings.TrimRight(b.cfg.RabbitMQManagementURL, "/") + "/api/queues/" + url.PathEscape(vhost) + "/" + url.PathEscape(b.name(queue))
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if e != nil {
			item.Error = e.Error()
			out = append(out, item)
			continue
		}
		if credentials != nil {
			password, _ := credentials.Password()
			req.SetBasicAuth(credentials.Username(), password)
		}
		resp, e := b.http.Do(req)
		if e != nil {
			item.Error = e.Error()
			out = append(out, item)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			item.Error = fmt.Sprintf("management HTTP %d", resp.StatusCode)
			resp.Body.Close()
			out = append(out, item)
			continue
		}
		var q struct {
			Ready     int64  `json:"messages_ready"`
			Unacked   int64  `json:"messages_unacknowledged"`
			Consumers int64  `json:"consumers"`
			State     string `json:"state"`
		}
		e = json.NewDecoder(resp.Body).Decode(&q)
		resp.Body.Close()
		if e != nil {
			item.Error = e.Error()
			out = append(out, item)
			continue
		}
		item.Ready, item.Active, item.Consumers = &q.Ready, &q.Unacked, &q.Consumers
		item.Healthy = !conn.IsClosed() && (q.State == "running" || q.State == "")
		if !item.Healthy {
			item.Error = "队列或 AMQP 连接异常"
		}
		out = append(out, item)
	}
	return out, nil
}
func (b *rabbitBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.conn != nil {
		return b.conn.Close()
	}
	return nil
}
func (b *rabbitBackend) AppendTaskEvent(ctx context.Context, data []byte) error {
	return b.publish(ctx, b.stateName(), data, nil)
}
func rabbitStreamOffset(d amqp.Delivery) (uint64, error) {
	switch n := d.Headers["x-stream-offset"].(type) {
	case int64:
		if n >= 0 {
			return uint64(n), nil
		}
	case int32:
		if n >= 0 {
			return uint64(n), nil
		}
	}
	return 0, fmt.Errorf("queue: RabbitMQ stream 缺少 offset: %T", d.Headers["x-stream-offset"])
}
func (b *rabbitBackend) TailTaskOffset(ctx context.Context) (uint64, error) {
	conn, err := b.connection()
	if err != nil {
		return 0, err
	}
	ch, err := conn.Channel()
	if err != nil {
		return 0, err
	}
	defer ch.Close()
	if err := ch.Qos(1, 0, false); err != nil {
		return 0, err
	}
	deliveries, err := ch.Consume(b.stateName(), "", false, false, false, false, amqp.Table{"x-stream-offset": "last"})
	if err != nil {
		return 0, err
	}
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case d, ok := <-deliveries:
		if !ok {
			return 0, errors.New("queue: RabbitMQ stream 末尾读取中断")
		}
		offset, err := rabbitStreamOffset(d)
		if err != nil {
			return 0, err
		}
		if err := d.Ack(false); err != nil {
			return 0, err
		}
		return offset + 1, nil
	}
}
func (b *rabbitBackend) ReplayTaskEvents(ctx context.Context, from, stop uint64, marker string, visit func([]byte) error) (uint64, error) {
	conn, err := b.connection()
	if err != nil {
		return 0, err
	}
	ch, err := conn.Channel()
	if err != nil {
		return 0, err
	}
	defer ch.Close()
	if err := ch.Qos(100, 0, false); err != nil {
		return 0, err
	}
	offset := any("first")
	if from > 0 {
		offset = int64(from)
	}
	deliveries, err := ch.Consume(b.stateName(), "", false, false, false, false, amqp.Table{"x-stream-offset": offset})
	if err != nil {
		return 0, err
	}
	expected := from
	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				return 0, errors.New("queue: RabbitMQ 任务状态 stream 消费中断")
			}
			offset, err := rabbitStreamOffset(d)
			if err != nil {
				return 0, err
			}
			if offset != expected {
				return 0, errors.New("queue: RabbitMQ 任务状态 stream 存在缺口")
			}
			if marker == "" && offset >= stop {
				return stop, nil
			}
			if err := visit(d.Body); err != nil {
				return 0, err
			}
			var event nativeTaskEvent
			if err := json.Unmarshal(d.Body, &event); err != nil {
				return 0, err
			}
			next := offset + 1
			expected = next
			if err := d.Ack(false); err != nil {
				return 0, err
			}
			if event.EventID == marker {
				return next, nil
			}
			if marker == "" && next >= stop {
				return stop, nil
			}
		}
	}
}
