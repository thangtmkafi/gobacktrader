package analyzers

import (
	"fmt"
	"math"

	"github.com/thangtmkafi/gobacktrader/core"
	"github.com/thangtmkafi/gobacktrader/engine"
)

// SharpeRatio computes the annualised Sharpe Ratio from per-trade net returns.
//
//	Sharpe = (mean(R) - Rf) / stddev(R) * sqrt(AnnualisationFactor)
//
// where R is the set of per-trade net PnL values normalised by starting cash.
// RiskFreeRate is expressed as an annual decimal (e.g. 0.02 = 2%).
// AnnualisationFactor defaults to 252 (daily bars).
type SharpeRatio struct {
	// RiskFreeRate is the annual risk-free rate (default 0).
	RiskFreeRate float64
	// AnnualisationFactor: 252 for daily, 52 for weekly, 12 for monthly.
	AnnualisationFactor float64

	Value float64 // computed Sharpe Ratio (NaN if fewer than 2 trades)
}

func (s *SharpeRatio) Name() string { return "SharpeRatio" }

func (s *SharpeRatio) Analyze(result *engine.RunResult, _ []*core.DataSeries) error {
	af := s.AnnualisationFactor
	if af == 0 {
		af = 252
	}

	// Collect closed-trade net returns as a fraction of starting cash
	var returns []float64
	for _, t := range result.Trades {
		if t.IsOpen {
			continue
		}
		if result.StartingCash != 0 {
			returns = append(returns, t.PnLComm/result.StartingCash)
		}
	}

	n := len(returns)
	if n < 2 {
		s.Value = math.NaN()
		return nil
	}

	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(n)

	// Per-period risk-free rate
	rfPer := s.RiskFreeRate / af
	excessMean := mean - rfPer

	variance := 0.0
	for _, r := range returns {
		d := r - mean
		variance += d * d
	}
	variance /= float64(n - 1) // sample stddev

	stddev := math.Sqrt(variance)
	if stddev < 1e-12 {
		s.Value = math.NaN()
		return nil
	}

	s.Value = excessMean / stddev * math.Sqrt(af)
	return nil
}

func (s *SharpeRatio) Print() {
	fmt.Println("=== Sharpe Ratio ===")
	if math.IsNaN(s.Value) {
		fmt.Println("  N/A (need at least 2 closed trades)")
	} else {
		fmt.Printf("  Sharpe Ratio : %.4f\n", s.Value)
	}
	fmt.Println()
}
