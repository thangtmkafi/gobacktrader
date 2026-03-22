package indicators

import (
	"fmt"
	"math"

	"github.com/thangtmkafi/gobacktrader/core"
)

// BollingerBands computes Bollinger Bands around a simple moving average.
//
//	Mid   = SMA(Close, period)
//	Upper = Mid + k × StdDev(Close, period)
//	Lower = Mid − k × StdDev(Close, period)
//
// Standard defaults: period=20, k=2.0.
type BollingerBands struct {
	mid    *core.Line
	upper  *core.Line
	lower  *core.Line
	period int
	k      float64
}

// NewBollingerBands computes Bollinger Bands with the given period and multiplier k.
func NewBollingerBands(data *core.DataSeries, period int, k float64) *BollingerBands {
	n := data.Len()
	midVals := make([]float64, n)
	upperVals := make([]float64, n)
	lowerVals := make([]float64, n)
	for i := range midVals {
		midVals[i] = math.NaN()
		upperVals[i] = math.NaN()
		lowerVals[i] = math.NaN()
	}

	for i := period - 1; i < n; i++ {
		ago := -(n - 1 - i)
		// Mean
		sum := 0.0
		for j := 0; j < period; j++ {
			sum += data.Close.Get(ago - j)
		}
		mean := sum / float64(period)

		// Population standard deviation
		variance := 0.0
		for j := 0; j < period; j++ {
			diff := data.Close.Get(ago-j) - mean
			variance += diff * diff
		}
		stddev := math.Sqrt(variance / float64(period))

		midVals[i] = mean
		upperVals[i] = mean + k*stddev
		lowerVals[i] = mean - k*stddev
	}

	return &BollingerBands{
		mid:    lineFromSlice(midVals),
		upper:  lineFromSlice(upperVals),
		lower:  lineFromSlice(lowerVals),
		period: period,
		k:      k,
	}
}

// Line returns the middle band (SMA).
func (b *BollingerBands) Line() *core.Line { return b.mid }

// Mid returns the middle band.
func (b *BollingerBands) Mid() *core.Line { return b.mid }

// Upper returns the upper band.
func (b *BollingerBands) Upper() *core.Line { return b.upper }

// Lower returns the lower band.
func (b *BollingerBands) Lower() *core.Line { return b.lower }

// BandWidth returns (Upper - Lower) / Mid × 100 for the current bar.
// This is a convenience method, not a pre-computed Line.
func (b *BollingerBands) BandWidth(ago int) float64 {
	mid := b.mid.Get(ago)
	if math.IsNaN(mid) || mid == 0 {
		return math.NaN()
	}
	return (b.upper.Get(ago) - b.lower.Get(ago)) / mid * 100
}

// PercentB returns %B for the given bar: (Close - Lower) / (Upper - Lower).
func (b *BollingerBands) PercentB(close float64, ago int) float64 {
	upper := b.upper.Get(ago)
	lower := b.lower.Get(ago)
	rng := upper - lower
	if math.IsNaN(rng) || rng < 1e-10 {
		return math.NaN()
	}
	return (close - lower) / rng
}

func (b *BollingerBands) Name() string {
	return fmt.Sprintf("BB(%d,%.1f)", b.period, b.k)
}
