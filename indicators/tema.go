package indicators

import (
	"fmt"
	"math"

	"github.com/thangtmkafi/gobacktrader/core"
)

// TEMA is a Triple Exponential Moving Average.
// Formula: TEMA = 3×EMA(n) − 3×EMA(EMA(n)) + EMA(EMA(EMA(n)))
// Further reduces lag compared to EMA or DEMA.
type TEMA struct {
	line   *core.Line
	period int
}

// NewTEMA computes a TEMA of the given period over data.Close.
func NewTEMA(data *core.DataSeries, period int) *TEMA {
	n := data.Len()
	vals := make([]float64, n)
	for i := range vals {
		vals[i] = math.NaN()
	}

	ema1 := emaLine(data.Close, period)
	ema2 := emaLine(ema1, period)
	ema3 := emaLine(ema2, period)

	for i := 0; i < n; i++ {
		v1 := ema1.Get(-(n - 1 - i))
		v2 := ema2.Get(-(n - 1 - i))
		v3 := ema3.Get(-(n - 1 - i))
		if math.IsNaN(v1) || math.IsNaN(v2) || math.IsNaN(v3) {
			continue
		}
		vals[i] = 3*v1 - 3*v2 + v3
	}
	return &TEMA{line: lineFromSlice(vals), period: period}
}

func (t *TEMA) Line() *core.Line { return t.line }
func (t *TEMA) Name() string     { return fmt.Sprintf("TEMA(%d)", t.period) }
