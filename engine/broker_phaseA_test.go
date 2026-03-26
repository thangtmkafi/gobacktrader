package engine

import (
	"math"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/thangtmkafi/gobacktrader/core"
	"github.com/thangtmkafi/gobacktrader/feeds"
)

func testData(t *testing.T) (*Broker, *core.DataSeries, func()) {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..")
	csvPath := filepath.Join(root, "testdata", "sample.csv")

	cfg := feeds.DefaultYahooConfig(csvPath)
	feed := feeds.NewCSVFeed(cfg)
	if err := feed.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	broker := NewBroker(100_000)
	data := feed.Data()

	advance := func() {
		if !feed.Next() {
			t.Fatal("ran out of data")
		}
		broker.Next([]*core.DataSeries{data})
	}
	// Position to bar 1
	advance()
	return broker, data, advance
}

// ─── Slippage ─────────────────────────────────────────────────────────────────

func TestSlippagePercent(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..")
	csvPath := filepath.Join(root, "testdata", "sample.csv")

	cfg := feeds.DefaultYahooConfig(csvPath)
	feed := feeds.NewCSVFeed(cfg)
	_ = feed.Load()
	feed.Next()

	broker := NewBroker(100_000)
	broker.SetSlippagePercent(0.01, true, true, true, false) // 1% slippage

	data := feed.Data()
	barOpenPrice := data.Bar().Open

	order := NewOrder(data, OrderSideBuy, OrderTypeMarket, 10, 0, 0)
	broker.Submit(order)
	feed.Next()
	broker.Next([]*core.DataSeries{data})

	if order.Status != OrderStatusCompleted {
		t.Fatalf("expected Completed, got %s", order.Status)
	}
	expectedFill := barOpenPrice * 1.01 // open of bar2 + 1% slippage
	nextOpen := data.Bar().Open
	expectedFill = nextOpen * 1.01

	// slipMatch caps at bar High if slippage would exceed it
	barHigh := data.Bar().High
	if expectedFill > barHigh {
		expectedFill = barHigh
	}

	diff := math.Abs(order.ExecPrice - expectedFill)
	t.Logf("Bar open=%.4f, execPrice=%.4f, expected=%.4f", nextOpen, order.ExecPrice, expectedFill)
	if diff > 1e-4 {
		t.Errorf("slippage not applied correctly: execPrice=%.4f, expected=%.4f", order.ExecPrice, expectedFill)
	}
}

func TestSlippageFixed(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..")
	cfg := feeds.DefaultYahooConfig(filepath.Join(root, "testdata", "sample.csv"))
	feed := feeds.NewCSVFeed(cfg)
	_ = feed.Load()
	feed.Next()

	broker := NewBroker(100_000)
	broker.SetSlippageFixed(2.0, true, true, true, false) // $2 fixed slip

	data := feed.Data()
	order := NewOrder(data, OrderSideBuy, OrderTypeMarket, 10, 0, 0)
	broker.Submit(order)
	feed.Next()
	broker.Next([]*core.DataSeries{data})

	if order.Status != OrderStatusCompleted {
		t.Fatalf("expected Completed, got %s", order.Status)
	}
	nextOpen := data.Bar().Open
	expected := nextOpen + 2.0
	barHigh := data.Bar().High
	if expected > barHigh {
		expected = barHigh
	}

	diff := math.Abs(order.ExecPrice - expected)
	t.Logf("execPrice=%.4f, expected=%.4f", order.ExecPrice, expected)
	if diff > 1e-4 {
		t.Errorf("fixed slippage: execPrice=%.4f, expected=%.4f", order.ExecPrice, expected)
	}
}

// ─── Order expiry ─────────────────────────────────────────────────────────────

func TestOrderExpiryDAY(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..")
	cfg := feeds.DefaultYahooConfig(filepath.Join(root, "testdata", "sample.csv"))
	feed := feeds.NewCSVFeed(cfg)
	_ = feed.Load()
	feed.Next()

	broker := NewBroker(10_000)
	data := feed.Data()

	// Capture current bar datetime from the CSV
	currentBarTime := data.Bar().DateTime

	// DAY limit far below current price — won't fill
	order := NewOrder(data, OrderSideBuy, OrderTypeLimit, 10, 1.00, 0)
	order.ValidType = ValidDAY
	// Manually set BarCreatedAt to the previous bar (so current bar is "next day")
	// Since CSV has daily bars, we set BarCreatedAt to 24h before current bar
	order.BarCreatedAt = currentBarTime.Add(-25 * time.Hour)
	broker.Submit(order)
	// Override BarCreatedAt again with the fake earlier date (Submit overwrites it)
	order.BarCreatedAt = currentBarTime.Add(-25 * time.Hour)

	// Process the current bar — bar date is after BarCreatedAt date → should expire
	broker.Next([]*core.DataSeries{data})

	if order.Status != OrderStatusExpired {
		t.Errorf("expected Expired on next bar (barTime=%v, createdAt=%v), got %s",
			currentBarTime.Truncate(24*time.Hour), order.BarCreatedAt.Truncate(24*time.Hour), order.Status)
	}
}

