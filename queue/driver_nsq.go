package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nsqio/go-nsq"
)

type nsqBackend struct {
	cfg      RuntimeConfig
	producer *nsq.Producer
	http     *http.Client
}

func (b *nsqBackend) Capabilities() Capabilities {
	return Capabilities{QueuePause: b.cfg.NSQHTTP != ""}
}

func openNSQ(_ context.Context, cfg RuntimeConfig) (nativeBackend, error) {
	if cfg.NSQD == "" {
		return nil, errors.New("queue: nsq.nsqd 未配置")
	}
	if cfg.NSQHTTP == "" {
		return nil, errors.New("queue: nsq.http 未配置，无法读取管理统计")
	}
	p, err := nsq.NewProducer(cfg.NSQD, nsq.NewConfig())
	if err != nil {
		return nil, err
	}
	if err = p.Ping(); err != nil {
		p.Stop()
		return nil, err
	}
	return &nsqBackend{cfg: cfg, producer: p, http: &http.Client{Timeout: 5 * time.Second}}, nil
}
func (b *nsqBackend) topic(queue string) string { return b.cfg.Namespace + "_" + queue }
func (b *nsqBackend) channel() string           { return b.cfg.Namespace + "_workers" }
func (b *nsqBackend) Publish(ctx context.Context, queue string, body []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return b.producer.Publish(b.topic(queue), body)
}
func (b *nsqBackend) Consume(ctx context.Context, queue string, process func(context.Context, []byte) error) error {
	cfg := nsq.NewConfig()
	cfg.MaxInFlight = b.cfg.Concurrency
	cfg.MaxAttempts = 0 // never silently finish a poison message
	cfg.DefaultRequeueDelay = 2 * time.Second
	cfg.LookupdPollInterval = 5 * time.Second // direct nsqd reconnect uses this interval too
	c, err := nsq.NewConsumer(b.topic(queue), b.channel(), cfg)
	if err != nil {
		return err
	}
	c.AddConcurrentHandlers(nsq.HandlerFunc(func(m *nsq.Message) error {
		return process(ctx, m.Body)
	}), b.cfg.Concurrency)
	if b.cfg.NSQLookupd != "" {
		err = c.ConnectToNSQLookupd(b.cfg.NSQLookupd)
	} else {
		err = c.ConnectToNSQD(b.cfg.NSQD)
	}
	if err != nil {
		c.Stop()
		return err
	}
	select {
	case <-ctx.Done():
		c.Stop()
		<-c.StopChan
		return ctx.Err()
	case <-c.StopChan:
		return errors.New("queue: NSQ 消费已停止")
	}
}

type nsqStats struct {
	Topics []struct {
		TopicName string `json:"topic_name"`
		Channels  []struct {
			ChannelName string            `json:"channel_name"`
			Depth       int64             `json:"depth"`
			InFlight    int64             `json:"in_flight_count"`
			Paused      bool              `json:"paused"`
			Clients     []json.RawMessage `json:"clients"`
		} `json:"channels"`
	} `json:"topics"`
}

func (b *nsqBackend) Overview(ctx context.Context, queues []string) ([]QueueOverview, error) {
	endpoint := strings.TrimRight(b.cfg.NSQHTTP, "/") + "/stats?format=json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.http.Do(req)
	if err != nil {
		return unavailableOverviews(queues, "nsq:stats", err), nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return unavailableOverviews(queues, "nsq:stats", fmt.Errorf("queue: NSQ stats HTTP %d", resp.StatusCode)), nil
	}
	var stats nsqStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return unavailableOverviews(queues, "nsq:stats", err), nil
	}
	out := make([]QueueOverview, 0, len(queues))
	for _, queue := range queues {
		item := QueueOverview{Queue: queue, Source: "nsq:stats", Error: "topic/channel 尚未建立"}
		for _, topic := range stats.Topics {
			if topic.TopicName != b.topic(queue) {
				continue
			}
			for _, ch := range topic.Channels {
				if ch.ChannelName != b.channel() {
					continue
				}
				ready, active, consumers := ch.Depth, ch.InFlight, int64(len(ch.Clients))
				item.Ready, item.Active, item.Consumers = &ready, &active, &consumers
				paused := ch.Paused
				item.Paused = &paused
				item.Healthy = true
				item.Error = ""
			}
		}
		out = append(out, item)
	}
	return out, nil
}
func (b *nsqBackend) Pause(ctx context.Context, queue string, pause bool) error {
	action := "pause"
	if !pause {
		action = "unpause"
	}
	u := strings.TrimRight(b.cfg.NSQHTTP, "/") + "/channel/" + action + "?topic=" + url.QueryEscape(b.topic(queue)) + "&channel=" + url.QueryEscape(b.channel())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return err
	}
	resp, err := b.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("queue: NSQ %s HTTP %d", action, resp.StatusCode)
	}
	return nil
}
func (b *nsqBackend) Close() error { b.producer.Stop(); return nil }
