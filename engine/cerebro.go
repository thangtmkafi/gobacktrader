// Package engine contains the backtesting engine components.
// This file implements Cerebro — the main orchestration engine —
// mirroring backtrader's cerebro.py.
package engine

import (
	"context"
	"fmt"

	"github.com/thangtmkafi/gobacktrader/core"
	"github.com/thangtmkafi/gobacktrader/feeds"
)

// DataFeed is implemented by any source that can supply bar-by-bar data.
type DataFeed interface {
	// Load pre-loads all data. Called before the backtest loop.
	Load() error
	// Next advances by one bar. Returns false when data is exhausted.
	Next() bool
	// Data returns the underlying DataSeries (populated progressively).
	Data() *core.DataSeries
}

// PreloadedDataFeed is an optional extension of DataFeed for feeds that can
// also provide a fully pre-populated DataSeries for indicator construction.
type PreloadedDataFeed interface {
	DataFeed
	// PreloadedData returns a DataSeries with all bars already loaded.
	PreloadedData() *core.DataSeries
}

// StrategyFactory is a function that constructs a new Strategy instance.
// Using a factory allows Cerebro to create fresh instances for optimization runs.
type StrategyFactory func() Strategy

// RunResult contains the results of a completed backtest.
type RunResult struct {
	StartingCash float64
	FinalValue   float64
	Trades       []*Trade
	// EquityCurve holds the total portfolio value after every bar (oldest → newest).
	EquityCurve []float64
	// CashCurve holds the available cash after every bar.
	CashCurve []float64
}

// Analyzer is implemented by any post-run performance analysis component.
// Cerebro calls Analyze() once after Run() completes.
type Analyzer interface {
	// Name returns the analyzer label used in Print output.
	Name() string
	// Analyze populates internal state from the run result.
	Analyze(result *RunResult, datas []*core.DataSeries) error
	// Print writes a formatted summary to stdout.
	Print()
}

// Cerebro is the main backtest engine.
type Cerebro struct {
	feeds      []DataFeed
	factories  []StrategyFactory
	analyzers  []Analyzer
	broker     BrokerBase
	startCash  float64
}

// NewCerebro creates a new Cerebro with default settings.
func NewCerebro() *Cerebro {
	c := &Cerebro{
		startCash: 10_000,
	}
	c.broker = NewBroker(c.startCash)
	return c
}

// SetCash sets the initial cash for the broker.
func (c *Cerebro) SetCash(cash float64) {
	c.startCash = cash
	c.broker.SetCash(cash)
}

// SetCommission sets the commission scheme.
func (c *Cerebro) SetCommission(comm CommissionInfo) {
	c.broker.SetCommission(comm)
}

// SetBroker replaces the default broker.
func (c *Cerebro) SetBroker(broker BrokerBase) {
	c.broker = broker
}

// AddData adds a data feed to Cerebro.
func (c *Cerebro) AddData(feed DataFeed) {
	c.feeds = append(c.feeds, feed)
}

// AddCSV is a convenience helper that creates and adds a CSVFeed.
func (c *Cerebro) AddCSV(cfg feeds.CSVFeedConfig) {
	c.feeds = append(c.feeds, feeds.NewCSVFeed(cfg))
}

// AddStrategy registers a strategy factory.
func (c *Cerebro) AddStrategy(factory StrategyFactory) {
	c.factories = append(c.factories, factory)
}

// AddAnalyzer attaches an analyzer that will be populated after Run().
func (c *Cerebro) AddAnalyzer(a Analyzer) {
	c.analyzers = append(c.analyzers, a)
}

