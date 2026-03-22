// Package indicators provides technical analysis indicators for gobacktrader.
//
// Design philosophy
//
// Each indicator pre-computes its full output series during construction
// (via a New* factory function). This trades a small amount of upfront memory
// for zero per-bar overhead during the strategy loop.
//
// Strategies attach indicators in Init() and read values bar-by-bar in Next():
//
//	func (s *MyStrategy) Init(ctx *StrategyContext) {
//	    s.sma = indicators.NewSMA(ctx.Data(), 20)
//	}
//
//	func (s *MyStrategy) Next() {
//	    current := s.sma.Line().Get(0)
//	    prev    := s.sma.Line().Get(-1)
//	}
//
// All indicators pad the first (period-1) values with NaN so indices stay
// aligned with the underlying DataSeries cursor.
package indicators

import "github.com/thangtmkafi/gobacktrader/core"

// Indicator is the common interface implemented by all indicators.
type Indicator interface {
	// Line returns the primary output line (e.g. the SMA values).
	Line() *core.Line
	// Name returns a human-readable label (e.g. "SMA(20)").
	Name() string
}

// lineFromSlice writes a []float64 slice into a freshly-built *core.Line.
// NaNs in the slice are written as-is (Forward/Set handles them naturally).
func lineFromSlice(vals []float64) *core.Line {
	l := core.NewLine()
	for _, v := range vals {
		l.Forward()
		l.Set(v)
	}
	return l
}
