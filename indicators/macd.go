package indicators

import (
	"fmt"
	"math"

	"github.com/thangtmkafi/gobacktrader/core"
)

// MACD is the Moving Average Convergence/Divergence indicator.
// It exposes three lines: MACDLine (fast EMA − slow EMA),
// SignalLine (EMA of MACDLine), and Histogram (MACDLine − SignalLine).
type MACD struct {
	macdLine   *core.Line
	signalLine *core.Line
	histogram  *core.Line
	fastPer    int
	slowPer    int
	signalPer  int
}

// NewMACD computes MACD with the given fast/slow/signal periods.
// Standard defaults: fast=12, slow=26, signal=9.
func NewMACD(data *core.DataSeries, fastPer, slowPer, signalPer int) *MACD {
	n := data.Len()

	// Build fast and slow EMA lines
	fastEMA := emaLine(data.Close, fastPer)
	slowEMA := emaLine(data.Close, slowPer)

	// MACD line = fast EMA − slow EMA (NaN wherever either is NaN)
	macdVals := make([]float64, n)
	for i := 0; i < n; i++ {
		f := fastEMA.Get(-(n - 1 - i))
		s := slowEMA.Get(-(n - 1 - i))
		if math.IsNaN(f) || math.IsNaN(s) {
			macdVals[i] = math.NaN()
		} else {
			macdVals[i] = f - s
		}
	}
	macdRaw := lineFromSlice(macdVals)

	// Signal line = EMA(MACD, signalPer)
	signalRaw := emaLine(macdRaw, signalPer)

	// Histogram = MACD − Signal
	histVals := make([]float64, n)
	for i := 0; i < n; i++ {
		m := macdRaw.Get(-(n - 1 - i))
		s := signalRaw.Get(-(n - 1 - i))
		if math.IsNaN(m) || math.IsNaN(s) {
			histVals[i] = math.NaN()
		} else {
			histVals[i] = m - s
		}
	}

	return &MACD{
		macdLine:   macdRaw,
		signalLine: signalRaw,
		histogram:  lineFromSlice(histVals),
		fastPer:    fastPer,
		slowPer:    slowPer,
		signalPer:  signalPer,
	}
}

// Line returns the primary MACD line (fast EMA − slow EMA).
func (m *MACD) Line() *core.Line { return m.macdLine }

// Signal returns the signal line (EMA of MACD).
func (m *MACD) Signal() *core.Line { return m.signalLine }

// Histogram returns the histogram (MACD − Signal).
func (m *MACD) Histogram() *core.Line { return m.histogram }

func (m *MACD) Name() string {
	return fmt.Sprintf("MACD(%d,%d,%d)", m.fastPer, m.slowPer, m.signalPer)
}
