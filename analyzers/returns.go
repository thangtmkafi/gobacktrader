package analyzers

import (
	"fmt"
	"math"

	"github.com/thangtmkafi/gobacktrader/core"
	"github.com/thangtmkafi/gobacktrader/engine"
)

// Returns computes overall return metrics for the backtest.
type Returns struct {
	TotalReturnPct float64 // (FinalValue - Start) / Start * 100
	StartValue     float64
	EndValue       float64
	PnL            float64

	BestTradePnL  float64 // highest net PnL of a single closed trade
	WorstTradePnL float64 // lowest  net PnL of a single closed trade
	AvgTradePnL   float64 // average net PnL across all closed trades
}

func (r *Returns) Name() string { return "Returns" }

func (r *Returns) Analyze(result *engine.RunResult, _ []*core.DataSeries) error {
	r.StartValue = result.StartingCash
	r.EndValue = result.FinalValue
	r.PnL = result.FinalValue - result.StartingCash
	if result.StartingCash != 0 {
		r.TotalReturnPct = r.PnL / result.StartingCash * 100
	}

	r.BestTradePnL = math.NaN()
	r.WorstTradePnL = math.NaN()
	closedCount := 0
	totalPnL := 0.0
	for _, t := range result.Trades {
		if t.IsOpen {
			continue
		}
		closedCount++
		totalPnL += t.PnLComm
		if math.IsNaN(r.BestTradePnL) || t.PnLComm > r.BestTradePnL {
			r.BestTradePnL = t.PnLComm
		}
		if math.IsNaN(r.WorstTradePnL) || t.PnLComm < r.WorstTradePnL {
			r.WorstTradePnL = t.PnLComm
		}
	}
	if closedCount > 0 {
		r.AvgTradePnL = totalPnL / float64(closedCount)
	}
	return nil
}

func (r *Returns) Print() {
	fmt.Println("=== Returns ===")
	fmt.Printf("  Starting Value : $%.2f\n", r.StartValue)
	fmt.Printf("  Final Value    : $%.2f\n", r.EndValue)
	fmt.Printf("  PnL            : $%.2f\n", r.PnL)
	fmt.Printf("  Total Return   : %.2f%%\n", r.TotalReturnPct)
	if !math.IsNaN(r.BestTradePnL) {
		fmt.Printf("  Best Trade     : $%.2f\n", r.BestTradePnL)
		fmt.Printf("  Worst Trade    : $%.2f\n", r.WorstTradePnL)
		fmt.Printf("  Avg Trade PnL  : $%.2f\n", r.AvgTradePnL)
	}
	fmt.Println()
}
