package livefeeds

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/thangtmkafi/gobacktrader/core"
)

// NATSConfig configures a NATS subscriber live feed.
type NATSConfig struct {
	// URL is the NATS server address, e.g. "nats://localhost:4222".
	URL string
	// Subject is the NATS subject to subscribe to, e.g. "market.bars.AAPL".
	Subject string
	// Symbol is used as the DataSeries name (defaults to Subject if empty).
	Symbol string
	// ParseFunc converts a NATS message payload into a Bar.
	// If nil, DefaultParseBar (JSON) is used.
	ParseFunc func(data []byte) (core.Bar, error)
}

// NATSFeed subscribes to a NATS subject for real-time bar messages.
type NATSFeed struct {
	*liveFeedBase
	cfg  NATSConfig
	conn *nats.Conn
	sub  *nats.Subscription
}

// NewNATSFeed creates a NATS live feed.
func NewNATSFeed(cfg NATSConfig) *NATSFeed {
	name := cfg.Symbol
	if name == "" {
		name = cfg.Subject
	}
	return &NATSFeed{
		liveFeedBase: newLiveFeedBase(name, 256),
		cfg:          cfg,
	}
}

// Start connects to NATS and subscribes to the configured subject.
func (f *NATSFeed) Start(ctx context.Context) error {
	f.ctx, f.cancel = context.WithCancel(ctx)

	conn, err := nats.Connect(f.cfg.URL)
	if err != nil {
		return fmt.Errorf("nats: connect %q: %w", f.cfg.URL, err)
	}
	f.conn = conn

	parse := f.cfg.ParseFunc
	if parse == nil {
		parse = DefaultParseBar
	}

	sub, err := conn.Subscribe(f.cfg.Subject, func(msg *nats.Msg) {
		bar, err := parse(msg.Data)
		if err != nil {
			return
		}
		select {
		case f.barCh <- bar:
		case <-f.ctx.Done():
		}
	})
	if err != nil {
		conn.Close()
		return fmt.Errorf("nats: subscribe %q: %w", f.cfg.Subject, err)
	}
	f.sub = sub

	// Close channel when context is cancelled
	go func() {
		<-f.ctx.Done()
		f.sub.Unsubscribe()
		f.conn.Close()
		close(f.barCh)
	}()

	return nil
}

// Stop unsubscribes and closes the NATS connection.
func (f *NATSFeed) Stop() error {
	if f.cancel != nil {
		f.cancel()
	}
	return nil
}
