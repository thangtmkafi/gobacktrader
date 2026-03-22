// gobacktrader – main entrypoint.
// Runs a SMA crossover strategy with all Phase 3 analyzers attached.
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/thangtmkafi/gobacktrader/analyzers"
	"github.com/thangtmkafi/gobacktrader/engine"
	"github.com/thangtmkafi/gobacktrader/feeds"
	"github.com/thangtmkafi/gobacktrader/indicators"
)

type quickSMA struct {
	ctx     *engine.StrategyContext
	fast    *indicators.SMA
	slow    *indicators.SMA
	pending *engine.Order
}

func (s *quickSMA) Init(ctx *engine.StrategyContext) {
	s.ctx = ctx
	s.fast = indicators.NewSMA(ctx.PreloadedData(), 5)
	s.slow = indicators.NewSMA(ctx.PreloadedData(), 20)
}

func (s *quickSMA) Next() {
	if s.pending != nil {
		return
	}
	pos := s.ctx.GetPosition(s.ctx.Data())
	fn := s.fast.Line().Get(0)
	sn := s.slow.Line().Get(0)
	fp := s.fast.Line().Get(-1)
	sp := s.slow.Line().Get(-1)
	if math.IsNaN(fn) || math.IsNaN(sn) || math.IsNaN(fp) || math.IsNaN(sp) {
		return
	}
	if !pos.IsOpen() && fp <= sp && fn > sn {
		s.pending = s.ctx.Buy(10)
	} else if pos.IsOpen() && fp >= sp && fn < sn {
		s.pending = s.ctx.Close()
	}
}

func (s *quickSMA) NotifyOrder(o *engine.Order) {
	if o.IsCompleted() {
		s.pending = nil
	}
}

func main() {
	csvPath := "testdata/sample.csv"
	if len(os.Args) > 1 {
		csvPath = os.Args[1]
	}

	c := engine.NewCerebro()
	c.SetCash(10_000)
	c.SetCommission(engine.PercentCommission{Percent: 0.001})
	c.AddCSV(feeds.DefaultYahooConfig(csvPath))
	c.AddStrategy(func() engine.Strategy { return &quickSMA{} })

	// Attach all Phase 3 analyzers
	ret := &analyzers.Returns{}
	sharpe := &analyzers.SharpeRatio{AnnualisationFactor: 252}
	dd := &analyzers.DrawDown{}
	ta := &analyzers.TradeAnalyzer{}
	sqn := &analyzers.SQN{}
	c.AddAnalyzer(ret)
	c.AddAnalyzer(sharpe)
	c.AddAnalyzer(dd)
	c.AddAnalyzer(ta)
	c.AddAnalyzer(sqn)

	results, err := c.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	r := results[0]
	fmt.Printf("=== gobacktrader SMA(5)/SMA(20) Crossover ===\n\n")
	ret.Print()
	sharpe.Print()
	dd.Print()
	ta.Print()
	sqn.Print()
	fmt.Printf("Equity curve bars: %d\n", len(r.EquityCurve))
}
