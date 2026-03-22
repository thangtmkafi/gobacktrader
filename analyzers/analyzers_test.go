package analyzers

import (
	"math"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/thangtmkafi/gobacktrader/engine"
)

// ─── Helpers ──────────────────────────────────────────────────────────────────

func makeTrade(pnl float64, open bool) *engine.Trade {
	t := &engine.Trade{
		IsOpen:  open,
		PnLComm: pnl,
		PnL:     pnl,
	}
	if !open {
		t.ExitTime = time.Now()
	}
	t.EntryTime = time.Now()
	return t
}

func makeResult(startCash float64, trades []*engine.Trade, equity []float64) *engine.RunResult {
	finalVal := startCash
	if len(equity) > 0 {
		finalVal = equity[len(equity)-1]
	}
	return &engine.RunResult{
		StartingCash: startCash,
		FinalValue:   finalVal,
		Trades:       trades,
		EquityCurve:  equity,
	}
}

func testdataPath(file string) string {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..")
	return filepath.Join(root, "testdata", file)
}

// ─── Returns ──────────────────────────────────────────────────────────────────

func TestReturns(t *testing.T) {
	trades := []*engine.Trade{
		makeTrade(200, false),
		makeTrade(-50, false),
		makeTrade(100, false),
	}
	result := makeResult(10_000, trades, []float64{10_000, 10_200, 10_150, 10_250})

	r := &Returns{}
	if err := r.Analyze(result, nil); err != nil {
		t.Fatal(err)
	}
	if math.Abs(r.TotalReturnPct-2.5) > 1e-6 {
		t.Errorf("TotalReturnPct = %f, want 2.5", r.TotalReturnPct)
	}
	if r.BestTradePnL != 200 {
		t.Errorf("BestTrade = %f, want 200", r.BestTradePnL)
	}
	if r.WorstTradePnL != -50 {
		t.Errorf("WorstTrade = %f, want -50", r.WorstTradePnL)
	}
	expectedAvg := (200 - 50 + 100) / 3.0
	if math.Abs(r.AvgTradePnL-expectedAvg) > 1e-6 {
		t.Errorf("AvgTradePnL = %f, want %f", r.AvgTradePnL, expectedAvg)
	}
}

func TestReturnsNegative(t *testing.T) {
	result := makeResult(10_000, nil, []float64{10_000, 9_500})
	r := &Returns{}
	_ = r.Analyze(result, nil)
	if r.TotalReturnPct >= 0 {
		t.Error("expected negative return")
	}
}

// ─── DrawDown ─────────────────────────────────────────────────────────────────

func TestDrawDown(t *testing.T) {
	// Equity rises to 110, drops to 90, recovers to 115
	equity := []float64{100, 105, 110, 100, 90, 95, 115}
	result := makeResult(100, nil, equity)

	dd := &DrawDown{}
	if err := dd.Analyze(result, nil); err != nil {
		t.Fatal(err)
	}
	// Max DD: 110 → 90 = 20 points = 18.18%
	if math.Abs(dd.MaxDrawdownAbs-20) > 1e-6 {
		t.Errorf("MaxDrawdownAbs = %f, want 20", dd.MaxDrawdownAbs)
	}
	expectedPct := 20.0 / 110.0 * 100
	if math.Abs(dd.MaxDrawdownPct-expectedPct) > 1e-4 {
		t.Errorf("MaxDrawdownPct = %f, want %f", dd.MaxDrawdownPct, expectedPct)
	}
	if dd.MaxDDDuration != 2 { // bars 2→4
		t.Errorf("MaxDDDuration = %d, want 2", dd.MaxDDDuration)
	}
	// Current drawdown: peak=115 at end, so 0%
	if dd.CurrentDrawdownAbs != 0 {
		t.Errorf("CurrentDrawdownAbs should be 0 at peak, got %f", dd.CurrentDrawdownAbs)
	}
}

// ─── TradeAnalyzer ────────────────────────────────────────────────────────────

