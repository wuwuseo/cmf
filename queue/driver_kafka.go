package queue

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
)

type kafkaBackend struct {
	cfg      RuntimeConfig
	producer *kgo.Client
}

func (*kafkaBackend) Capabilities() Capabilities { return Capabilities{} }

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
	for queue := range cfg.Queues {
		topic := cfg.Namespace + "." + queue
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
	return &kafkaBackend{cfg: cfg, producer: client}, nil
}
func (b *kafkaBackend) topic(queue string) string { return b.cfg.Namespace + "." + queue }
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
			// A poison record blocks only its partition. Never commit past a failed task.
			for ctx.Err() == nil {
				if err := process(ctx, record.Value); err != nil {
					select {
					case <-ctx.Done():
						break
					case <-time.After(time.Second):
					}
					continue
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
