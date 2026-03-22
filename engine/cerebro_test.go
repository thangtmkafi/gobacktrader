package engine

import (
	"math"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/thangtmkafi/gobacktrader/core"
	"github.com/thangtmkafi/gobacktrader/feeds"
)

func testdataPath(file string) string {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..")
	return filepath.Join(root, "testdata", file)
}

// ────────────────────────────────────────
// Broker unit tests
// ────────────────────────────────────────

func makeBrokerWithData(t *testing.T) (*Broker, *core.DataSeries) {
	t.Helper()
	cfg := feeds.DefaultYahooConfig(testdataPath("sample.csv"))
	feed := feeds.NewCSVFeed(cfg)
	if err := feed.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	// Advance one bar so we have a current price
	feed.Next()
	return NewBroker(10_000), feed.Data()
}

func TestBrokerMarketBuy(t *testing.T) {
	broker, data := makeBrokerWithData(t)

	order := newOrder(data, OrderSideBuy, OrderTypeMarket, 10, 0, 0)
	broker.Submit(order)

	// Advance data one more bar so broker can fill at open
	cfg := feeds.DefaultYahooConfig(testdataPath("sample.csv"))
	feed2 := feeds.NewCSVFeed(cfg)
	_ = feed2.Load()
	feed2.Next()
	feed2.Next() // second bar

	broker.Next([]*core.DataSeries{feed2.Data()})

	// Market orders should be completed on the next bar
	if order.Status != OrderStatusCompleted {
		t.Fatalf("expected Completed, got %s", order.Status)
	}
	if order.ExecPrice <= 0 {
		t.Error("expected positive exec price")
	}
	expectedCash := 10_000 - order.ExecSize*order.ExecPrice
	if math.Abs(broker.GetCash()-expectedCash) > 1e-6 {
		t.Errorf("expected cash=%f, got %f", expectedCash, broker.GetCash())
	}
}

func TestBrokerLimitOrder(t *testing.T) {
	cfg := feeds.DefaultYahooConfig(testdataPath("sample.csv"))
	feed := feeds.NewCSVFeed(cfg)
	_ = feed.Load()
	feed.Next()

	broker := NewBroker(10_000)
	data := feed.Data()

	// Bar1 close=130.73; set limit below bar2 low=124.44 → should not fill
	order := newOrder(data, OrderSideBuy, OrderTypeLimit, 10, 120.00, 0)
	broker.Submit(order)
	feed.Next()
	broker.Next([]*core.DataSeries{data})
	if order.Status == OrderStatusCompleted {
		t.Error("limit order should not fill when price is above bar's range")
	}

	// Now set limit at a price within bar2's range (High=131.07, Low=124.44)
	order2 := newOrder(data, OrderSideBuy, OrderTypeLimit, 10, 126.00, 0)
	broker.Submit(order2)
	broker.Next([]*core.DataSeries{data}) // same bar, already advanced
	if order2.Status != OrderStatusCompleted {
		t.Errorf("limit order should fill when price is within bar's low/high, got %s", order2.Status)
	}
}

func TestBrokerCancel(t *testing.T) {
	cfg := feeds.DefaultYahooConfig(testdataPath("sample.csv"))
	feed := feeds.NewCSVFeed(cfg)
	_ = feed.Load()
	feed.Next()

	broker := NewBroker(10_000)
	data := feed.Data()

	order := newOrder(data, OrderSideBuy, OrderTypeLimit, 10, 50.00, 0) // won't fill
	broker.Submit(order)
	broker.Cancel(order)
	if order.Status != OrderStatusCanceled {
		t.Errorf("expected Canceled, got %s", order.Status)
	}
}

// ────────────────────────────────────────
// End-to-end SMA crossover test
// ────────────────────────────────────────

type smaCrossStrategy struct {
	ctx    *StrategyContext
	period int
}

func (s *smaCrossStrategy) Init(ctx *StrategyContext) {
	s.ctx = ctx
	s.period = 5
}

func sma(data *core.DataSeries, period, shift int) float64 {
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += data.Close.Get(shift - i)
	}
	return sum / float64(period)
}

func (s *smaCrossStrategy) Next() {
	data := s.ctx.Data()
	if data.Len() < s.period+1 {
		return
	}
	pos := s.ctx.GetPosition(data)
	fast := sma(data, 3, 0)
	slow := sma(data, s.period, 0)
	fastPrev := sma(data, 3, -1)
	slowPrev := sma(data, s.period, -1)

	if !pos.IsOpen() && fast > slow && fastPrev <= slowPrev {
		s.ctx.Buy(10)
	} else if pos.IsOpen() && fast < slow && fastPrev >= slowPrev {
		s.ctx.Close()
	}
}

func TestSMACrossover(t *testing.T) {
	c := NewCerebro()
	c.SetCash(10_000)

	csvCfg := feeds.DefaultYahooConfig(testdataPath("sample.csv"))
	c.AddCSV(csvCfg)
	c.AddStrategy(func() Strategy {
		return &smaCrossStrategy{period: 5}
	})

	results, err := c.Run()
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no results")
	}

	r := results[0]
	t.Logf("Starting cash: %.2f", r.StartingCash)
	t.Logf("Final value:   %.2f", r.FinalValue)
	t.Logf("Trades:        %d", len(r.Trades))

	if r.FinalValue <= 0 {
		t.Error("final value should be positive")
	}
}
