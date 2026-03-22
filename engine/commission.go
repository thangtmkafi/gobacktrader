// Package engine contains the backtesting engine components.
// This file implements commission schemes, mirroring backtrader's comminfo.py.
package engine

// CommissionInfo defines the interface for computing trading commissions.
type CommissionInfo interface {
	// GetCommission returns the commission for a given trade (size × price).
	GetCommission(size, price float64) float64
}

// PercentCommission charges a percentage of the trade notional value.
// e.g. 0.001 = 0.1%
type PercentCommission struct {
	Percent float64 // e.g. 0.001 for 0.1%
}

func (c PercentCommission) GetCommission(size, price float64) float64 {
	if size < 0 {
		size = -size
	}
	return size * price * c.Percent
}

// FixedCommission charges a flat fee per share/contract traded.
// e.g. 0.01 = $0.01 per share.
type FixedCommission struct {
	PerShare float64
}

func (c FixedCommission) GetCommission(size, price float64) float64 {
	if size < 0 {
		size = -size
	}
	return size * c.PerShare
}

// ZeroCommission charges no commissions (useful for quick testing).
type ZeroCommission struct{}

func (ZeroCommission) GetCommission(_, _ float64) float64 { return 0 }