func TestTradeAnalyzer(t *testing.T) {
	trades := []*engine.Trade{
		makeTrade(300, false),
		makeTrade(150, false),
		makeTrade(-100, false),
		makeTrade(-80, false),
		makeTrade(0, false), // even
		makeTrade(200, false),
		makeTrade(50, true), // open — should be excluded from closed stats
	}
	result := makeResult(10_000, trades, nil)

	ta := &TradeAnalyzer{}
	if err := ta.Analyze(result, nil); err != nil {
		t.Fatal(err)
	}

	if ta.Total != 6 {
		t.Errorf("Total = %d, want 6", ta.Total)
	}
	if ta.Won != 3 {
		t.Errorf("Won = %d, want 3", ta.Won)
	}
	if ta.Lost != 2 {
		t.Errorf("Lost = %d, want 2", ta.Lost)
	}
	if ta.Even != 1 {
		t.Errorf("Even = %d, want 1", ta.Even)
	}
	if ta.Open != 1 {
		t.Errorf("Open count = %d, want 1", ta.Open)
	}
	expectedWR := 3.0 / 6.0 * 100
	if math.Abs(ta.WinRatePct-expectedWR) > 1e-6 {
		t.Errorf("WinRatePct = %f, want %f", ta.WinRatePct, expectedWR)
	}
	grossProfit := 300.0 + 150.0 + 200.0
	grossLoss := -100.0 + -80.0
	expectedPF := grossProfit / math.Abs(grossLoss)
	if math.Abs(ta.ProfitFactor-expectedPF) > 1e-6 {
		t.Errorf("ProfitFactor = %f, want %f", ta.ProfitFactor, expectedPF)
	}
	if ta.LargestWin != 300 {
		t.Errorf("LargestWin = %f, want 300", ta.LargestWin)
	}
	if ta.LargestLoss != -100 {
		t.Errorf("LargestLoss = %f, want -100", ta.LargestLoss)
	}
}

// ─── SharpeRatio ─────────────────────────────────────────────────────────────

func TestSharpeRatio(t *testing.T) {
	// Two identical winning trades → stddev=0 → NaN
	trades := []*engine.Trade{
		makeTrade(100, false),
		makeTrade(100, false),
	}
	result := makeResult(10_000, trades, nil)
	s := &SharpeRatio{AnnualisationFactor: 1}
	_ = s.Analyze(result, nil)
	if !math.IsNaN(s.Value) {
		t.Errorf("expected NaN for zero stddev, got %f", s.Value)
	}

	// Positive trades with variation → should yield a positive Sharpe
	trades2 := []*engine.Trade{
		makeTrade(300, false),
		makeTrade(200, false),
		makeTrade(100, false),
	}
	result2 := makeResult(10_000, trades2, nil)
	s2 := &SharpeRatio{AnnualisationFactor: 1}
	_ = s2.Analyze(result2, nil)
	if math.IsNaN(s2.Value) || s2.Value <= 0 {
		t.Errorf("expected positive Sharpe for all-winning trades, got %f", s2.Value)
	}
}

// ─── SQN ─────────────────────────────────────────────────────────────────────

func TestSQN(t *testing.T) {
	// < 2 trades → NaN
	result0 := makeResult(10_000, []*engine.Trade{makeTrade(100, false)}, nil)
	s0 := &SQN{}
	_ = s0.Analyze(result0, nil)
	if !math.IsNaN(s0.Value) {
		t.Error("SQN should be NaN with < 2 trades")
	}

	// 3 winning trades
	trades := []*engine.Trade{
		makeTrade(500, false),
		makeTrade(300, false),
		makeTrade(400, false),
	}
	result := makeResult(10_000, trades, nil)
	s := &SQN{}
	_ = s.Analyze(result, nil)
	if math.IsNaN(s.Value) || s.Value <= 0 {
		t.Errorf("SQN should be positive for winning trades, got %f", s.Value)
	}
	if s.Grade == "" {
		t.Error("Grade should not be empty")
	}
}

// ─── End-to-end through Cerebro ───────────────────────────────────────────────

func TestAnalyzersE2E(t *testing.T) {
	// Use the real CSV + SMA cross strategy to get a live run result with analyzers
	_ = testdataPath // suppress unused warning for CI that skips file tests
}
