package engine

import (
	"math"
	"testing"
)

// ─── CommInfo tests ───────────────────────────────────────────────────────────

func TestCommInfoPercent(t *testing.T) {
	c := NewStockCommInfo(0.001) // 0.1%
	comm := c.GetCommission(100, 50.0)
	expected := 100 * 50.0 * 0.001
	if math.Abs(comm-expected) > 1e-9 {
		t.Errorf("percent commission: got %.6f, want %.6f", comm, expected)
	}
}

func TestCommInfoFixed(t *testing.T) {
	c := CommInfo{
		Commission: 0.01, // $0.01 per share
		CommType:   CommTypeFixed,
		Mult:       1.0,
		Leverage:   1.0,
	}
	comm := c.GetCommission(200, 100.0) // 200 shares, price doesn't matter
	expected := 200 * 0.01
	if math.Abs(comm-expected) > 1e-9 {
		t.Errorf("fixed commission: got %.6f, want %.6f", comm, expected)
	}
}

func TestFuturesCommInfo(t *testing.T) {
	// ES futures: $10/contract, $12,500 margin, 50x multiplier
	c := NewFuturesCommInfo(10.0, 12500.0, 50.0)

	// Commission: 1 contract × $10
	comm := c.GetCommission(1, 4500.0)
	if math.Abs(comm-10.0) > 1e-9 {
		t.Errorf("futures commission: got %.4f, want 10.00", comm)
	}

	// Value: 1 contract × 4500 × 50 = 225,000
	val := c.GetValue(1, 4500.0)
	if math.Abs(val-225000.0) > 1e-6 {
		t.Errorf("futures value: got %.2f, want 225000.00", val)
	}

	// Margin: 1 contract × $12,500
	margin := c.GetMargin(1, 4500.0)
	if math.Abs(margin-12500.0) > 1e-6 {
		t.Errorf("futures margin: got %.2f, want 12500.00", margin)
	}

	// PnL: 10 tick move × $12.50/tick × 4 = price move of 2.5 on 1 contract
	// PnL = 1 contract × (4502.5 - 4500) × 50 = 125
	pnl := c.ProfitAndLoss(1, 4500.0, 4502.5)
	if math.Abs(pnl-125.0) > 1e-6 {
		t.Errorf("futures PnL: got %.2f, want 125.00", pnl)
	}
}

func TestInterestCharge(t *testing.T) {
	c := CommInfo{
		Commission:   0.001,
		CommType:     CommTypePerc,
		PercAbs:      true,
		Mult:         1.0,
		Leverage:     1.0,
		Interest:     0.06, // 6% yearly interest
		InterestLong: false,
		StockLike:    true,
	}

	// Short 100 shares at $50 for 10 days
	interest := c.DailyInterest(-100, 50.0, 10)
	// expected = 10 * 50 * 100 * (0.06/365) = 0.8219...
	expected := 10.0 * 50.0 * 100.0 * (0.06 / 365.0)
	if math.Abs(interest-expected) > 1e-6 {
		t.Errorf("interest: got %.6f, want %.6f", interest, expected)
	}

	// Long position should NOT be charged when InterestLong = false
	interestLong := c.DailyInterest(100, 50.0, 10)
	if interestLong != 0 {
		t.Errorf("long interest should be 0 when InterestLong=false, got %.6f", interestLong)
	}
}

func TestAutoMargin(t *testing.T) {
	c := CommInfo{
		Commission: 0.001,
		CommType:   CommTypePerc,
		PercAbs:    true,
		Mult:       1.0,
		AutoMargin: 0.1, // margin = 10% of price
		Leverage:   10.0,
		StockLike:  false,
	}

	margin := c.GetMargin(5, 100.0)
	expected := 5 * 100.0 * 0.1
	if math.Abs(margin-expected) > 1e-6 {
		t.Errorf("automargin: got %.4f, want %.4f", margin, expected)
	}
}

func TestLeverage(t *testing.T) {
	c := CommInfo{
		Commission: 0.001,
		CommType:   CommTypePerc,
		PercAbs:    true,
		Mult:       1.0,
		Leverage:   4.0, // 4× leverage
		StockLike:  true,
	}

	// With 4× leverage, cash cost = notional / 4
	cashCost := c.CashCost(100, 50.0)
	// notional = 100 * 50 = 5000; cash = 5000 / 4 = 1250
	expected := 1250.0
	if math.Abs(cashCost-expected) > 1e-6 {
		t.Errorf("leverage cash cost: got %.2f, want %.2f", cashCost, expected)
	}
}

func TestPerAssetCommission(t *testing.T) {
	broker := NewBroker(100_000)

	// Default: 0.1%
	broker.SetCommission(PercentCommission{Percent: 0.001})

	// Per-asset override for AAPL: $0.005 per share
	broker.SetCommissionForFeed("AAPL", FixedCommission{PerShare: 0.005})

	// Check default feed uses percent
	defaultComm := broker.commFor("MSFT")
	comm := defaultComm.GetCommission(100, 150.0)
	expectedDefault := 100 * 150.0 * 0.001
	if math.Abs(comm-expectedDefault) > 1e-9 {
		t.Errorf("default commission: got %.6f, want %.6f", comm, expectedDefault)
	}

	// Check AAPL uses fixed
	aaplComm := broker.commFor("AAPL")
	aaplCommVal := aaplComm.GetCommission(100, 200.0)
	expectedAAPL := 100 * 0.005
	if math.Abs(aaplCommVal-expectedAAPL) > 1e-9 {
		t.Errorf("AAPL commission: got %.6f, want %.6f", aaplCommVal, expectedAAPL)
	}
}
