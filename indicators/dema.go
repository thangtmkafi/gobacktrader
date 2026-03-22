package indicators

import (
	"fmt"
	"math"

	"github.com/thangtmkafi/gobacktrader/core"
)

// DEMA is a Double Exponential Moving Average.
// Formula: DEMA = 2×EMA(n) − EMA(EMA(n))
// This removes some of the lag inherent in a standard EMA.
type DEMA struct {
	line   *core.Line
	period int
}

// NewDEMA computes a DEMA of the given period over data.Close.
func NewDEMA(data *core.DataSeries, period int) *DEMA {
	n := data.Len()
	vals := make([]float64, n)
	for i := range vals {
		vals[i] = math.NaN()
	}

	ema1 := emaLine(data.Close, period)
	ema2 := emaLine(ema1, period)

	for i := 0; i < n; i++ {
		v1 := ema1.Get(-(n - 1 - i))
		v2 := ema2.Get(-(n - 1 - i))
		if math.IsNaN(v1) || math.IsNaN(v2) {
			continue
		}
		vals[i] = 2*v1 - v2
	}
	return &DEMA{line: lineFromSlice(vals), period: period}
}

func (d *DEMA) Line() *core.Line { return d.line }
func (d *DEMA) Name() string     { return fmt.Sprintf("DEMA(%d)", d.period) }
