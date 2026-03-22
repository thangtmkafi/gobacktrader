package indicators

import (
	"math"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/thangtmkafi/gobacktrader/feeds"
)

// loadTestData loads the 50-bar sample CSV into a DataSeries ready to query.
func loadTestData(t *testing.T) *feeds.CSVFeed {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..")
	cfg := feeds.DefaultYahooConfig(filepath.Join(root, "testdata", "sample.csv"))
	feed := feeds.NewCSVFeed(cfg)
	if err := feed.Load(); err != nil {
		t.Fatalf("load test data: %v", err)
	}
	// Advance all bars so the DataSeries is fully populated
	for feed.Next() {
	}
	return feed
}

// ─── SMA ───────────────────────────────────────────────────────────────────

func TestSMA(t *testing.T) {
	feed := loadTestData(t)
	data := feed.Data()
	period := 5

	sma := NewSMA(data, period)

	n := data.Len()
	if n != 50 {
		t.Fatalf("expected 50 bars, got %d", n)
	}

	// First (period-1) bars must be NaN
	for i := 0; i < period-1; i++ {
		v := sma.Line().Get(-(n - 1 - i))
		if !math.IsNaN(v) {
			t.Errorf("SMA[%d] should be NaN, got %f", i, v)
		}
	}

	// Bar at index (period-1) must be valid
	firstValid := sma.Line().Get(-(n - 1 - (period - 1)))
	if math.IsNaN(firstValid) {
		t.Error("SMA first valid bar should not be NaN")
	}

	// Hand-check: SMA(5) at index 4 = average of closes[0..4]
	sum := 0.0
	for j := 0; j < period; j++ {
		sum += data.Close.Get(-(n - 1 - j))
	}
	expected := sum / float64(period)
	if math.Abs(firstValid-expected) > 1e-6 {
		t.Errorf("SMA(5)[4] = %f, expected %f", firstValid, expected)
	}

	// Last bar should be valid
	last := sma.Line().Get(0)
	if math.IsNaN(last) {
		t.Error("SMA last bar should not be NaN")
	}

	// Name
	if sma.Name() != "SMA(5)" {
		t.Errorf("Name: got %s", sma.Name())
	}
}

// ─── EMA ───────────────────────────────────────────────────────────────────

func TestEMA(t *testing.T) {
	feed := loadTestData(t)
	data := feed.Data()
	period := 10
	ema := NewEMA(data, period)
	n := data.Len()

	// First (period-1) bars NaN
	for i := 0; i < period-1; i++ {
		v := ema.Line().Get(-(n - 1 - i))
		if !math.IsNaN(v) {
			t.Errorf("EMA[%d] should be NaN, got %f", i, v)
		}
	}

	// Bar at (period-1): seed = SMA of first period closes
	seed := 0.0
	for j := 0; j < period; j++ {
		seed += data.Close.Get(-(n - 1 - j))
	}
	seed /= float64(period)
	got := ema.Line().Get(-(n - 1 - (period - 1)))
	if math.Abs(got-seed) > 1e-6 {
		t.Errorf("EMA seed = %f, expected %f", got, seed)
	}

	// Last bar should be finite and positive
	last := ema.Line().Get(0)
	if math.IsNaN(last) || last <= 0 {
		t.Errorf("EMA last bar invalid: %f", last)
	}
}

// ─── WMA ───────────────────────────────────────────────────────────────────

func TestWMA(t *testing.T) {
	feed := loadTestData(t)
	data := feed.Data()
	period := 5
	wma := NewWMA(data, period)
	n := data.Len()

	// NaN prefix
	for i := 0; i < period-1; i++ {
		if !math.IsNaN(wma.Line().Get(-(n - 1 - i))) {
			t.Errorf("WMA[%d] should be NaN", i)
		}
	}

	// Hand-check first valid bar: weights 1,2,3,4,5 / 15
	denom := float64(period * (period + 1) / 2)
	expected := 0.0
	for j := 0; j < period; j++ {
		expected += float64(j+1) * data.Close.Get(-(n-1-(period-1))+(j-period+1))
	}
	expected /= denom
	got := wma.Line().Get(-(n - 1 - (period - 1)))
	if math.Abs(got-expected) > 1e-6 {
		t.Errorf("WMA first valid = %f, expected %f", got, expected)
	}
}

// ─── DEMA / TEMA ──────────────────────────────────────────────────────────

func TestDEMA(t *testing.T) {
	feed := loadTestData(t)
	data := feed.Data()
	dema := NewDEMA(data, 5)
	n := data.Len()

	// Should have at least some valid bars
	validCount := 0
	for i := 0; i < n; i++ {
		if !math.IsNaN(dema.Line().Get(-(n - 1 - i))) {
			validCount++
		}
	}
	if validCount == 0 {
		t.Error("DEMA produced no valid values")
	}
	if dema.Name() != "DEMA(5)" {
		t.Errorf("Name: %s", dema.Name())
	}
}

func TestTEMA(t *testing.T) {
	feed := loadTestData(t)
	data := feed.Data()
	tema := NewTEMA(data, 5)
	n := data.Len()

	validCount := 0
	for i := 0; i < n; i++ {
		if !math.IsNaN(tema.Line().Get(-(n - 1 - i))) {
			validCount++
		}
	}
	if validCount == 0 {
		t.Error("TEMA produced no valid values")
	}
}

// ─── RSI ───────────────────────────────────────────────────────────────────

