package livefeeds

import (
	"context"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	"github.com/thangtmkafi/gobacktrader/core"
)

// WebSocketConfig configures a WebSocket live feed.
type WebSocketConfig struct {
	// URL is the WebSocket endpoint, e.g. "ws://localhost:8080/bars".
	URL string
	// Symbol is used as the DataSeries name.
	Symbol string
	// ReconnectDelay is the wait before retrying after disconnect (0 = no retry).
	ReconnectDelay time.Duration
	// ParseFunc converts a raw message into a Bar.
	// If nil, DefaultParseBar (JSON) is used.
	ParseFunc func(msg []byte) (core.Bar, error)
}

// WebSocketFeed streams bars from a WebSocket server.
type WebSocketFeed struct {
	*liveFeedBase
	cfg  WebSocketConfig
	conn *websocket.Conn
}

// NewWebSocketFeed creates a WebSocket live feed.
func NewWebSocketFeed(cfg WebSocketConfig) *WebSocketFeed {
	name := cfg.Symbol
	if name == "" {
		name = "ws"
	}
	return &WebSocketFeed{
		liveFeedBase: newLiveFeedBase(name, 256),
		cfg:          cfg,
	}
}

// Start connects to the WebSocket and begins reading messages in a goroutine.
func (f *WebSocketFeed) Start(ctx context.Context) error {
	f.ctx, f.cancel = context.WithCancel(ctx)

	conn, _, err := websocket.DefaultDialer.DialContext(f.ctx, f.cfg.URL, nil)
	if err != nil {
		return fmt.Errorf("websocket: dial %q: %w", f.cfg.URL, err)
	}
	f.conn = conn

	parse := f.cfg.ParseFunc
	if parse == nil {
		parse = DefaultParseBar
	}

	go f.readLoop(parse)
	return nil
}

func (f *WebSocketFeed) readLoop(parse func([]byte) (core.Bar, error)) {
	defer close(f.barCh)
	defer f.conn.Close()

	for {
		select {
		case <-f.ctx.Done():
			return
		default:
		}

		_, msg, err := f.conn.ReadMessage()
		if err != nil {
			if f.ctx.Err() != nil {
				return // context cancelled
			}
			if f.cfg.ReconnectDelay > 0 {
				time.Sleep(f.cfg.ReconnectDelay)
				conn, _, dialErr := websocket.DefaultDialer.DialContext(f.ctx, f.cfg.URL, nil)
				if dialErr != nil {
					continue
				}
				f.conn.Close()
				f.conn = conn
				continue
			}
			return
		}

		bar, err := parse(msg)
		if err != nil {
			continue // skip unparseable messages
		}

		select {
		case f.barCh <- bar:
		case <-f.ctx.Done():
			return
		}
	}
}

// Stop gracefully closes the WebSocket connection.
func (f *WebSocketFeed) Stop() error {
	if f.cancel != nil {
		f.cancel()
	}
	if f.conn != nil {
		return f.conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
	}
	return nil
}
