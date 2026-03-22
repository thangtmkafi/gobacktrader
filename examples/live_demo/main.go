// Command live_demo demonstrates gobacktrader's live feed support.
// It starts a tiny local WebSocket server that replays testdata/sample.csv
// as if it were a live market feed (1 bar per 100ms), then runs a SMA crossover
// strategy against it using RunLive().
//
// No external infrastructure required — fully self-contained.
//
// Usage:
//
//	go run ./examples/live_demo/
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/thangtmkafi/gobacktrader/analyzers"
	"github.com/thangtmkafi/gobacktrader/core"
	"github.com/thangtmkafi/gobacktrader/engine"
	"github.com/thangtmkafi/gobacktrader/indicators"
	"github.com/thangtmkafi/gobacktrader/livefeeds"
)

// ─── Embedded WebSocket replay server ─────────────────────────────────────────

func replayServer(addr, csvPath string) *http.Server {
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/bars", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "upgrade: %v\n", err)
			return
		}
		defer conn.Close()

		bars, err := loadBarsFromCSV(csvPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load csv: %v\n", err)
			return
		}

		for _, bar := range bars {
			jb := livefeeds.JSONBar{
				T: bar.DateTime.Format(time.RFC3339),
				O: bar.Open, H: bar.High, L: bar.Low, C: bar.Close, V: bar.Volume,
			}
			data, _ := json.Marshal(jb)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
			time.Sleep(50 * time.Millisecond) // simulate real-time delay
		}

		// Signal end by closing cleanly
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"))
	})

	srv := &http.Server{Addr: addr, Handler: mux}
	go srv.ListenAndServe()
	time.Sleep(100 * time.Millisecond) // let server start
	return srv
}

func loadBarsFromCSV(path string) ([]core.Bar, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	var bars []core.Bar
	for i, rec := range records {
		if i == 0 {
			continue // skip header
		}
		dt, _ := time.Parse("2006-01-02", strings.TrimSpace(rec[0]))
		o, _ := strconv.ParseFloat(strings.TrimSpace(rec[1]), 64)
		h, _ := strconv.ParseFloat(strings.TrimSpace(rec[2]), 64)
		l, _ := strconv.ParseFloat(strings.TrimSpace(rec[3]), 64)
		c, _ := strconv.ParseFloat(strings.TrimSpace(rec[4]), 64)
		v, _ := strconv.ParseFloat(strings.TrimSpace(rec[6]), 64)
		bars = append(bars, core.Bar{DateTime: dt, Open: o, High: h, Low: l, Close: c, Volume: v})
	}
	return bars, nil
}

// ─── Strategy ─────────────────────────────────────────────────────────────────

type LiveSMA struct {
	ctx     *engine.StrategyContext
	fast    *indicators.SMA
	slow    *indicators.SMA
	pending *engine.Order
	barNum  int
}

func (s *LiveSMA) Init(ctx *engine.StrategyContext) {
	s.ctx = ctx
	// For live feeds, PreloadedData() may be empty. Build indicators from
	// whatever data is available. In this demo the live feed starts empty,
	// so we build indicators with period=5 and =10 (smaller for demo).
	// Indicators will have NaN until enough bars accumulate.
}

func (s *LiveSMA) Next() {
	s.barNum++
	data := s.ctx.Data()

	// Build running SMA by hand for live mode (no pre-loaded data)
	if data.Len() < 10 {
		return
	}
	fast := avg(data.Close, 5)
	slow := avg(data.Close, 10)

	if s.pending != nil {
		return
	}
	pos := s.ctx.GetPosition(data)
	close := data.Close.Get(0)

	if math.IsNaN(fast) || math.IsNaN(slow) {
		return
	}

	if !pos.IsOpen() && fast > slow {
		fmt.Printf("  [LIVE bar %d] BUY SIGNAL  close=%.2f fast=%.2f slow=%.2f\n",
			s.barNum, close, fast, slow)
		s.pending = s.ctx.Buy(10)
	} else if pos.IsOpen() && fast < slow {
		fmt.Printf("  [LIVE bar %d] SELL SIGNAL close=%.2f fast=%.2f slow=%.2f\n",
			s.barNum, close, fast, slow)
		s.pending = s.ctx.Close()
	}
}

func (s *LiveSMA) NotifyOrder(o *engine.Order) {
	if o.Status == engine.OrderStatusCompleted {
		side := "BUY"
		if !o.IsBuy() {
			side = "SELL"
		}
		fmt.Printf("  [LIVE] %s FILLED  price=%.2f\n", side, o.ExecPrice)
	}
	if o.IsCompleted() {
		s.pending = nil
	}
}

func avg(line *core.Line, period int) float64 {
	sum := 0.0
	for i := 0; i < period; i++ {
		v := line.Get(-i)
		if math.IsNaN(v) {
			return math.NaN()
		}
		sum += v
	}
	return sum / float64(period)
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	csvPath := "testdata/sample.csv"
	if len(os.Args) > 1 {
		csvPath = os.Args[1]
	}

	fmt.Println("=== gobacktrader Live Demo ===")
	fmt.Printf("Replaying %s via WebSocket at ws://localhost:9876/bars\n\n", csvPath)

	// 1. Start the replay WebSocket server
	srv := replayServer(":9876", csvPath)
	defer srv.Close()

	// 2. Set up Cerebro with a WebSocket live feed
	c := engine.NewCerebro()
	c.SetCash(10_000)
	c.SetCommission(engine.PercentCommission{Percent: 0.001})

	feed := livefeeds.NewWebSocketFeed(livefeeds.WebSocketConfig{
		URL:    "ws://localhost:9876/bars",
		Symbol: "DEMO",
	})
	c.AddData(feed)
	c.AddStrategy(func() engine.Strategy { return &LiveSMA{} })

	ret := &analyzers.Returns{}
	dd := &analyzers.DrawDown{}
	ta := &analyzers.TradeAnalyzer{}
	c.AddAnalyzer(ret)
	c.AddAnalyzer(dd)
	c.AddAnalyzer(ta)

	// 3. Run live (blocks until feed ends or context cancels)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := c.RunLive(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// 4. Print results
	fmt.Printf("\n=== Live Demo Results ===\n")
	r := results[0]
	ret.Print()
	dd.Print()
	ta.Print()
	fmt.Printf("Equity curve: %d data points\n", len(r.EquityCurve))
}
