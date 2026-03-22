package indicators

import (
	"fmt"
	"math"

	"github.com/thangtmkafi/gobacktrader/core"
)

// WMA is a Weighted Moving Average (linearly weighted, recent bars get higher weight).
type WMA struct {
	line   *core.Line
	period int
}

// NewWMA computes a WMA of the given period over data.Close.
func NewWMA(data *core.DataSeries, period int) *WMA {
	n := data.Len()
	vals := make([]float64, n)
	for i := range vals {
		vals[i] = math.NaN()
	}
	// weights: 1, 2, ..., period  (oldest → newest)
	denom := float64(period * (period + 1) / 2)
	for i := period - 1; i < n; i++ {
		sum := 0.0
		for j := 0; j < period; j++ {
			// weight for oldest bar = 1, newest = period
			w := float64(j + 1)
			sum += w * data.Close.Get(-(n-1-i)-(period-1-j))
		}
		vals[i] = sum / denom
	}
	return &WMA{line: lineFromSlice(vals), period: period}
}

func (w *WMA) Line() *core.Line { return w.line }
func (w *WMA) Name() string     { return fmt.Sprintf("WMA(%d)", w.period) }
