package livefeeds

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/thangtmkafi/gobacktrader/core"
)

// KafkaConfig configures a Kafka consumer live feed.
type KafkaConfig struct {
	// Brokers is the list of Kafka broker addresses.
	Brokers []string
	// Topic is the Kafka topic to consume from.
	Topic string
	// GroupID is the consumer group ID. If empty, a random one is used.
	GroupID string
	// Symbol is used as the DataSeries name (defaults to Topic if empty).
	Symbol string
	// ParseFunc converts a Kafka message (key + value) into a Bar.
	// If nil, DefaultParseBar is applied to the value.
	ParseFunc func(key, value []byte) (core.Bar, error)
}

// KafkaFeed consumes bars from a Kafka topic.
type KafkaFeed struct {
	*liveFeedBase
	cfg    KafkaConfig
	reader *kafka.Reader
}

// NewKafkaFeed creates a Kafka consumer live feed.
func NewKafkaFeed(cfg KafkaConfig) *KafkaFeed {
	name := cfg.Symbol
	if name == "" {
		name = cfg.Topic
	}
	return &KafkaFeed{
		liveFeedBase: newLiveFeedBase(name, 256),
		cfg:          cfg,
	}
}

// Start creates a Kafka reader and begins consuming in a goroutine.
func (f *KafkaFeed) Start(ctx context.Context) error {
	f.ctx, f.cancel = context.WithCancel(ctx)

	groupID := f.cfg.GroupID
	if groupID == "" {
		groupID = "gobacktrader"
	}

	f.reader = kafka.NewReader(kafka.ReaderConfig{
		Brokers: f.cfg.Brokers,
		Topic:   f.cfg.Topic,
		GroupID: groupID,
	})

	parse := f.cfg.ParseFunc
	if parse == nil {
		parse = func(_, value []byte) (core.Bar, error) {
			return DefaultParseBar(value)
		}
	}

	go f.readLoop(parse)
	return nil
}

func (f *KafkaFeed) readLoop(parse func(key, value []byte) (core.Bar, error)) {
	defer close(f.barCh)
	defer f.reader.Close()

	for {
		msg, err := f.reader.ReadMessage(f.ctx)
		if err != nil {
			if f.ctx.Err() != nil {
				return
			}
			continue
		}

		bar, err := parse(msg.Key, msg.Value)
		if err != nil {
			continue
		}

		select {
		case f.barCh <- bar:
		case <-f.ctx.Done():
			return
		}
	}
}

// Stop closes the Kafka reader.
func (f *KafkaFeed) Stop() error {
	if f.cancel != nil {
		f.cancel()
	}
	if f.reader != nil {
		return f.reader.Close()
	}
	return nil
}

// String returns a summary.
func (f *KafkaFeed) String() string {
	return fmt.Sprintf("KafkaFeed[topic=%s, brokers=%v]", f.cfg.Topic, f.cfg.Brokers)
}
