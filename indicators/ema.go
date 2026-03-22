package indicators

import (
	"fmt"
	"math"

	"github.com/thangtmkafi/gobacktrader/core"
)

// EMA is an Exponential Moving Average indicator.
// Uses multiplier k = 2/(period+1) (standard EMA formula).
// Seed: first EMA value is the SMA of the first `period` closes.
type EMA struct {
	line   *core.Line
	period int
}

// NewEMA computes an EMA of the given period over data.Close.
func NewEMA(data *core.DataSeries, period int) *EMA {
	return &EMA{line: emaLine(data.Close, period), period: period}
}

// NewEMAOnLine computes an EMA over any *core.Line.
func NewEMAOnLine(src *core.Line, period int) *EMA {
	return &EMA{line: emaLine(src, period), period: period}
}

// emaLine is the internal EMA computation shared by EMA, DEMA, TEMA.
// It handles a src that may have NaN-prefixed values (e.g. output of a previous
// emaLine call) by finding the first valid index before seeding.
func emaLine(src *core.Line, period int) *core.Line {
	n := src.Len()
	vals := make([]float64, n)
	for i := range vals {
		vals[i] = math.NaN()
	}
	if n < period {
		return lineFromSlice(vals)
	}

	// Find the first non-NaN index in src (handles chained EMA inputs).
	firstValid := -1
	for i := 0; i < n; i++ {
		if !math.IsNaN(src.Get(-(n - 1 - i))) {
			firstValid = i
			break
		}
	}
	if firstValid < 0 || firstValid+period > n {
		return lineFromSlice(vals) // not enough data
	}

	k := 2.0 / float64(period+1)

	// Seed: SMA of `period` consecutive non-NaN values starting at firstValid.
	seed := 0.0
	for j := 0; j < period; j++ {
		seed += src.Get(-(n - 1 - (firstValid + j)))
	}
	seed /= float64(period)

	seedIdx := firstValid + period - 1
	vals[seedIdx] = seed
	prev := seed
	for i := seedIdx + 1; i < n; i++ {
		v := src.Get(-(n - 1 - i))
		if math.IsNaN(v) {
			vals[i] = math.NaN()
			continue
		}
		prev = v*k + prev*(1-k)
		vals[i] = prev
	}
	return lineFromSlice(vals)
}

func (e *EMA) Line() *core.Line { return e.line }
func (e *EMA) Name() string     { return fmt.Sprintf("EMA(%d)", e.period) }
