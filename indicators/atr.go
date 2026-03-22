package indicators

import (
	"fmt"
	"math"

	"github.com/thangtmkafi/gobacktrader/core"
)

// ATR is the Average True Range indicator (Wilder smoothing).
//
// True Range for bar i:
//
//	TR = max(High−Low, |High−prevClose|, |Low−prevClose|)
//
// ATR = Wilder EMA of TR (alpha = 1/period).
// First bar's TR = High−Low (no previous close). First ATR is the SMA of first
// `period` TRs; subsequent values use Wilder smoothing.
type ATR struct {
	line   *core.Line
	trLine *core.Line
	period int
}

// NewATR computes the ATR of the given period.
func NewATR(data *core.DataSeries, period int) *ATR {
	n := data.Len()
	trVals := make([]float64, n)
	atrVals := make([]float64, n)
	for i := range atrVals {
		atrVals[i] = math.NaN()
	}

	// Compute True Range for every bar
	for i := 0; i < n; i++ {
		ago := -(n - 1 - i)
		h := data.High.Get(ago)
		l := data.Low.Get(ago)
		if i == 0 {
			trVals[i] = h - l
		} else {
			pc := data.Close.Get(ago - 1)
			tr := math.Max(h-l, math.Max(math.Abs(h-pc), math.Abs(l-pc)))
			trVals[i] = tr
		}
	}

	if n < period {
		return &ATR{
			line:   lineFromSlice(atrVals),
			trLine: lineFromSlice(trVals),
			period: period,
		}
	}

	// Seed: SMA of first `period` TRs
	seed := 0.0
	for i := 0; i < period; i++ {
		seed += trVals[i]
	}
	seed /= float64(period)
	atrVals[period-1] = seed

	// Wilder smoothing: ATR = (prevATR*(period-1) + TR) / period
	prev := seed
	for i := period; i < n; i++ {
		prev = (prev*float64(period-1) + trVals[i]) / float64(period)
		atrVals[i] = prev
	}

	return &ATR{
		line:   lineFromSlice(atrVals),
		trLine: lineFromSlice(trVals),
		period: period,
	}
}

// Line returns the ATR line.
func (a *ATR) Line() *core.Line { return a.line }

// TrueRange returns the raw True Range line (useful for building other indicators).
func (a *ATR) TrueRange() *core.Line { return a.trLine }

func (a *ATR) Name() string { return fmt.Sprintf("ATR(%d)", a.period) }
