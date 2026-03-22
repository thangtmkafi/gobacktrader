package livefeeds

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/thangtmkafi/gobacktrader/core"
)

// ─── liveFeedBase mechanics ──────────────────────────────────────────────────

func TestLiveFeedBaseNext(t *testing.T) {
	base := newLiveFeedBase("test", 16)
	ctx, cancel := context.WithCancel(context.Background())
	base.ctx = ctx
	base.cancel = cancel

	// Push 5 bars then close channel
	go func() {
		for i := 0; i < 5; i++ {
			base.barCh <- core.Bar{
				DateTime: time.Date(2024, 1, 1+i, 0, 0, 0, 0, time.UTC),
				Close:    float64(100 + i),
			}
		}
		close(base.barCh)
	}()

	count := 0
	for base.Next() {
		count++
	}
	if count != 5 {
		t.Errorf("expected 5 bars, got %d", count)
	}
	if base.Data().Len() != 5 {
		t.Errorf("DataSeries should have 5 bars, got %d", base.Data().Len())
	}
	cancel()
}

func TestLiveFeedBaseContextCancel(t *testing.T) {
	base := newLiveFeedBase("test", 16)
	ctx, cancel := context.WithCancel(context.Background())
	base.ctx = ctx
	base.cancel = cancel

	// Cancel immediately — Next should return false
	cancel()
	got := base.Next()
	if got {
		t.Error("Next() should return false when context is cancelled")
	}
}

// ─── DefaultParseBar ────────────────────────────────────────────────────────

func TestDefaultParseBarRFC3339(t *testing.T) {
	input := `{"t":"2024-01-15T10:30:00Z","o":150.5,"h":155.0,"l":149.0,"c":153.5,"v":50000}`
	bar, err := DefaultParseBar([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if bar.Close != 153.5 {
		t.Errorf("Close=%f, want 153.5", bar.Close)
	}
	if bar.DateTime.Year() != 2024 || bar.DateTime.Month() != 1 || bar.DateTime.Day() != 15 {
		t.Errorf("DateTime=%v", bar.DateTime)
	}
}

func TestDefaultParseBarUnixTimestamp(t *testing.T) {
	input := `{"t":1705312200,"o":150.0,"h":155.0,"l":149.0,"c":153.0,"v":100000}`
	bar, err := DefaultParseBar([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if bar.Open != 150.0 {
		t.Errorf("Open=%f, want 150.0", bar.Open)
	}
}

// ─── WebSocket feed ─────────────────────────────────────────────────────────

func TestWebSocketFeed(t *testing.T) {
	upgrader := websocket.Upgrader{}
	bars := []core.Bar{
		{DateTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Open: 100, High: 105, Low: 99, Close: 103, Volume: 1000},
		{DateTime: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Open: 103, High: 108, Low: 102, Close: 107, Volume: 2000},
		{DateTime: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), Open: 107, High: 110, Low: 106, Close: 109, Volume: 3000},
	}

	// Start test WebSocket server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for _, bar := range bars {
			jb := JSONBar{T: bar.DateTime.Format(time.RFC3339), O: bar.Open, H: bar.High, L: bar.Low, C: bar.Close, V: bar.Volume}
			data, _ := json.Marshal(jb)
			conn.WriteMessage(websocket.TextMessage, data)
			time.Sleep(10 * time.Millisecond)
		}
		// Close cleanly
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	feed := NewWebSocketFeed(WebSocketConfig{
		URL:    wsURL,
		Symbol: "TEST",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := feed.Start(ctx); err != nil {
		t.Fatal(err)
	}

	count := 0
	for feed.Next() {
		count++
	}
	if count != 3 {
		t.Errorf("expected 3 bars, got %d", count)
	}
	if feed.Data().Close.Get(0) != 109 {
		t.Errorf("last close=%f, want 109", feed.Data().Close.Get(0))
	}
}

// ─── REST feed ──────────────────────────────────────────────────────────────

func TestRESTFeed(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		// Return a single bar each poll
		jb := JSONBar{
			T: time.Date(2024, 1, callCount, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
			O: float64(100 + callCount), H: float64(105 + callCount),
			L: float64(99 + callCount), C: float64(103 + callCount), V: 1000,
		}
		data, _ := json.Marshal(jb)
		w.Write([]byte("[" + string(data) + "]"))
	}))
	defer srv.Close()

	feed := NewRESTFeed(RESTConfig{
		URL:          srv.URL,
		Symbol:       "TEST",
		PollInterval: 50 * time.Millisecond,
		ParseFunc: func(body []byte) ([]core.Bar, error) {
			var jbs []JSONBar
			if err := json.Unmarshal(body, &jbs); err != nil {
				return nil, err
			}
			var bars []core.Bar
			for _, jb := range jbs {
				b, err := DefaultParseBar(func() []byte {
					d, _ := json.Marshal(jb)
					return d
				}())
				if err != nil {
					continue
				}
				bars = append(bars, b)
			}
			return bars, nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	if err := feed.Start(ctx); err != nil {
		t.Fatal(err)
	}

	count := 0
	for feed.Next() {
		count++
		if count >= 3 {
			cancel() // stop after 3 bars
		}
	}

	if count < 2 {
		t.Errorf("expected at least 2 bars from REST polling, got %d", count)
	}
}
