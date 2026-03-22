// Package engine contains the backtesting engine components.
// This file implements position tracking, mirroring backtrader's position.py.
package engine

import "math"

// Position tracks the current holding for a single instrument.
// Size > 0 → long, Size < 0 → short (Phase 1 only supports long).
type Position struct {
	Size  float64 // current position size (positive = long)
	Price float64 // average entry price
}

// IsOpen returns true if there is an open position.
func (p *Position) IsOpen() bool { return p.Size != 0 }

// Update adjusts the position when an order fills.
// size is signed: positive for a buy fill, negative for a sell fill.
// price is the fill price.
// Returns the realised PnL of any closed portion.
func (p *Position) Update(size, price float64) float64 {
	if size == 0 {
		return 0
	}

	pnl := 0.0
	newSize := p.Size + size

	if p.Size == 0 {
		// Opening a new position
		p.Price = price
		p.Size = newSize
		return 0
	}

	// Same direction: extend the position (average in)
	if math.Signbit(size) == math.Signbit(p.Size) {
		// Weighted average price
		p.Price = (p.Price*math.Abs(p.Size) + price*math.Abs(size)) /
			(math.Abs(p.Size) + math.Abs(size))
		p.Size = newSize
		return 0
	}

	// Opposite direction: closing (partially or fully)
	closingSize := math.Min(math.Abs(size), math.Abs(p.Size))
	if p.Size > 0 {
		pnl = closingSize * (price - p.Price)
	} else {
		pnl = closingSize * (p.Price - price)
	}

	if math.Abs(newSize) < 1e-9 {
		// Fully closed
		p.Size = 0
		p.Price = 0
	} else if math.Signbit(newSize) != math.Signbit(p.Size) {
		// Reversed: small remainder in opposite direction
		p.Size = newSize
		p.Price = price // entry price of the new reversed leg
	} else {
		// Partially closed, same direction
		p.Size = newSize
		// price (of remaining position) stays the same
	}

	return pnl
}

// PnL returns the unrealised PnL at the given current market price.
func (p *Position) PnL(currentPrice float64) float64 {
	if !p.IsOpen() {
		return 0
	}
	return p.Size * (currentPrice - p.Price)
}