// Run executes the backtest and returns results for each strategy.
// Attached analyzers are populated and printed automatically.
func (c *Cerebro) Run() ([]*RunResult, error) {
	if len(c.feeds) == 0 {
		return nil, fmt.Errorf("cerebro: no data feeds added")
	}
	if len(c.factories) == 0 {
		return nil, fmt.Errorf("cerebro: no strategies added")
	}

	// 1. Load all data feeds
	for _, feed := range c.feeds {
		if err := feed.Load(); err != nil {
			return nil, fmt.Errorf("cerebro: load feed: %w", err)
		}
	}

	// 2. Collect all DataSeries references (live + pre-loaded)
	datas := make([]*core.DataSeries, len(c.feeds))
	preloaded := make([]*core.DataSeries, len(c.feeds))
	for i, feed := range c.feeds {
		datas[i] = feed.Data()
		if pf, ok := feed.(PreloadedDataFeed); ok {
			preloaded[i] = pf.PreloadedData()
		} else {
			preloaded[i] = feed.Data() // fallback
		}
	}

	// 3. Initialise strategies — pass pre-loaded data so indicators can be built
	ctx := &StrategyContext{
		Datas:          datas,
		PreloadedDatas: preloaded,
		broker:         c.broker,
	}
	strategies := make([]Strategy, len(c.factories))
	for i, factory := range c.factories {
		s := factory()
		s.Init(ctx)
		strategies[i] = s
	}

	// Call Start() on strategies that support it
	for _, s := range strategies {
		if st, ok := s.(Starter); ok {
			st.Start()
		}
	}

	startingCash := c.broker.GetCash()
	var equityCurve []float64
	var cashCurve []float64

	// 4. Main backtest loop
	for {
		// Advance all feeds; stop when any feed runs out
		advanced := false
		allAdvanced := true
		for _, feed := range c.feeds {
			if feed.Next() {
				advanced = true
			} else {
				allAdvanced = false
			}
		}
		if !advanced || !allAdvanced {
			break
		}

		// Let broker process pending orders against the new bar
		c.broker.Next(datas)

		// Record Cash and Value after broker processes this bar
		equityCurve = append(equityCurve, c.broker.GetValue(datas))
		cashCurve = append(cashCurve, c.broker.GetCash())

		// Drain order/trade notifications and deliver to strategies
		orders, trades := c.broker.DrainNotifications()
		cash := c.broker.GetCash()
		value := c.broker.GetValue(datas)
		for _, s := range strategies {
			if no, ok := s.(NotifyOrderer); ok {
				for _, o := range orders {
					no.NotifyOrder(o)
				}
			}
			if nt, ok := s.(NotifyTrader); ok {
				for _, t := range trades {
					nt.NotifyTrade(t)
				}
			}
			if cv, ok := s.(CashValueNotifier); ok {
				cv.NotifyCashValue(cash, value)
			}
		}

		// Evaluate and deliver timers
		now := datas[0].Bar().DateTime
		triggeredTimers := ctx.popTriggeredTimers(now)
		if len(triggeredTimers) > 0 {
			for _, s := range strategies {
				if nt, ok := s.(NotifyTimer); ok {
					for _, t := range triggeredTimers {
						nt.NotifyTimer(t, now)
					}
				}
			}
		}

		// Call strategy Next()
		for _, s := range strategies {
			s.Next()
		}
	}

	// Call Stop() on strategies that support it
	for _, s := range strategies {
		if st, ok := s.(Stopper); ok {
			st.Stop()
		}
	}

	// 5. Build results
	var results []*RunResult
	for range strategies {
		results = append(results, &RunResult{
			StartingCash: startingCash,
			FinalValue:   c.broker.GetValue(datas),
			Trades:       c.broker.GetTrades(),
			EquityCurve:  equityCurve,
			CashCurve:    cashCurve,
		})
	}

	// 6. Run analyzers on every result
	for _, result := range results {
		for _, az := range c.analyzers {
			if err := az.Analyze(result, datas); err != nil {
				return nil, fmt.Errorf("cerebro: analyzer %q: %w", az.Name(), err)
			}
		}
	}

	return results, nil
}

// LiveFeed is implemented by real-time data sources (defined in livefeeds package).
// Cerebro detects feeds implementing this interface in RunLive.
type LiveFeed interface {
	DataFeed
	Start(ctx context.Context) error
	Stop() error
}

