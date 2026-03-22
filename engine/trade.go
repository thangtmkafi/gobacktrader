// Package engine contains the backtesting engine components.
// This file implements trade tracking, mirroring backtrader's trade.py.
package engine

import "time"

// Trade represents a complete round-trip (open → close) of a position.
// A trade is opened on the first fill and closed when the position returns to zero.
type Trade struct {
	Ref int // unique reference

	DataName string

	// Entry details
	EntryOrder *Order
	EntryPrice float64
	EntryValue float64
	EntryComm  float64
	EntryTime  time.Time

	// Exit details (set when closed)
	ExitOrder *Order
	ExitPrice float64
	ExitValue float64
	ExitComm  float64
	ExitTime  time.Time

	Size float64 // size of the trade

	// Result
	PnL     float64 // gross PnL
	PnLComm float64 // net PnL after commissions

	IsOpen bool // true while position is still open
}

var tradeRefCounter int

// newTrade creates a new open trade from a fill.
func newTrade(order *Order, size, price, comm float64) *Trade {
	tradeRefCounter++
	return &Trade{
		Ref:        tradeRefCounter,
		DataName:   order.Data.Name,
		EntryOrder: order,
		EntryPrice: price,
		EntryValue: size * price,
		EntryComm:  comm,
		EntryTime:  order.ExecutedAt,
		Size:       size,
		IsOpen:     true,
	}
}

// close fills in the exit details and computes PnL.
func (t *Trade) close(order *Order, price, comm float64) {
	t.ExitOrder = order
	t.ExitPrice = price
	t.ExitValue = t.Size * price
	t.ExitComm = comm
	t.ExitTime = order.ExecutedAt
	t.IsOpen = false

	// long trade: profit = (exit - entry) * size
	t.PnL = (price - t.EntryPrice) * t.Size
	t.PnLComm = t.PnL - (t.EntryComm + t.ExitComm)
}
