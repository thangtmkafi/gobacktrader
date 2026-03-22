// Package engine contains the backtesting engine components.
// This file provides position sizing helpers.
package engine

import "github.com/thangtmkafi/gobacktrader/core"

// Sizer computes how many shares/contracts to trade.
type Sizer interface {
	Size(ctx *StrategyContext, data *core.DataSeries) float64
}

// FixedSizer always returns a fixed number of shares.
type FixedSizer struct {
	Amount float64
}

func (s FixedSizer) Size(_ *StrategyContext, _ *core.DataSeries) float64 {
	return s.Amount
}

// PercentSizer sizes the position as a percentage of current portfolio value.
// Percent is expressed as a decimal: 0.10 = 10% of portfolio.
type PercentSizer struct {
	Percent float64
}

func (s PercentSizer) Size(ctx *StrategyContext, data *core.DataSeries) float64 {
	val := ctx.GetValue()
	price := data.Close.Get(0)
	if price <= 0 {
		return 0
	}
	return (val * s.Percent) / price
}

// AllInSizer sizes the position using all available cash.
type AllInSizer struct{}

func (AllInSizer) Size(ctx *StrategyContext, data *core.DataSeries) float64 {
	cash := ctx.GetCash()
	price := data.Close.Get(0)
	if price <= 0 {
		return 0
	}
	return cash / price
}