// RunLive runs the engine in live/paper-trading mode.
// Live feeds are started and Next() blocks until a bar arrives or ctx is cancelled.
// Attach CSV feeds first for indicator warm-up, then live feeds for real-time data.
func (c *Cerebro) RunLive(ctx context.Context) ([]*RunResult, error) {
	if len(c.feeds) == 0 {
		return nil, fmt.Errorf("cerebro: no data feeds added")
	}
	if len(c.factories) == 0 {
		return nil, fmt.Errorf("cerebro: no strategies added")
	}

	// 1. Load all data feeds (CSV feeds pre-load; live feeds no-op)
	for _, feed := range c.feeds {
		if err := feed.Load(); err != nil {
			return nil, fmt.Errorf("cerebro: load feed: %w", err)
		}
	}

	// 2. Start live feeds
	var liveFeeds []LiveFeed
	for _, feed := range c.feeds {
		if lf, ok := feed.(LiveFeed); ok {
			if err := lf.Start(ctx); err != nil {
				return nil, fmt.Errorf("cerebro: start live feed: %w", err)
			}
			liveFeeds = append(liveFeeds, lf)
		}
	}

	// 3. Collect DataSeries references
	datas := make([]*core.DataSeries, len(c.feeds))
	preloaded := make([]*core.DataSeries, len(c.feeds))
	for i, feed := range c.feeds {
		datas[i] = feed.Data()
		if pf, ok := feed.(PreloadedDataFeed); ok {
			preloaded[i] = pf.PreloadedData()
		} else {
			preloaded[i] = feed.Data()
		}
	}

	// 4. Initialise strategies
	sctx := &StrategyContext{
		Datas:          datas,
		PreloadedDatas: preloaded,
		broker:         c.broker,
	}
	strategies := make([]Strategy, len(c.factories))
	for i, factory := range c.factories {
		s := factory()
		s.Init(sctx)
		strategies[i] = s
	}

	startingCash := c.broker.GetCash()
	var equityCurve []float64
	var cashCurve []float64

	// 5. Main live loop — same as backtest but Next() blocks
	for {
		advanced := false
		allAdvanced := true
		for _, feed := range c.feeds {
			if feed.Next() {
				advanced = true
			} else {
				allAdvanced = false
			}
		}
		if !advanced || !allAdvanced {
			break
		}

		c.broker.Next(datas)
		equityCurve = append(equityCurve, c.broker.GetValue(datas))
		cashCurve = append(cashCurve, c.broker.GetCash())

		orders, trades := c.broker.DrainNotifications()
		for _, s := range strategies {
			if no, ok := s.(NotifyOrderer); ok {
				for _, o := range orders {
					no.NotifyOrder(o)
				}
			}
			if nt, ok := s.(NotifyTrader); ok {
				for _, t := range trades {
					nt.NotifyTrade(t)
				}
			}
		}

		// Evaluate and deliver timers
		now := datas[0].Bar().DateTime
		triggeredTimers := sctx.popTriggeredTimers(now)
		if len(triggeredTimers) > 0 {
			for _, s := range strategies {
				if nt, ok := s.(NotifyTimer); ok {
					for _, t := range triggeredTimers {
						nt.NotifyTimer(t, now)
					}
				}
			}
		}

		for _, s := range strategies {
			s.Next()
		}
	}

	// 6. Stop live feeds
	for _, lf := range liveFeeds {
		_ = lf.Stop()
	}

	// 7. Build results
	var results []*RunResult
	for range strategies {
		results = append(results, &RunResult{
			StartingCash: startingCash,
			FinalValue:   c.broker.GetValue(datas),
			Trades:       c.broker.GetTrades(),
			EquityCurve:  equityCurve,
			CashCurve:    cashCurve,
		})
	}

	for _, result := range results {
		for _, az := range c.analyzers {
			if err := az.Analyze(result, datas); err != nil {
				return nil, fmt.Errorf("cerebro: analyzer %q: %w", az.Name(), err)
			}
		}
	}

	return results, nil
}
