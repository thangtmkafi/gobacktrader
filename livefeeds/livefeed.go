// Package livefeeds provides real-time data feed implementations for gobacktrader.
//
// Each feed type runs a background goroutine that pushes core.Bar values onto
// a channel. The shared liveFeedBase.Next() method reads from that channel
// and advances the DataSeries, so the engine/strategy loop works identically
// to backtesting — it just blocks until the next bar arrives.
package livefeeds

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/thangtmkafi/gobacktrader/core"
	"github.com/thangtmkafi/gobacktrader/engine"
)

// LiveFeed extends DataFeed for real-time streaming sources.
type LiveFeed interface {
	engine.DataFeed
	// Start begins streaming bars in a background goroutine.
	Start(ctx context.Context) error
	// Stop gracefully shuts down the feed.
	Stop() error
	// BarChan returns the read-only channel that delivers new bars.
	BarChan() <-chan core.Bar
}

// liveFeedBase is the shared foundation for all live feed implementations.
// Embed this struct and push bars onto barCh from your Start goroutine.
type liveFeedBase struct {
	name   string
	data   *core.DataSeries
	barCh  chan core.Bar
	ctx    context.Context
	cancel context.CancelFunc
}

func newLiveFeedBase(name string, bufSize int) *liveFeedBase {
	if bufSize <= 0 {
		bufSize = 256
	}
	return &liveFeedBase{
		name:  name,
		data:  core.NewDataSeries(name),
		barCh: make(chan core.Bar, bufSize),
	}
}

// Load is a no-op for live feeds (data streams in real-time).
func (f *liveFeedBase) Load() error { return nil }

// Next blocks until a bar arrives on the channel or the context is cancelled.
// Returns false when the channel is closed or the context is done.
func (f *liveFeedBase) Next() bool {
	select {
	case bar, ok := <-f.barCh:
		if !ok {
			return false
		}
		f.data.Forward()
		f.data.AppendBar(bar)
		return true
	case <-f.ctx.Done():
		return false
	}
}

// Data returns the live DataSeries (grows bar-by-bar as Next() is called).
func (f *liveFeedBase) Data() *core.DataSeries { return f.data }

// BarChan returns the bar delivery channel.
func (f *liveFeedBase) BarChan() <-chan core.Bar { return f.barCh }

// ─── Default JSON bar format ──────────────────────────────────────────────────

// JSONBar is the default wire format for bars sent over WebSocket/NATS/Redis/Kafka.
//
//	{"t":"2024-01-02T15:04:05Z","o":150.0,"h":155.0,"l":149.0,"c":153.0,"v":100000}
//
// or with unix timestamp:
//
//	{"t":1704207845,"o":150.0,"h":155.0,"l":149.0,"c":153.0,"v":100000}
type JSONBar struct {
	T interface{} `json:"t"` // time: string (RFC3339) or number (unix seconds)
	O float64     `json:"o"`
	H float64     `json:"h"`
	L float64     `json:"l"`
	C float64     `json:"c"`
	V float64     `json:"v"`
}

// DefaultParseBar is the default JSON → core.Bar parser.
func DefaultParseBar(data []byte) (core.Bar, error) {
	var jb JSONBar
	if err := json.Unmarshal(data, &jb); err != nil {
		return core.Bar{}, fmt.Errorf("parse bar JSON: %w", err)
	}
	var dt time.Time
	switch v := jb.T.(type) {
	case string:
		var err error
		dt, err = time.Parse(time.RFC3339, v)
		if err != nil {
			dt, err = time.Parse("2006-01-02", v)
			if err != nil {
				return core.Bar{}, fmt.Errorf("parse datetime %q: %w", v, err)
			}
		}
	case float64:
		dt = time.Unix(int64(v), 0).UTC()
	default:
		return core.Bar{}, fmt.Errorf("unsupported timestamp type %T", jb.T)
	}
	return core.Bar{
		DateTime: dt,
		Open:     jb.O,
		High:     jb.H,
		Low:      jb.L,
		Close:    jb.C,
		Volume:   jb.V,
	}, nil
}
