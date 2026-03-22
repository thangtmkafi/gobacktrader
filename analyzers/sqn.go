package analyzers

import (
	"fmt"
	"math"

	"github.com/thangtmkafi/gobacktrader/core"
	"github.com/thangtmkafi/gobacktrader/engine"
)

// SQN is the System Quality Number as defined by Van K. Tharp.
//
//	SQN = mean(R) / stddev(R) * sqrt(N)
//
// where R = per-trade net PnL / risk (here we use per-trade net PnL / starting cash).
// Grades:
//
//	< 1.6   = Difficult to trade
//	1.6-1.9 = Below average
//	2.0-2.4 = Average
//	2.5-2.9 = Good
//	3.0-5.0 = Excellent
//	> 5.0   = Holy Grail
type SQN struct {
	Value float64
	Grade string
	N     int // number of closed trades used
}

func (s *SQN) Name() string { return "SQN" }

func (s *SQN) Analyze(result *engine.RunResult, _ []*core.DataSeries) error {
	var rs []float64
	for _, t := range result.Trades {
		if t.IsOpen {
			continue
		}
		if result.StartingCash != 0 {
			rs = append(rs, t.PnLComm/result.StartingCash)
		}
	}
	s.N = len(rs)
	if s.N < 2 {
		s.Value = math.NaN()
		s.Grade = "N/A"
		return nil
	}

	mean := 0.0
	for _, r := range rs {
		mean += r
	}
	mean /= float64(s.N)

	variance := 0.0
	for _, r := range rs {
		d := r - mean
		variance += d * d
	}
	variance /= float64(s.N - 1)
	stddev := math.Sqrt(variance)
	if stddev < 1e-12 {
		s.Value = math.NaN()
		s.Grade = "N/A"
		return nil
	}

	s.Value = mean / stddev * math.Sqrt(float64(s.N))
	s.Grade = sqnGrade(s.Value)
	return nil
}

func sqnGrade(v float64) string {
	switch {
	case v >= 5.0:
		return "Holy Grail"
	case v >= 3.0:
		return "Excellent"
	case v >= 2.5:
		return "Good"
	case v >= 2.0:
		return "Average"
	case v >= 1.6:
		return "Below Average"
	default:
		return "Difficult to Trade"
	}
}

func (s *SQN) Print() {
	fmt.Println("=== SQN (System Quality Number) ===")
	if math.IsNaN(s.Value) {
		fmt.Printf("  N/A (need at least 2 closed trades, got %d)\n", s.N)
	} else {
		fmt.Printf("  SQN   : %.4f\n", s.Value)
		fmt.Printf("  Grade : %s\n", s.Grade)
		fmt.Printf("  Trades: %d\n", s.N)
	}
	fmt.Println()
}
