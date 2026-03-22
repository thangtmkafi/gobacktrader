// Package engine contains the backtesting engine components.
// This file defines order types, statuses, and the Order struct,
// mirroring backtrader's order.py.
package engine

import (
	"fmt"
	"time"

	"github.com/thangtmkafi/gobacktrader/core"
)

// OrderType distinguishes market, limit, stop, etc.
type OrderType int

const (
	OrderTypeMarket    OrderType = iota // execute at next bar's open
	OrderTypeLimit                      // execute only if price reaches limit
	OrderTypeStop                       // becomes market when stop price hit
	OrderTypeStopLimit                  // becomes limit when stop price hit
	OrderTypeClose                      // execute at bar's close price
)

func (t OrderType) String() string {
	switch t {
	case OrderTypeMarket:
		return "Market"
	case OrderTypeLimit:
		return "Limit"
	case OrderTypeStop:
		return "Stop"
	case OrderTypeStopLimit:
		return "StopLimit"
	case OrderTypeClose:
		return "Close"
	default:
		return fmt.Sprintf("OrderType(%d)", int(t))
	}
}

// OrderSide is Buy or Sell.
type OrderSide int

const (
	OrderSideBuy  OrderSide = iota
	OrderSideSell           // covers both short-sell and close-long
)

func (s OrderSide) String() string {
	if s == OrderSideBuy {
		return "Buy"
	}
	return "Sell"
}

// OrderStatus tracks the lifecycle of an order.
type OrderStatus int

const (
	OrderStatusCreated   OrderStatus = iota
	OrderStatusSubmitted             // handed to broker
	OrderStatusAccepted             // broker accepted
	OrderStatusPartial              // partially filled
	OrderStatusCompleted            // fully filled
	OrderStatusCanceled
	OrderStatusExpired
	OrderStatusRejected
	OrderStatusMargin // rejected due to insufficient margin
)

func (s OrderStatus) String() string {
	switch s {
	case OrderStatusCreated:
		return "Created"
	case OrderStatusSubmitted:
		return "Submitted"
	case OrderStatusAccepted:
		return "Accepted"
	case OrderStatusPartial:
		return "Partial"
	case OrderStatusCompleted:
		return "Completed"
	case OrderStatusCanceled:
		return "Canceled"
	case OrderStatusExpired:
		return "Expired"
	case OrderStatusRejected:
		return "Rejected"
	case OrderStatusMargin:
		return "Margin"
	default:
		return fmt.Sprintf("OrderStatus(%d)", int(s))
	}
}

var orderRefCounter int

// Order represents a single trade order.
type Order struct {
	Ref    int // unique reference number
	Data   *core.DataSeries
	Side   OrderSide
	Type   OrderType
	Size   float64 // requested size (shares/contracts)
	Price  float64 // limit or stop price (0 for market)
	Price2 float64 // second price for StopLimit (the limit price)

	Status OrderStatus

	// Execution details (filled in by broker)
	ExecSize  float64 // total executed size so far
	ExecPrice float64 // average executed price
	ExecValue float64 // total executed value
	ExecComm  float64 // total commission paid

	CreatedAt time.Time
	ExecutedAt time.Time
}

// newOrder is used internally by the broker.
func newOrder(data *core.DataSeries, side OrderSide, ot OrderType, size, price, price2 float64) *Order {
	orderRefCounter++
	return &Order{
		Ref:       orderRefCounter,
		Data:      data,
		Side:      side,
		Type:      ot,
		Size:      size,
		Price:     price,
		Price2:    price2,
		Status:    OrderStatusCreated,
		CreatedAt: time.Now(),
	}
}

// IsBuy returns true for buy orders.
func (o *Order) IsBuy() bool { return o.Side == OrderSideBuy }

// RemSize returns remaining unfilled size.
func (o *Order) RemSize() float64 { return o.Size - o.ExecSize }

// IsCompleted returns true if the order is fully or terminally done.
func (o *Order) IsCompleted() bool {
	return o.Status == OrderStatusCompleted ||
		o.Status == OrderStatusCanceled ||
		o.Status == OrderStatusExpired ||
		o.Status == OrderStatusRejected ||
		o.Status == OrderStatusMargin
}

func (o *Order) String() string {
	return fmt.Sprintf("Order[%d] %s %s %g @ %g (%s)",
		o.Ref, o.Side, o.Type, o.Size, o.Price, o.Status)
}
