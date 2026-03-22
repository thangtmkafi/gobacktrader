package analyzers

import (
	"fmt"
	"math"

	"github.com/thangtmkafi/gobacktrader/core"
	"github.com/thangtmkafi/gobacktrader/engine"
)

// TradeAnalyzer computes detailed statistics about closed trades.
type TradeAnalyzer struct {
	Total  int
	Won    int
	Lost   int
	Open   int
	Even   int // PnL == 0

	WinRatePct float64

	GrossProfit float64
	GrossLoss   float64
	ProfitFactor float64 // GrossProfit / |GrossLoss|; Inf if no losses

	AvgWinPnL  float64
	AvgLossPnL float64

	MaxConsecWins   int
	MaxConsecLosses int

	LargestWin  float64
	LargestLoss float64
}

func (t *TradeAnalyzer) Name() string { return "TradeAnalyzer" }

func (t *TradeAnalyzer) Analyze(result *engine.RunResult, _ []*core.DataSeries) error {
	t.LargestWin = math.NaN()
	t.LargestLoss = math.NaN()

	consecWins, consecLosses := 0, 0
	for _, tr := range result.Trades {
		if tr.IsOpen {
			t.Open++
			continue
		}
		t.Total++
		pnl := tr.PnLComm
		switch {
		case pnl > 0:
			t.Won++
			t.GrossProfit += pnl
			consecWins++
			consecLosses = 0
			if math.IsNaN(t.LargestWin) || pnl > t.LargestWin {
				t.LargestWin = pnl
			}
		case pnl < 0:
			t.Lost++
			t.GrossLoss += pnl // negative
			consecLosses++
			consecWins = 0
			if math.IsNaN(t.LargestLoss) || pnl < t.LargestLoss {
				t.LargestLoss = pnl
			}
		default:
			t.Even++
			consecWins = 0
			consecLosses = 0
		}
		if consecWins > t.MaxConsecWins {
			t.MaxConsecWins = consecWins
		}
		if consecLosses > t.MaxConsecLosses {
			t.MaxConsecLosses = consecLosses
		}
	}

	if t.Total > 0 {
		t.WinRatePct = float64(t.Won) / float64(t.Total) * 100
	}
	if t.Won > 0 {
		t.AvgWinPnL = t.GrossProfit / float64(t.Won)
	}
	if t.Lost > 0 {
		t.AvgLossPnL = t.GrossLoss / float64(t.Lost)
	}
	if t.GrossLoss == 0 {
		if t.GrossProfit > 0 {
			t.ProfitFactor = math.Inf(1)
		}
	} else {
		t.ProfitFactor = t.GrossProfit / math.Abs(t.GrossLoss)
	}
	return nil
}

func (t *TradeAnalyzer) Print() {
	fmt.Println("=== Trade Analyzer ===")
	fmt.Printf("  Total Trades   : %d  (Open: %d)\n", t.Total, t.Open)
	fmt.Printf("  Won / Lost     : %d / %d  (Win Rate: %.1f%%)\n",
		t.Won, t.Lost, t.WinRatePct)
	fmt.Printf("  Even           : %d\n", t.Even)
	fmt.Printf("  Gross Profit   : $%.2f\n", t.GrossProfit)
	fmt.Printf("  Gross Loss     : $%.2f\n", t.GrossLoss)
	if math.IsInf(t.ProfitFactor, 1) {
		fmt.Println("  Profit Factor  : ∞  (no losing trades)")
	} else {
		fmt.Printf("  Profit Factor  : %.2f\n", t.ProfitFactor)
	}
	if t.Won > 0 {
		fmt.Printf("  Avg Win PnL    : $%.2f\n", t.AvgWinPnL)
		fmt.Printf("  Largest Win    : $%.2f\n", t.LargestWin)
	}
	if t.Lost > 0 {
		fmt.Printf("  Avg Loss PnL   : $%.2f\n", t.AvgLossPnL)
		fmt.Printf("  Largest Loss   : $%.2f\n", t.LargestLoss)
	}
	fmt.Printf("  Max Consec Wins  : %d\n", t.MaxConsecWins)
	fmt.Printf("  Max Consec Loss  : %d\n", t.MaxConsecLosses)
	fmt.Println()
}
