package indicators

import (
	"fmt"
	"math"

	"github.com/thangtmkafi/gobacktrader/core"
)

// SMA is a Simple Moving Average indicator.
type SMA struct {
	line   *core.Line
	period int
}

// NewSMA computes a SMA of the given period over data.Close.
// The first (period-1) values are NaN.
func NewSMA(data *core.DataSeries, period int) *SMA {
	n := data.Len()
	vals := make([]float64, n)
	for i := range vals {
		vals[i] = math.NaN()
	}
	for i := period - 1; i < n; i++ {
		sum := 0.0
		for j := 0; j < period; j++ {
			sum += data.Close.Get(-(n - 1 - i) - j)
		}
		vals[i] = sum / float64(period)
	}
	return &SMA{line: lineFromSlice(vals), period: period}
}

// NewSMAOnLine computes a SMA over any *core.Line (not just Close).
func NewSMAOnLine(src *core.Line, period int) *SMA {
	n := src.Len()
	vals := make([]float64, n)
	for i := range vals {
		vals[i] = math.NaN()
	}
	// src.Cursor() == n-1 after all bars have been loaded
	cursor := src.Cursor()
	for i := period - 1; i < n; i++ {
		sum := 0.0
		ago := cursor - i // we'll iterate backwards
		for j := 0; j < period; j++ {
			sum += src.Get(ago - j + (n - 1 - cursor))
		}
		vals[i] = sum / float64(period)
	}
	return &SMA{line: lineFromSlice(vals), period: period}
}

func (s *SMA) Line() *core.Line { return s.line }
func (s *SMA) Name() string     { return fmt.Sprintf("SMA(%d)", s.period) }
