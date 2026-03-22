// Package engine contains the backtesting engine components.
// This file implements the simulated Broker, mirroring backtrader's broker.py.
package engine

import (
	"fmt"
	"time"

	"github.com/thangtmkafi/gobacktrader/core"
)

// Broker simulates order execution, cash management, and position tracking.
type Broker struct {
	cash       float64
	commission CommissionInfo
	positions  map[string]*Position // keyed by DataSeries.Name
	pending    []*Order
	Trades     []*Trade
	completed  []*Order // recent filled/canceled orders (flushed each bar)

	// Notifications are queued here for the strategy to consume.
	orderNotifications []*Order
	tradeNotifications []*Trade
}

// NewBroker creates a Broker with the given starting cash.
func NewBroker(cash float64) *Broker {
	return &Broker{
		cash:      cash,
		positions: make(map[string]*Position),
		commission: ZeroCommission{},
	}
}

// SetCash sets the starting / current cash balance.
func (b *Broker) SetCash(cash float64) { b.cash = cash }

// GetCash returns current cash balance.
func (b *Broker) GetCash() float64 { return b.cash }

// SetCommission sets the commission scheme.
func (b *Broker) SetCommission(c CommissionInfo) { b.commission = c }

// GetValue returns total portfolio value: cash + open position values.
func (b *Broker) GetValue(datas []*core.DataSeries) float64 {
	total := b.cash
	for _, data := range datas {
		pos := b.positions[data.Name]
		if pos == nil || !pos.IsOpen() {
			continue
		}
		total += pos.Size * data.Close.Get(0)
	}
	return total
}

// GetPosition returns the position for the given data series.
func (b *Broker) GetPosition(data *core.DataSeries) *Position {
	p := b.positions[data.Name]
	if p == nil {
		p = &Position{}
		b.positions[data.Name] = p
	}
	return p
}

// Submit queues an order with the broker.
func (b *Broker) Submit(order *Order) {
	order.Status = OrderStatusSubmitted
	b.pending = append(b.pending, order)
	b.orderNotifications = append(b.orderNotifications, order)
}

// Cancel cancels a pending order.
func (b *Broker) Cancel(order *Order) {
	order.Status = OrderStatusCanceled
	b.orderNotifications = append(b.orderNotifications, order)
	// Remove from pending
	for i, o := range b.pending {
		if o.Ref == order.Ref {
			b.pending = append(b.pending[:i], b.pending[i+1:]...)
			return
		}
	}
}

// DrainNotifications returns and clears pending order/trade notifications.
func (b *Broker) DrainNotifications() ([]*Order, []*Trade) {
	orders := b.orderNotifications
	trades := b.tradeNotifications
	b.orderNotifications = nil
	b.tradeNotifications = nil
	return orders, trades
}

// Next processes all pending orders against the current bar data.
// Should be called once per bar, after the bar is loaded.
func (b *Broker) Next(datas []*core.DataSeries) {
	var remaining []*Order
	for _, order := range b.pending {
		if order.Status == OrderStatusCanceled || order.Status == OrderStatusRejected {
			continue
		}
		data := order.Data
		bar := data.Bar()

		filled := false
		fillPrice := 0.0

		switch order.Type {
		case OrderTypeMarket:
			// Fill at open of current bar (order was placed on previous bar)
			fillPrice = bar.Open
			filled = true

		case OrderTypeClose:
			fillPrice = bar.Close
			filled = true

		case OrderTypeLimit:
			if order.IsBuy() && bar.Low <= order.Price {
				fillPrice = order.Price
				filled = true
			} else if !order.IsBuy() && bar.High >= order.Price {
				fillPrice = order.Price
				filled = true
			}

		case OrderTypeStop:
			if order.IsBuy() && bar.High >= order.Price {
				fillPrice = order.Price
				filled = true
			} else if !order.IsBuy() && bar.Low <= order.Price {
				fillPrice = order.Price
				filled = true
			}

		case OrderTypeStopLimit:
			// First check stop trigger
			stopTriggered := false
			if order.IsBuy() && bar.High >= order.Price {
				stopTriggered = true
			} else if !order.IsBuy() && bar.Low <= order.Price {
				stopTriggered = true
			}
			if stopTriggered {
				// Then check limit
				if order.IsBuy() && bar.Low <= order.Price2 {
					fillPrice = order.Price2
					filled = true
				} else if !order.IsBuy() && bar.High >= order.Price2 {
					fillPrice = order.Price2
					filled = true
				}
			}
		}

		if !filled {
			remaining = append(remaining, order)
			continue
		}

		// Execute the fill
		size := order.Size
		comm := b.commission.GetCommission(size, fillPrice)

		if order.IsBuy() {
			cost := size*fillPrice + comm
			if cost > b.cash {
				order.Status = OrderStatusMargin
				b.orderNotifications = append(b.orderNotifications, order)
				continue
			}
			b.cash -= cost
		} else {
			b.cash += size*fillPrice - comm
		}

		order.ExecSize = size
		order.ExecPrice = fillPrice
		order.ExecValue = size * fillPrice
		order.ExecComm = comm
		order.Status = OrderStatusCompleted
		order.ExecutedAt = time.Now() // wall-clock; use bar.DateTime for real time

		b.orderNotifications = append(b.orderNotifications, order)

		// Update position and track trade
		pos := b.GetPosition(data)
		signedSize := size
		if !order.IsBuy() {
			signedSize = -size
		}
		b.updatePositionAndTrades(order, pos, signedSize, fillPrice, comm)
	}
	b.pending = remaining
}

func (b *Broker) updatePositionAndTrades(order *Order, pos *Position, signedSize, price, comm float64) {
	wasOpen := pos.IsOpen()
	wasSize := pos.Size

	_ = pos.Update(signedSize, price)

	if !wasOpen && pos.IsOpen() {
		// New trade opened
		abs := signedSize
		if abs < 0 {
			abs = -abs
		}
		t := newTrade(order, abs, price, comm)
		b.Trades = append(b.Trades, t)
		b.tradeNotifications = append(b.tradeNotifications, t)
	} else if wasOpen && !pos.IsOpen() {
		// Trade closed — find the open trade for this data
		for i := len(b.Trades) - 1; i >= 0; i-- {
			t := b.Trades[i]
			if t.IsOpen && t.DataName == order.Data.Name {
				t.close(order, price, comm)
				b.tradeNotifications = append(b.tradeNotifications, t)
				break
			}
		}
	} else if wasOpen && pos.IsOpen() && signedSize != 0 {
		// Position partially closed or extended — simplified: just notify
		_ = wasSize
		for i := len(b.Trades) - 1; i >= 0; i-- {
			t := b.Trades[i]
			if t.IsOpen && t.DataName == order.Data.Name {
				b.tradeNotifications = append(b.tradeNotifications, t)
				break
			}
		}
	}
}

// Diagnostics

// PendingOrders returns currently pending orders.
func (b *Broker) PendingOrders() []*Order { return b.pending }

// String returns a summary of the broker state.
func (b *Broker) String() string {
	return fmt.Sprintf("Broker[cash=%.2f, pending=%d, trades=%d]",
		b.cash, len(b.pending), len(b.Trades))
}
