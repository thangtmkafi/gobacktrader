// Command sma_cross demonstrates a SMA crossover strategy using the gobacktrader
// indicators package (Phase 2). The hand-rolled simpleMA helper is replaced by
// indicators.NewSMA.
//
// Usage:
//
//	go run ./examples/sma_cross [path/to/data.csv]
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/thangtmkafi/gobacktrader/engine"
	"github.com/thangtmkafi/gobacktrader/feeds"
	"github.com/thangtmkafi/gobacktrader/indicators"
	"github.com/thangtmkafi/gobacktrader/plotter"
)

// ─── Strategy ────────────────────────────────────────────────────────────────

type SMACrossStrategy struct {
	ctx      *engine.StrategyContext
	fastSMA  *indicators.SMA
	slowSMA  *indicators.SMA
	orderRef *engine.Order
}

func (s *SMACrossStrategy) Init(ctx *engine.StrategyContext) {
	s.ctx = ctx
	data := ctx.PreloadedData() // use pre-loaded series for indicator building
	s.fastSMA = indicators.NewSMA(data, 5)
	s.slowSMA = indicators.NewSMA(data, 20)
	fmt.Printf("Loaded %d bars.  %s  %s\n\n",
		data.Len(), s.fastSMA.Name(), s.slowSMA.Name())
}

func (s *SMACrossStrategy) NotifyOrder(o *engine.Order) {
	if o.Status == engine.OrderStatusCompleted {
		side := "BUY"
		if !o.IsBuy() {
			side = "SELL"
		}
		s.ctx.Log("%s EXECUTED  Price: %.2f  Size: %.0f  Comm: %.2f",
			side, o.ExecPrice, o.ExecSize, o.ExecComm)
	} else if o.Status == engine.OrderStatusMargin || o.Status == engine.OrderStatusRejected {
		s.ctx.Log("ORDER REJECTED")
		s.orderRef = nil
	}
	if o.IsCompleted() {
		s.orderRef = nil
	}
}

func (s *SMACrossStrategy) NotifyTrade(t *engine.Trade) {
	if !t.IsOpen {
		s.ctx.Log("TRADE CLOSED  Gross: %.2f  Net: %.2f", t.PnL, t.PnLComm)
	}
}

func (s *SMACrossStrategy) Next() {
	if s.orderRef != nil {
		return
	}
	pos := s.ctx.GetPosition(s.ctx.Data())

	fastNow := s.fastSMA.Line().Get(0)
	slowNow := s.slowSMA.Line().Get(0)
	fastPrev := s.fastSMA.Line().Get(-1)
	slowPrev := s.slowSMA.Line().Get(-1)

	if math.IsNaN(fastNow) || math.IsNaN(slowNow) || math.IsNaN(fastPrev) || math.IsNaN(slowPrev) {
		return
	}

	if !pos.IsOpen() && fastPrev <= slowPrev && fastNow > slowNow {
		s.ctx.Log("BUY SIGNAL   Fast=%.2f  Slow=%.2f", fastNow, slowNow)
		s.orderRef = s.ctx.Buy(10)
	} else if pos.IsOpen() && fastPrev >= slowPrev && fastNow < slowNow {
		s.ctx.Log("SELL SIGNAL  Fast=%.2f  Slow=%.2f", fastNow, slowNow)
		s.orderRef = s.ctx.Close()
	}
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	csvPath := "testdata/sample.csv"
	if len(os.Args) > 1 {
		csvPath = os.Args[1]
	}

	fmt.Printf("=== gobacktrader SMA Cross (Phase 2 — indicators package) ===\n")
	fmt.Printf("Data: %s\n\n", csvPath)

	c := engine.NewCerebro()
	c.SetCash(10_000)
	c.SetCommission(engine.PercentCommission{Percent: 0.001})
	csvCfg := feeds.DefaultYahooConfig(csvPath)
	feed := feeds.NewCSVFeed(csvCfg)
	c.AddData(feed)
	
	c.AddStrategy(func() engine.Strategy { return &SMACrossStrategy{} })

	results, err := c.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	r := results[0]
	fmt.Printf("\n=== Results ===\n")
	fmt.Printf("Starting cash: $%10.2f\n", r.StartingCash)
	fmt.Printf("Final value:   $%10.2f\n", r.FinalValue)
	fmt.Printf("PnL:           $%10.2f  (%.2f%%)\n",
		r.FinalValue-r.StartingCash,
		(r.FinalValue-r.StartingCash)/r.StartingCash*100)
	fmt.Printf("Trades:        %d\n", len(r.Trades))
	for i, t := range r.Trades {
		status := "CLOSED"
		if t.IsOpen {
			status = "OPEN"
		}
		fmt.Printf("  Trade %d [%s]  Entry: %.2f  Exit: %.2f  NetPnL: %.2f\n",
			i+1, status, t.EntryPrice, t.ExitPrice, t.PnLComm)
	}

	// Generate interactive chart
	err = plotter.Plot(feed.Data(), r, "chart.html")
	if err != nil {
		fmt.Printf("Error generating plot: %v\n", err)
	} else {
		fmt.Printf("\nChart successfully generated at chart.html\n")
	}
}