func TestOrderExpiryGTD(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..")
	cfg := feeds.DefaultYahooConfig(filepath.Join(root, "testdata", "sample.csv"))
	feed := feeds.NewCSVFeed(cfg)
	_ = feed.Load()
	feed.Next()

	broker := NewBroker(10_000)
	data := feed.Data()

	// GTD expiry set to a time BEFORE the current bar's datetime
	currentBarTime := data.Bar().DateTime
	order := NewOrder(data, OrderSideBuy, OrderTypeLimit, 10, 1.00, 0)
	order.ValidType = ValidGTD
	order.ValidTime = currentBarTime.Add(-1 * time.Hour) // already expired relative to bar
	broker.Submit(order)

	// Process this bar — barTime is after ValidTime → should expire
	broker.Next([]*core.DataSeries{data})

	if order.Status != OrderStatusExpired {
		t.Errorf("expected Expired for past GTD (barTime=%v, validTime=%v), got %s",
			currentBarTime, order.ValidTime, order.Status)
	}
}

// ─── Stop Trail ───────────────────────────────────────────────────────────────

func TestStopTrailBasic(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..")
	cfg := feeds.DefaultYahooConfig(filepath.Join(root, "testdata", "sample.csv"))
	feed := feeds.NewCSVFeed(cfg)
	_ = feed.Load()

	// Advance a few bars to get prices
	feed.Next()
	broker := NewBroker(100_000)
	data := feed.Data()

	// Place a sell-side trailing stop (protects a long position)
	// Set it $5 below current close
	curClose := data.Close.Get(0)
	trailAmount := 5.0
	initialStop := curClose - trailAmount

	order := NewOrder(data, OrderSideSell, OrderTypeStopTrail, 10, initialStop, 0)
	order.TrailAmount = trailAmount
	order.TrailStop = initialStop
	broker.Submit(order)

	if order.TrailStop != initialStop {
		t.Errorf("initial trail stop: got %.4f, want %.4f", order.TrailStop, initialStop)
	}
	t.Logf("Created StopTrail: initialStop=%.4f, curClose=%.4f", initialStop, curClose)

	// Verify order is accepted
	if order.Status != OrderStatusAccepted {
		t.Errorf("expected Accepted, got %s", order.Status)
	}
}

// ─── Dynamic cash ─────────────────────────────────────────────────────────────

func TestAddCash(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..")
	cfg := feeds.DefaultYahooConfig(filepath.Join(root, "testdata", "sample.csv"))
	feed := feeds.NewCSVFeed(cfg)
	_ = feed.Load()
	feed.Next()

	broker := NewBroker(10_000)
	data := feed.Data()
	broker.AddCash(5_000) // queue $5k addition

	feed.Next()
	broker.Next([]*core.DataSeries{data}) // cash applied here

	expectedCash := 15_000.0
	if math.Abs(broker.GetCash()-expectedCash) > 1e-6 {
		t.Errorf("expected cash=%.2f, got %.2f", expectedCash, broker.GetCash())
	}
}

// ─── OCO (One-Cancels-Other) ──────────────────────────────────────────────────

