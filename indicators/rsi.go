package indicators

import (
	"fmt"
	"math"

	"github.com/thangtmkafi/gobacktrader/core"
)

// RSI is the Relative Strength Index.
// Uses Wilder's smoothing (equivalent to an EMA with alpha = 1/period).
// Output is in [0, 100]. First (period) bars are NaN.
type RSI struct {
	line   *core.Line
	period int
}

// NewRSI computes the RSI of the given period over data.Close.
func NewRSI(data *core.DataSeries, period int) *RSI {
	n := data.Len()
	vals := make([]float64, n)
	for i := range vals {
		vals[i] = math.NaN()
	}
	if n <= period {
		return &RSI{line: lineFromSlice(vals), period: period}
	}

	// Collect close values (oldest first), indexed 0..n-1
	closes := make([]float64, n)
	for i := 0; i < n; i++ {
		closes[i] = data.Close.Get(-(n - 1 - i))
	}

	// Seed: average gain / average loss over first `period` changes
	var avgGain, avgLoss float64
	for i := 1; i <= period; i++ {
		diff := closes[i] - closes[i-1]
		if diff > 0 {
			avgGain += diff
		} else {
			avgLoss -= diff // make positive
		}
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	emitRSI := func(ag, al float64) float64 {
		if al == 0 {
			return 100
		}
		rs := ag / al
		return 100 - 100/(1+rs)
	}

	vals[period] = emitRSI(avgGain, avgLoss)

	// Wilder smoothing for remaining bars
	alpha := 1.0 / float64(period)
	for i := period + 1; i < n; i++ {
		diff := closes[i] - closes[i-1]
		gain, loss := 0.0, 0.0
		if diff > 0 {
			gain = diff
		} else {
			loss = -diff
		}
		avgGain = alpha*gain + (1-alpha)*avgGain
		avgLoss = alpha*loss + (1-alpha)*avgLoss
		vals[i] = emitRSI(avgGain, avgLoss)
	}

	return &RSI{line: lineFromSlice(vals), period: period}
}

func (r *RSI) Line() *core.Line { return r.line }
func (r *RSI) Name() string     { return fmt.Sprintf("RSI(%d)", r.period) }
