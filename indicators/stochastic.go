package indicators

import (
	"fmt"
	"math"

	"github.com/thangtmkafi/gobacktrader/core"
)

// Stochastic computes the Stochastic Oscillator with %K and %D lines.
//
//	%K = (Close − Lowest Low[kPeriod]) / (Highest High[kPeriod] − Lowest Low[kPeriod]) × 100
//	%D = SMA(%K, dPeriod)
//
// Standard defaults: kPeriod=14, dPeriod=3.
type Stochastic struct {
	k      *core.Line
	d      *core.Line
	kPer   int
	dPer   int
}

// NewStochastic computes the stochastic oscillator.
func NewStochastic(data *core.DataSeries, kPeriod, dPeriod int) *Stochastic {
	n := data.Len()
	kVals := make([]float64, n)
	for i := range kVals {
		kVals[i] = math.NaN()
	}

	for i := kPeriod - 1; i < n; i++ {
		lowestLow := math.Inf(1)
		highestHigh := math.Inf(-1)
		ago := -(n - 1 - i)
		for j := 0; j < kPeriod; j++ {
			l := data.Low.Get(ago - j)
			h := data.High.Get(ago - j)
			if l < lowestLow {
				lowestLow = l
			}
			if h > highestHigh {
				highestHigh = h
			}
		}
		rng := highestHigh - lowestLow
		if rng < 1e-10 {
			kVals[i] = 50 // avoid division by zero
		} else {
			k := (data.Close.Get(ago) - lowestLow) / rng * 100
			// Clamp to [0,100]: can exceed bounds with adjusted-price data
			if k < 0 {
				k = 0
			} else if k > 100 {
				k = 100
			}
			kVals[i] = k
		}
	}

	kLine := lineFromSlice(kVals)

	// %D = SMA of %K over dPeriod
	dVals := make([]float64, n)
	for i := range dVals {
		dVals[i] = math.NaN()
	}
	for i := kPeriod - 1 + dPeriod - 1; i < n; i++ {
		sum := 0.0
		valid := true
		for j := 0; j < dPeriod; j++ {
			v := kLine.Get(-(n - 1 - i) - j)
			if math.IsNaN(v) {
				valid = false
				break
			}
			sum += v
		}
		if valid {
			dVals[i] = sum / float64(dPeriod)
		}
	}

	return &Stochastic{
		k:    kLine,
		d:    lineFromSlice(dVals),
		kPer: kPeriod,
		dPer: dPeriod,
	}
}

// Line returns the %K line (primary output).
func (s *Stochastic) Line() *core.Line { return s.k }

// K returns the %K line.
func (s *Stochastic) K() *core.Line { return s.k }

// D returns the %D line (signal).
func (s *Stochastic) D() *core.Line { return s.d }

func (s *Stochastic) Name() string {
	return fmt.Sprintf("Stoch(%d,%d)", s.kPer, s.dPer)
}