func TestRSI(t *testing.T) {
	feed := loadTestData(t)
	data := feed.Data()
	period := 14
	rsi := NewRSI(data, period)
	n := data.Len()

	// First `period` bars must be NaN
	for i := 0; i < period; i++ {
		v := rsi.Line().Get(-(n - 1 - i))
		if !math.IsNaN(v) {
			t.Errorf("RSI[%d] should be NaN, got %f", i, v)
		}
	}

	// All valid values must be in [0, 100]
	for i := period; i < n; i++ {
		v := rsi.Line().Get(-(n - 1 - i))
		if math.IsNaN(v) {
			t.Errorf("RSI[%d] is NaN (expected valid)", i)
			continue
		}
		if v < 0 || v > 100 {
			t.Errorf("RSI[%d] = %f, out of [0,100]", i, v)
		}
	}

	if rsi.Name() != "RSI(14)" {
		t.Errorf("Name: %s", rsi.Name())
	}
}

// ─── MACD ──────────────────────────────────────────────────────────────────

func TestMACD(t *testing.T) {
	feed := loadTestData(t)
	data := feed.Data()
	macd := NewMACD(data, 5, 10, 3) // shorter periods for 50-bar dataset
	n := data.Len()

	// Verify histogram = MACD − Signal wherever both are valid
	for i := 0; i < n; i++ {
		ago := -(n - 1 - i)
		m := macd.Line().Get(ago)
		s := macd.Signal().Get(ago)
		h := macd.Histogram().Get(ago)
		if math.IsNaN(m) || math.IsNaN(s) {
			if !math.IsNaN(h) {
				t.Errorf("MACD histogram[%d] should be NaN when MACD/Signal are NaN", i)
			}
			continue
		}
		if math.Abs(h-(m-s)) > 1e-9 {
			t.Errorf("MACD histogram[%d] = %f, expected %f", i, h, m-s)
		}
	}

	// At least some valid bars
	last := macd.Line().Get(0)
	if math.IsNaN(last) {
		t.Error("MACD last bar should be valid")
	}
}

// ─── Stochastic ────────────────────────────────────────────────────────────

func TestStochastic(t *testing.T) {
	feed := loadTestData(t)
	data := feed.Data()
	stoch := NewStochastic(data, 14, 3)
	n := data.Len()

	validK := 0
	for i := 0; i < n; i++ {
		k := stoch.K().Get(-(n - 1 - i))
		if math.IsNaN(k) {
			continue
		}
		validK++
		if k < 0 || k > 100 {
			t.Errorf("Stoch %%K[%d] = %f out of [0,100]", i, k)
		}
	}
	if validK == 0 {
		t.Error("Stochastic %K produced no valid values")
	}

	// D should exist and be a smoothed K
	last := stoch.D().Get(0)
	if math.IsNaN(last) {
		t.Error("Stochastic %D last bar should be valid (50 bars > period 14+3)")
	}
}

// ─── ATR ───────────────────────────────────────────────────────────────────

func TestATR(t *testing.T) {
	feed := loadTestData(t)
	data := feed.Data()
	period := 14
	atr := NewATR(data, period)
	n := data.Len()

	// First (period-1) bars NaN
	for i := 0; i < period-1; i++ {
		v := atr.Line().Get(-(n - 1 - i))
		if !math.IsNaN(v) {
			t.Errorf("ATR[%d] should be NaN, got %f", i, v)
		}
	}

	// All valid ATR values must be >= 0
	for i := period - 1; i < n; i++ {
		v := atr.Line().Get(-(n - 1 - i))
		if math.IsNaN(v) {
			t.Errorf("ATR[%d] is unexpectedly NaN", i)
			continue
		}
		if v < 0 {
			t.Errorf("ATR[%d] = %f, should be >= 0", i, v)
		}
	}

	// True Range first bar = High-Low
	bar0 := data.Bar()
	_ = bar0
	tr0 := atr.TrueRange().Get(-(n - 1))
	h0 := data.High.Get(-(n - 1))
	l0 := data.Low.Get(-(n - 1))
	if math.Abs(tr0-(h0-l0)) > 1e-6 {
		t.Errorf("TR[0] = %f, expected High-Low = %f", tr0, h0-l0)
	}
}

// ─── Bollinger Bands ───────────────────────────────────────────────────────

func TestBollingerBands(t *testing.T) {
	feed := loadTestData(t)
	data := feed.Data()
	period := 10
	bb := NewBollingerBands(data, period, 2.0)
	n := data.Len()

	// First (period-1) bars NaN
	for i := 0; i < period-1; i++ {
		if !math.IsNaN(bb.Mid().Get(-(n - 1 - i))) {
			t.Errorf("BB Mid[%d] should be NaN", i)
		}
	}

	// For all valid bars: lower <= mid <= upper
	for i := period - 1; i < n; i++ {
		ago := -(n - 1 - i)
		mid := bb.Mid().Get(ago)
		upper := bb.Upper().Get(ago)
		lower := bb.Lower().Get(ago)
		if math.IsNaN(mid) {
			t.Errorf("BB Mid[%d] is NaN", i)
			continue
		}
		if lower > mid+1e-9 {
			t.Errorf("BB[%d]: lower(%f) > mid(%f)", i, lower, mid)
		}
		if upper < mid-1e-9 {
			t.Errorf("BB[%d]: upper(%f) < mid(%f)", i, upper, mid)
		}
	}

	// BandWidth should be positive
	bw := bb.BandWidth(0)
	if math.IsNaN(bw) || bw < 0 {
		t.Errorf("BandWidth = %f, expected positive value", bw)
	}

	// %B for a close at mid should be ~0.5
	mid := bb.Mid().Get(0)
	pb := bb.PercentB(mid, 0)
	if math.Abs(pb-0.5) > 1e-9 {
		t.Errorf("%%B at mid price should be 0.5, got %f", pb)
	}

	if bb.Name() != "BB(10,2.0)" {
		t.Errorf("Name: %s", bb.Name())
	}
}
