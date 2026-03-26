// Package engine contains the backtesting engine components.
// This file implements commission schemes, mirroring backtrader's comminfo.py
// with full feature parity: percent/fixed, futures margin+multiplier, leverage,
// and interest charges on short positions.
package engine

import "math"

// ─── Commission types ─────────────────────────────────────────────────────────

// CommType distinguishes percentage vs fixed commissions.
type CommType int

const (
	CommTypePerc  CommType = iota // commission as a percentage of trade value
	CommTypeFixed                 // flat commission per share/contract
)

// ─── CommissionInfo interface (kept for backward compatibility) ───────────────

// CommissionInfo defines the minimal interface for commission computation.
// Existing PercentCommission, FixedCommission, and ZeroCommission implement this.
type CommissionInfo interface {
	GetCommission(size, price float64) float64
}

// ─── Thin helpers (kept for backward compat) ─────────────────────────────────

// PercentCommission charges a percentage of the trade notional value.
// e.g. 0.001 = 0.1%.
type PercentCommission struct {
	Percent float64
}

func (c PercentCommission) GetCommission(size, price float64) float64 {
	if size < 0 {
		size = -size
	}
	return size * price * c.Percent
}

// FixedCommission charges a flat fee per share/contract traded.
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

// ─── Rich CommInfo  (mirrors CommInfoBase from backtrader's comminfo.py) ──────

// CommInfo is a full-featured commission + margin + leverage descriptor.
// It mirrors Backtrader's CommInfoBase with all 10 parameters.
type CommInfo struct {
	// Commission rate
	Commission float64  // base rate; meaning depends on CommType
	CommType   CommType // CommTypePerc or CommTypeFixed
	PercAbs    bool     // if true, Commission is absolute (e.g. 0.002 = 0.2%); if false, interpret as XX%

	// Futures-style settings
	Margin     float64 // margin required per contract (0 = stock-like, no margin)
	Mult       float64 // contract multiplier (e.g. 50 for ES futures)
	AutoMargin float64 // if > 0, auto-calc margin = AutoMargin * price; if < 0, use Mult*price
	StockLike  bool    // true = stock behavior; false = futures behavior

	// Leverage
	Leverage float64 // asset leverage factor (1 = no leverage)

	// Interest on short positions
	Interest     float64 // yearly interest rate (0.05 = 5%)
	InterestLong bool    // if true, charge interest on long positions too
}

// NewStockCommInfo creates a CommInfo suitable for equity/stock trading.
// commission: e.g. 0.001 = 0.1% of trade value.
func NewStockCommInfo(commission float64) CommInfo {
	return CommInfo{
		Commission: commission,
		CommType:   CommTypePerc,
		PercAbs:    true,
		Mult:       1.0,
		Leverage:   1.0,
		StockLike:  true,
	}
}

// NewFuturesCommInfo creates a CommInfo for futures instruments.
// commission: per-contract round-trip or one-way fee.
// margin: required margin per contract in currency units.
// mult: contract multiplier (e.g. 50 for ES, 1000 for CL).
func NewFuturesCommInfo(commission, margin, mult float64) CommInfo {
	return CommInfo{
		Commission: commission,
		CommType:   CommTypeFixed,
		Margin:     margin,
		Mult:       mult,
		Leverage:   1.0,
		StockLike:  false,
	}
}

// GetCommission returns the commission for a given trade.
func (c CommInfo) GetCommission(size, price float64) float64 {
	size = math.Abs(size)
	switch c.CommType {
	case CommTypePerc:
		rate := c.Commission
		if !c.PercAbs {
			rate /= 100.0 // interpret XX% as decimal
		}
		return size * price * c.Mult * rate
	case CommTypeFixed:
		return size * c.Commission
	}
	return 0
}

// GetMargin returns the margin required to open/hold size contracts at price.
// For stock-like: returns notional value / leverage.
// For futures: returns Margin * size (or auto-computed if AutoMargin != 0).
func (c CommInfo) GetMargin(size, price float64) float64 {
	size = math.Abs(size)
	if c.StockLike {
		lev := c.Leverage
		if lev <= 0 {
			lev = 1.0
		}
		return size * price / lev
	}
	// Futures margin
	if c.AutoMargin > 0 {
		return size * price * c.AutoMargin
	}
	if c.AutoMargin < 0 {
		return size * price * c.Mult
	}
	return size * c.Margin
}

// GetValue returns the notional value of a position.
// For stocks: size * price.
// For futures: size * price * Mult.
func (c CommInfo) GetValue(size, price float64) float64 {
	mult := c.Mult
	if mult <= 0 {
		mult = 1.0
	}
	return math.Abs(size) * price * mult
}

// GetLeverage returns the leverage factor.
func (c CommInfo) GetLeverage() float64 {
	if c.Leverage <= 0 {
		return 1.0
	}
	return c.Leverage
}

// ProfitAndLoss calculates realized PnL.
// For stocks: (exitPrice - entryPrice) * size.
// For futures: (exitPrice - entryPrice) * size * Mult.
func (c CommInfo) ProfitAndLoss(size, entryPrice, exitPrice float64) float64 {
	mult := c.Mult
	if mult <= 0 {
		mult = 1.0
	}
	return size * (exitPrice - entryPrice) * mult
}

// DailyInterest calculates the interest charge for holding a short (or long) position
// over the given number of days.
// Formula: days * price * abs(size) * (interest / 365)
func (c CommInfo) DailyInterest(size, price float64, days int) float64 {
	if c.Interest == 0 {
		return 0
	}
	if !c.InterestLong && size > 0 {
		return 0 // only charge longs if InterestLong is set
	}
	return float64(days) * price * math.Abs(size) * (c.Interest / 365.0)
}

// Satisfy CommissionInfo interface so CommInfo can be used everywhere CommissionInfo is accepted.
func (c CommInfo) GetCommissionInfo() CommissionInfo { return c }

// CashCost returns the cash cost of opening a position (buy side).
// For stocks: size * price / leverage.
// For futures: margin per contract (not the full notional).
func (c CommInfo) CashCost(size, price float64) float64 {
	if c.StockLike {
		return c.GetValue(size, price) / c.GetLeverage()
	}
	return c.GetMargin(size, price)
}
