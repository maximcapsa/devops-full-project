// Package kafka wraps franz-go with small producer/consumer helpers. The
// producer is synchronous and idempotent; the consumer is at-least-once with
// per-record retry and commits offsets only after a record is handled — so
// consumers MUST be idempotent (dedupe on the event's order_id).
package kafka

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/maximcapsa/devops-full-project/pkg/retry"
)

// EnsureTopics creates the given topics if they do not exist. We create topics
// explicitly (rather than relying on broker auto-create) so partition count and
// replication factor are deterministic. Single broker => replication factor 1
// (documented tradeoff). Already-existing topics are ignored.
func EnsureTopics(ctx context.Context, brokers []string, partitions int32, replication int16, topics ...string) error {
	cl, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		return fmt.Errorf("admin client: %w", err)
	}
	defer cl.Close()

	adm := kadm.NewClient(cl)
	return retry.Do(ctx, 8, 500*time.Millisecond, 5*time.Second, func() error {
		resp, err := adm.CreateTopics(ctx, partitions, replication, nil, topics...)
		if err != nil {
			return err
		}
		for _, ct := range resp.Sorted() {
			if ct.Err != nil && !errors.Is(ct.Err, kerr.TopicAlreadyExists) {
				return fmt.Errorf("create topic %s: %w", ct.Topic, ct.Err)
			}
		}
		return nil
	})
}

// Producer publishes events synchronously with retry.
type Producer struct {
	client *kgo.Client
}

// NewProducer creates an idempotent, acks=all producer (franz-go defaults).
func NewProducer(brokers []string) (*Producer, error) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ProducerLinger(5*time.Millisecond),
		kgo.RecordRetries(10),
	)
	if err != nil {
		return nil, fmt.Errorf("new kafka producer: %w", err)
	}
	return &Producer{client: cl}, nil
}

// Publish synchronously produces one record, retrying on transient errors.
func (p *Producer) Publish(ctx context.Context, topic, key string, value []byte) error {
	return retry.Do(ctx, 5, 200*time.Millisecond, 5*time.Second, func() error {
		rec := &kgo.Record{Topic: topic, Key: []byte(key), Value: value}
		return p.client.ProduceSync(ctx, rec).FirstErr()
	})
}

// Ping checks broker connectivity (for readiness probes).
func (p *Producer) Ping(ctx context.Context) error { return p.client.Ping(ctx) }

// Close flushes and closes the producer.
func (p *Producer) Close() { p.client.Close() }

// Handler processes a single record. Returning an error triggers retry; a
// persistent failure stops the consumer (the service restarts and reprocesses
// from the last commit, hence the idempotency requirement).
type Handler func(ctx context.Context, r *kgo.Record) error

// Consumer is a franz-go consumer-group client with manual commits.
type Consumer struct {
	client *kgo.Client
}

// NewConsumer joins the given consumer group and subscribes to topics.
func NewConsumer(brokers []string, group string, topics ...string) (*Consumer, error) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topics...),
		kgo.DisableAutoCommit(),
		kgo.FetchMaxWait(time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("new kafka consumer: %w", err)
	}
	return &Consumer{client: cl}, nil
}

// Run polls and dispatches records to handler until ctx is cancelled. Each
// record is retried with backoff; offsets for a fetched batch are committed
// only after every record in it is handled successfully (at-least-once).
func (c *Consumer) Run(ctx context.Context, handler Handler) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fetches := c.client.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				if errors.Is(e.Err, context.Canceled) {
					return ctx.Err()
				}
			}
			// Transient fetch error: brief pause, then retry the poll.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
			continue
		}

		var handled []*kgo.Record
		iter := fetches.RecordIter()
		for !iter.Done() {
			rec := iter.Next()
			err := retry.Do(ctx, 6, 200*time.Millisecond, 5*time.Second, func() error {
				return handler(ctx, rec)
			})
			if err != nil {
				return fmt.Errorf("handle record topic=%s partition=%d offset=%d: %w",
					rec.Topic, rec.Partition, rec.Offset, err)
			}
			handled = append(handled, rec)
		}
		if len(handled) > 0 {
			if err := c.client.CommitRecords(ctx, handled...); err != nil {
				return fmt.Errorf("commit offsets: %w", err)
			}
		}
	}
}

// Ping checks broker connectivity (for readiness probes).
func (c *Consumer) Ping(ctx context.Context) error { return c.client.Ping(ctx) }

// Close leaves the group and closes the client.
func (c *Consumer) Close() { c.client.Close() }
