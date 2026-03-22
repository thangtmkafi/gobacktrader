package livefeeds

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/thangtmkafi/gobacktrader/core"
)

// RedisConfig configures a Redis Streams live feed.
type RedisConfig struct {
	// Addr is the Redis address, e.g. "localhost:6379".
	Addr     string
	Password string
	DB       int
	// Stream is the Redis stream key, e.g. "bars:AAPL".
	Stream string
	// Group is the consumer group name. If empty, uses XREAD instead of XREADGROUP.
	Group    string
	Consumer string
	// Symbol is used as the DataSeries name (defaults to Stream if empty).
	Symbol string
	// ParseFunc converts Redis stream field values into a Bar.
	// If nil, a default parser is used that reads fields: t, o, h, l, c, v.
	ParseFunc func(values map[string]interface{}) (core.Bar, error)
}

// RedisFeed reads bars from a Redis Stream using XREAD BLOCK.
type RedisFeed struct {
	*liveFeedBase
	cfg    RedisConfig
	client *redis.Client
}

// NewRedisFeed creates a Redis Streams live feed.
func NewRedisFeed(cfg RedisConfig) *RedisFeed {
	name := cfg.Symbol
	if name == "" {
		name = cfg.Stream
	}
	return &RedisFeed{
		liveFeedBase: newLiveFeedBase(name, 256),
		cfg:          cfg,
	}
}

// Start connects to Redis and begins reading the stream in a goroutine.
func (f *RedisFeed) Start(ctx context.Context) error {
	f.ctx, f.cancel = context.WithCancel(ctx)

	f.client = redis.NewClient(&redis.Options{
		Addr:     f.cfg.Addr,
		Password: f.cfg.Password,
		DB:       f.cfg.DB,
	})

	// Verify connection
	if err := f.client.Ping(f.ctx).Err(); err != nil {
		return fmt.Errorf("redis: ping: %w", err)
	}

	// Create consumer group if specified and not already existing
	if f.cfg.Group != "" {
		_ = f.client.XGroupCreateMkStream(f.ctx, f.cfg.Stream, f.cfg.Group, "0").Err()
	}

	parse := f.cfg.ParseFunc
	if parse == nil {
		parse = defaultRedisParseBar
	}

	go f.readLoop(parse)
	return nil
}

func (f *RedisFeed) readLoop(parse func(map[string]interface{}) (core.Bar, error)) {
	defer close(f.barCh)
	defer f.client.Close()

	lastID := "$" // only new messages
	if f.cfg.Group != "" {
		lastID = ">"
	}

	for {
		select {
		case <-f.ctx.Done():
			return
		default:
		}

		var streams []redis.XStream
		var err error

		if f.cfg.Group != "" {
			streams, err = f.client.XReadGroup(f.ctx, &redis.XReadGroupArgs{
				Group:    f.cfg.Group,
				Consumer: f.cfg.Consumer,
				Streams:  []string{f.cfg.Stream, lastID},
				Count:    10,
				Block:    1 * time.Second,
			}).Result()
		} else {
			streams, err = f.client.XRead(f.ctx, &redis.XReadArgs{
				Streams: []string{f.cfg.Stream, lastID},
				Count:   10,
				Block:   1 * time.Second,
			}).Result()
		}

		if err != nil {
			if err == redis.Nil {
				continue
			}
			if f.ctx.Err() != nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				bar, err := parse(msg.Values)
				if err != nil {
					continue
				}
				if f.cfg.Group == "" {
					lastID = msg.ID
				}
				select {
				case f.barCh <- bar:
				case <-f.ctx.Done():
					return
				}
			}
		}
	}
}

// Stop closes the Redis connection.
func (f *RedisFeed) Stop() error {
	if f.cancel != nil {
		f.cancel()
	}
	return nil
}

// defaultRedisParseBar reads fields: t (string or unix), o, h, l, c, v from a Redis stream entry.
func defaultRedisParseBar(values map[string]interface{}) (core.Bar, error) {
	parseFloat := func(key string) (float64, error) {
		v, ok := values[key]
		if !ok {
			return 0, nil
		}
		switch val := v.(type) {
		case string:
			return strconv.ParseFloat(val, 64)
		case float64:
			return val, nil
		default:
			return 0, fmt.Errorf("field %q: unexpected type %T", key, v)
		}
	}

	var dt time.Time
	if t, ok := values["t"]; ok {
		switch tv := t.(type) {
		case string:
			var err error
			dt, err = time.Parse(time.RFC3339, tv)
			if err != nil {
				dt, err = time.Parse("2006-01-02", tv)
				if err != nil {
					return core.Bar{}, fmt.Errorf("parse 't': %w", err)
				}
			}
		}
	} else {
		dt = time.Now().UTC()
	}

	o, _ := parseFloat("o")
	h, _ := parseFloat("h")
	l, _ := parseFloat("l")
	c, _ := parseFloat("c")
	v, _ := parseFloat("v")

	return core.Bar{DateTime: dt, Open: o, High: h, Low: l, Close: c, Volume: v}, nil
}
