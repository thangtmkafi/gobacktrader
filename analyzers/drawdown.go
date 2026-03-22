package analyzers

import (
	"fmt"
	"math"

	"github.com/thangtmkafi/gobacktrader/core"
	"github.com/thangtmkafi/gobacktrader/engine"
)

// DrawDown analyses the equity curve to find the maximum peak-to-trough drawdown.
type DrawDown struct {
	MaxDrawdownPct float64 // peak-to-trough drop as a percentage of the peak
	MaxDrawdownAbs float64 // absolute dollar drawdown
	MaxDDStart     int     // bar index where the peak was set
	MaxDDEnd       int     // bar index where the trough was first reached
	MaxDDDuration  int     // duration in bars of the max drawdown period
	// Current (last) drawdown at end of backtest
	CurrentDrawdownPct float64
	CurrentDrawdownAbs float64
}

func (d *DrawDown) Name() string { return "DrawDown" }

func (d *DrawDown) Analyze(result *engine.RunResult, _ []*core.DataSeries) error {
	curve := result.EquityCurve
	if len(curve) == 0 {
		return nil
	}

	peak := curve[0]
	peakIdx := 0
	maxDD := 0.0
	maxDDPct := 0.0
	maxDDStart := 0
	maxDDEnd := 0

	for i, v := range curve {
		if v > peak {
			peak = v
			peakIdx = i
		}
		dd := peak - v
		if dd > maxDD {
			maxDD = dd
			maxDDStart = peakIdx
			maxDDEnd = i
			if peak != 0 {
				maxDDPct = dd / peak * 100
			}
		}
	}

	d.MaxDrawdownAbs = maxDD
	d.MaxDrawdownPct = maxDDPct
	d.MaxDDStart = maxDDStart
	d.MaxDDEnd = maxDDEnd
	d.MaxDDDuration = maxDDEnd - maxDDStart

	// Current drawdown at end of backtest
	last := curve[len(curve)-1]
	// Find the running peak up to last bar
	runPeak := curve[0]
	for _, v := range curve {
		if v > runPeak {
			runPeak = v
		}
	}
	d.CurrentDrawdownAbs = math.Max(0, runPeak-last)
	if runPeak != 0 {
		d.CurrentDrawdownPct = d.CurrentDrawdownAbs / runPeak * 100
	}
	return nil
}

func (d *DrawDown) Print() {
	fmt.Println("=== DrawDown ===")
	fmt.Printf("  Max Drawdown       : $%.2f  (%.2f%%)\n",
		d.MaxDrawdownAbs, d.MaxDrawdownPct)
	fmt.Printf("  Max DD Duration    : %d bars  (bars %d → %d)\n",
		d.MaxDDDuration, d.MaxDDStart, d.MaxDDEnd)
	fmt.Printf("  Current Drawdown   : $%.2f  (%.2f%%)\n",
		d.CurrentDrawdownAbs, d.CurrentDrawdownPct)
	fmt.Println()
}