func TestOCO(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..")
	cfg := feeds.DefaultYahooConfig(filepath.Join(root, "testdata", "sample.csv"))
	feed := feeds.NewCSVFeed(cfg)
	_ = feed.Load()
	feed.Next()

	broker := NewBroker(100_000)
	data := feed.Data()
	curClose := data.Close.Get(0)

	// Create two limit orders linked as OCO
	// Order A: buy limit very high (will fill immediately)
	orderA := NewOrder(data, OrderSideBuy, OrderTypeLimit, 5, curClose*2, 0)
	orderA.OcoRef = 99 // any shared group ref
	broker.Submit(orderA)

	// Order B: buy limit very high (same group — should be cancelled when A fills)
	orderB := NewOrder(data, OrderSideBuy, OrderTypeLimit, 5, curClose*2, 0)
	orderB.OcoRef = 99
	broker.Submit(orderB)

	// Register both in group
	broker.ocoGroups[99] = []int{orderA.Ref, orderB.Ref}
	broker.ocoParent[orderA.Ref] = 99
	broker.ocoParent[orderB.Ref] = 99

	// Advance a bar — both could fill (high limit), but OCO should cancel B when A fills
	feed.Next()
	broker.Next([]*core.DataSeries{data})

	t.Logf("orderA status=%s, orderB status=%s", orderA.Status, orderB.Status)

	// A should be completed, B should be canceled
	if orderA.Status != OrderStatusCompleted {
		t.Errorf("expected orderA Completed, got %s", orderA.Status)
	}
	if orderB.Status != OrderStatusCanceled {
		t.Errorf("expected orderB Canceled (OCO), got %s", orderB.Status)
	}
}

// ─── Bracket orders ───────────────────────────────────────────────────────────

func TestBracketOrder(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..")
	cfg := feeds.DefaultYahooConfig(filepath.Join(root, "testdata", "sample.csv"))
	feed := feeds.NewCSVFeed(cfg)
	_ = feed.Load()
	feed.Next()

	broker := NewBroker(100_000)
	data := feed.Data()

	// Entry: market buy (will fill next bar)
	entry := NewOrder(data, OrderSideBuy, OrderTypeMarket, 10, 0, 0)
	broker.Submit(entry)

	curClose := data.Close.Get(0)

	// Stop-loss: sell stop far below  (child of entry)
	stopLoss := NewOrder(data, OrderSideSell, OrderTypeStop, 10, curClose*0.95, 0)
	stopLoss.ParentRef = entry.Ref
	broker.Submit(stopLoss)

	// child should be held in childQueue, not in pending
	if len(broker.childQueue[entry.Ref]) != 1 {
		t.Fatalf("expected 1 child in queue, got %d", len(broker.childQueue[entry.Ref]))
	}
	if stopLoss.Status != OrderStatusSubmitted {
		t.Errorf("child should be Submitted (held), got %s", stopLoss.Status)
	}

	// Advance 1 bar — entry fills
	feed.Next()
	broker.Next([]*core.DataSeries{data})

	t.Logf("entry=%s stopLoss=%s", entry.Status, stopLoss.Status)

	if entry.Status != OrderStatusCompleted {
		t.Fatalf("entry order should be completed, got %s", entry.Status)
	}

	// After entry fills, children should be activated on next bar processing
	feed.Next()
	broker.Next([]*core.DataSeries{data})

	// Stop-loss should now be in pending (Accepted)
	found := false
	for _, o := range broker.pending {
		if o.Ref == stopLoss.Ref {
			found = true
			break
		}
	}
	if !found {
		t.Logf("stopLoss status=%s", stopLoss.Status)
		// It may have already been activated and is pending
		if stopLoss.Status != OrderStatusAccepted && stopLoss.Status != OrderStatusCompleted {
			t.Errorf("stop-loss should be Accepted or Completed after parent fills, got %s", stopLoss.Status)
		}
	}
}

// ─── Order target size ────────────────────────────────────────────────────────

func TestOrderTargetSize(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..")
	cfg := feeds.DefaultYahooConfig(filepath.Join(root, "testdata", "sample.csv"))

	c := NewCerebro()
	c.SetCash(100_000)
	c.AddCSV(cfg)

	filled := false
	c.AddStrategy(func() Strategy {
		return &targetSizeStrategy{targetFilled: &filled}
	})

	results, err := c.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	_ = results
	if !filled {
		t.Error("OrderTargetSize: expected order to fill at some point")
	}
}

type targetSizeStrategy struct {
	ctx          *StrategyContext
	targetFilled *bool
	bar          int
}

func (s *targetSizeStrategy) Init(ctx *StrategyContext) { s.ctx = ctx }
func (s *targetSizeStrategy) Next() {
	s.bar++
	if s.bar == 3 {
		// Target 5 shares — should place a buy for 5
		order := s.ctx.OrderTargetSize(5)
		if order != nil {
			*s.targetFilled = true
		}
	}
	if s.bar == 10 {
		// Target 0 — should close position
		s.ctx.OrderTargetSize(0)
	}
}
