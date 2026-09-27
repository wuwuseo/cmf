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

func (*rabbitBackend) Capabilities() Capabilities { return Capabilities{} }

func openRabbitMQ(_ context.Context, cfg RuntimeConfig) (nativeBackend, error) {
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
	for queue := range cfg.Queues {
		ch, e := conn.Channel()
		if e != nil {
			conn.Close()
			return nil, e
		}
		_, e = ch.QueueDeclare(b.name(queue), true, false, false, false, nil)
		ch.Close()
		if e != nil {
			conn.Close()
			return nil, e
		}
	}
	return b, nil
}

func (b *rabbitBackend) name(queue string) string { return b.cfg.Namespace + "." + queue }
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
	err = ch.PublishWithContext(ctx, "", b.name(queue), true, false, amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: body,
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
				if err := process(ctx, d.Body); err != nil {
					select {
					case <-ctx.Done():
					case <-time.After(time.Second):
					}
					_ = d.Nack(false, true)
				} else {
					_ = d.Ack(false)
				}
			}(d)
		}
	}
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
