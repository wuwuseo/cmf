package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
)

type kafkaBackend struct {
	cfg      RuntimeConfig
	producer *kgo.Client
}

func (*kafkaBackend) Capabilities() Capabilities { return Capabilities{MaxRetry: true, Delay: true} }

func openKafka(ctx context.Context, cfg RuntimeConfig) (nativeBackend, error) {
	if len(cfg.KafkaBrokers) == 0 {
		return nil, errors.New("queue: kafka.brokers 未配置")
	}
	client, err := kgo.NewClient(kgo.SeedBrokers(cfg.KafkaBrokers...), kgo.RequiredAcks(kgo.AllISRAcks()))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx); err != nil {
		client.Close()
		return nil, err
	}
	partitions := cfg.KafkaPartitions
	if partitions <= 0 {
		partitions = 1
	}
	replicas := cfg.KafkaReplicationFactor
	if replicas <= 0 {
		replicas = 1
	}
	admin := kadm.NewClient(client)
	stateTopic := cfg.Namespace + ".task-state"
	stateCreated, err := admin.CreateTopics(ctx, 1, replicas, map[string]*string{
		"cleanup.policy": kadm.StringPtr("delete"), "retention.ms": kadm.StringPtr("-1"),
		"retention.bytes": kadm.StringPtr("-1"),
	}, stateTopic)
	if err != nil {
		client.Close()
		return nil, err
	}
	if err := stateCreated[stateTopic].Err; err != nil && !errors.Is(err, kerr.TopicAlreadyExists) {
		client.Close()
		return nil, fmt.Errorf("queue: Kafka topic %s: %w", stateTopic, err)
	}
	if errors.Is(stateCreated[stateTopic].Err, kerr.TopicAlreadyExists) {
		changes, err := admin.AlterTopicConfigs(ctx, []kadm.AlterConfig{
			{Op: kadm.SetConfig, Name: "cleanup.policy", Value: kadm.StringPtr("delete")},
			{Op: kadm.SetConfig, Name: "retention.ms", Value: kadm.StringPtr("-1")},
			{Op: kadm.SetConfig, Name: "retention.bytes", Value: kadm.StringPtr("-1")},
		}, stateTopic)
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("queue: Kafka 任务状态 topic 必须永久保留事件: %w", err)
		}
		if len(changes) != 1 || changes[0].Err != nil {
			client.Close()
			return nil, fmt.Errorf("queue: Kafka 任务状态 topic 必须永久保留事件: %+v", changes)
		}
	}
	for queue := range cfg.Queues {
		for _, topic := range []string{cfg.Namespace + "." + queue, cfg.Namespace + "." + queue + ".dead"} {
			created, err := admin.CreateTopics(ctx, partitions, replicas, nil, topic)
			if err != nil {
				client.Close()
				return nil, err
			}
			if err := created[topic].Err; err != nil && !errors.Is(err, kerr.TopicAlreadyExists) {
				client.Close()
				return nil, fmt.Errorf("queue: Kafka topic %s: %w", topic, err)
			}
		}
	}
	return &kafkaBackend{cfg: cfg, producer: client}, nil
}
func (b *kafkaBackend) topic(queue string) string     { return b.cfg.Namespace + "." + queue }
func (b *kafkaBackend) deadTopic(queue string) string { return b.topic(queue) + ".dead" }
func (b *kafkaBackend) stateTopic() string            { return b.cfg.Namespace + ".task-state" }
func (b *kafkaBackend) group(queue string) string {
	base := b.cfg.KafkaGroup
	if base == "" {
		base = b.cfg.Namespace + "_workers"
	}
	return base + "_" + queue
}
func (b *kafkaBackend) Publish(ctx context.Context, queue string, body []byte) error {
	return b.producer.ProduceSync(ctx, &kgo.Record{Topic: b.topic(queue), Value: body}).FirstErr()
}
func (b *kafkaBackend) Consume(ctx context.Context, queue string, process func(context.Context, []byte) error) error {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(b.cfg.KafkaBrokers...), kgo.ConsumerGroup(b.group(queue)),
		kgo.ConsumeTopics(b.topic(queue)), kgo.DisableAutoCommit(), kgo.BlockRebalanceOnPoll(),
	)
	if err != nil {
		return err
	}
	defer client.Close()
	for ctx.Err() == nil {
		fetches := client.PollRecords(ctx, 1)
		if fetches.IsClientClosed() {
			return errors.New("queue: Kafka 消费端关闭")
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			client.AllowRebalance()
			return fmt.Errorf("queue: Kafka 拉取失败: %v", errs[0].Err)
		}
		iter := fetches.RecordIter()
		for !iter.Done() {
			record := iter.Next()
			// The replacement is confirmed before the original offset is committed.
			// A crash between those steps can duplicate the task (at-least-once delivery).
			for ctx.Err() == nil {
				attempt := kafkaAttempt(record.Headers)
				err := process(context.WithValue(ctx, runtimeAttemptKey{}, attempt), record.Value)
				if errors.Is(err, errNativeTaskBusy) {
					select {
					case <-ctx.Done():
					case <-time.After(time.Second):
					}
					continue
				}
				if ctx.Err() != nil {
					break
				}
				if err != nil {
					task, _ := nativeNotification(record.Value, b.cfg.MaxRetry)
					destination := b.deadTopic(queue)
					headers := []kgo.RecordHeader{{Key: "x-cmf-attempt", Value: []byte(strconv.Itoa(attempt))}}
					if attempt < nativeRetryLimit(task.MaxRetry) {
						destination = b.topic(queue)
						headers[0].Value = []byte(strconv.Itoa(attempt + 1))
					}
					for ctx.Err() == nil {
						out := &kgo.Record{Topic: destination, Key: record.Key, Value: record.Value, Headers: headers}
						if pubErr := b.producer.ProduceSync(ctx, out).FirstErr(); pubErr == nil {
							break
						}
						select {
						case <-ctx.Done():
						case <-time.After(time.Second):
						}
					}
					if ctx.Err() != nil {
						break
					}
				}
				for ctx.Err() == nil {
					if err := client.CommitRecords(ctx, record); err == nil {
						break
					}
					select {
					case <-ctx.Done():
						break
					case <-time.After(time.Second):
					}
				}
				break
			}
		}
		client.AllowRebalance()
	}
	return ctx.Err()
}
func kafkaAttempt(headers []kgo.RecordHeader) int {
	for _, h := range headers {
		if h.Key != "x-cmf-attempt" {
			continue
		}
		n, err := strconv.Atoi(string(h.Value))
		if err == nil && n >= 0 {
			return n
		}
	}
	return 0
}
func (b *kafkaBackend) Overview(ctx context.Context, queues []string) ([]QueueOverview, error) {
	admin := kadm.NewClient(b.producer)
	out := make([]QueueOverview, 0, len(queues))
	for _, queue := range queues {
		item := QueueOverview{Queue: queue, Source: "kafka"}
		lags, err := admin.Lag(ctx, b.group(queue))
		if err != nil {
			item.Error = err.Error()
			out = append(out, item)
			continue
		}
		group, ok := lags[b.group(queue)]
		if !ok {
			item.Error = "consumer group 未建立"
			out = append(out, item)
			continue
		}
		if err := group.Error(); err != nil {
			item.Error = err.Error()
			out = append(out, item)
			continue
		}
		var lag, partitions, committed, end int64
		valid := true
		validCommit, validEnd := true, true
		for partition, p := range group.Lag[b.topic(queue)] {
			partitions++
			part := PartitionOffset{Partition: partition}
			if p.Err != nil || p.Lag < 0 {
				valid = false
			} else {
				lag += p.Lag
				value := p.Lag
				part.Lag = &value
			}
			if p.Commit.At < 0 {
				validCommit = false
			} else {
				committed += p.Commit.At
				value := p.Commit.At
				part.Committed = &value
			}
			if p.End.Err != nil || p.End.Offset < 0 {
				validEnd = false
			} else {
				end += p.End.Offset
				value := p.End.Offset
				part.End = &value
			}
			item.PartitionOffsets = append(item.PartitionOffsets, part)
		}
		sort.Slice(item.PartitionOffsets, func(i, j int) bool { return item.PartitionOffsets[i].Partition < item.PartitionOffsets[j].Partition })
		item.Partitions = &partitions
		if valid && partitions > 0 {
			item.Lag = &lag
		}
		if validCommit && partitions > 0 {
			item.ConsumerOffset = &committed
		}
		if validEnd && partitions > 0 {
			item.EndOffset = &end
		}
		item.Healthy = partitions > 0 && valid && validEnd
		out = append(out, item)
	}
	return out, nil
}
func (b *kafkaBackend) Close() error { b.producer.Close(); return nil }
func (b *kafkaBackend) AppendTaskEvent(ctx context.Context, data []byte) error {
	return b.producer.ProduceSync(ctx, &kgo.Record{Topic: b.stateTopic(), Value: data}).FirstErr()
}
func (b *kafkaBackend) TailTaskOffset(ctx context.Context) (uint64, error) {
	offsets, err := kadm.NewClient(b.producer).ListEndOffsets(ctx, b.stateTopic())
	if err != nil {
		return 0, err
	}
	if err := offsets.Error(); err != nil {
		return 0, err
	}
	partition, ok := offsets[b.stateTopic()][0]
	if !ok || partition.Offset < 0 {
		return 0, errors.New("queue: Kafka 任务状态 topic 缺少分区 0")
	}
	return uint64(partition.Offset), nil
}
func (b *kafkaBackend) ReplayTaskEvents(ctx context.Context, from, stop uint64, marker string, visit func([]byte) error) (uint64, error) {
	if from == 0 {
		starts, err := kadm.NewClient(b.producer).ListStartOffsets(ctx, b.stateTopic())
		if err != nil {
			return 0, err
		}
		if err := starts.Error(); err != nil {
			return 0, err
		}
		if start, ok := starts[b.stateTopic()][0]; !ok || start.Offset != 0 {
			return 0, errors.New("queue: Kafka 任务状态日志起始 offset 不为 0")
		}
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(b.cfg.KafkaBrokers...),
		kgo.ConsumePartitions(map[string]map[int32]kgo.Offset{b.stateTopic(): {0: kgo.NewOffset().At(int64(from))}}),
	)
	if err != nil {
		return 0, err
	}
	defer client.Close()
	for ctx.Err() == nil {
		fetches := client.PollRecords(ctx, 100)
		if errs := fetches.Errors(); len(errs) > 0 {
			return 0, fmt.Errorf("queue: Kafka 任务状态日志读取失败: %w", errs[0].Err)
		}
		iter := fetches.RecordIter()
		for !iter.Done() {
			record := iter.Next()
			if marker == "" && uint64(record.Offset) >= stop {
				return stop, nil
			}
			if err := visit(record.Value); err != nil {
				return 0, err
			}
			var event nativeTaskEvent
			if err := json.Unmarshal(record.Value, &event); err != nil {
				return 0, err
			}
			if event.EventID == marker {
				return uint64(record.Offset) + 1, nil
			}
			if marker == "" && uint64(record.Offset)+1 >= stop {
				return stop, nil
			}
		}
	}
	return 0, ctx.Err()
}
