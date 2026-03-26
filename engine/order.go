// Package engine contains the backtesting engine components.
// This file defines order types, statuses, and the Order struct,
// mirroring backtrader's order.py with full feature parity.
package engine

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/thangtmkafi/gobacktrader/core"
)

// OrderType distinguishes market, limit, stop, trail, etc.
type OrderType int

const (
	OrderTypeMarket         OrderType = iota // execute at next bar's open
	OrderTypeClose                           // execute at bar's close price
	OrderTypeLimit                           // execute only if price reaches limit
	OrderTypeStop                            // becomes market when stop price hit
	OrderTypeStopLimit                       // becomes limit when stop price hit
	OrderTypeStopTrail                       // trailing stop: price tracks favorably
	OrderTypeStopTrailLimit                  // trailing stop → limit execution
)

func (t OrderType) String() string {
	switch t {
	case OrderTypeMarket:
		return "Market"
	case OrderTypeClose:
		return "Close"
	case OrderTypeLimit:
		return "Limit"
	case OrderTypeStop:
		return "Stop"
	case OrderTypeStopLimit:
		return "StopLimit"
	case OrderTypeStopTrail:
		return "StopTrail"
	case OrderTypeStopTrailLimit:
		return "StopTrailLimit"
	default:
		return fmt.Sprintf("OrderType(%d)", int(t))
	}
}

// OrderSide is Buy or Sell.
type OrderSide int

const (
	OrderSideBuy  OrderSide = iota
	OrderSideSell // covers both short-sell and close-long
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

// ValidType controls when an order expires.
type ValidType int

const (
	ValidGTC ValidType = iota // Good Till Cancelled (default, no expiry)
	ValidDAY                  // Expire at end of the current session/bar
	ValidGTD                  // Good Till Date (expires at ValidTime)
)

// OrderExecutionBit records a single partial fill.
type OrderExecutionBit struct {
	Time       time.Time
	Size       float64
	Price      float64
	Value      float64
	Commission float64
	PnL        float64
}

var orderRefSeq int64

// Order represents a single trade order with full Backtrader parity.
type Order struct {
	Ref  int // unique monotonically increasing reference
	Data *core.DataSeries
	Side OrderSide
	Type OrderType
	Size float64 // requested size (shares/contracts)

	// Price fields
	Price  float64 // primary price: limit, stop, or initial trail stop price
	Price2 float64 // secondary: StopLimit limit price, StopTrailLimit limit offset

	// Trailing stop fields (StopTrail / StopTrailLimit)
	TrailAmount  float64 // absolute distance from price to trail stop
	TrailPercent float64 // percentage distance (0.05 = 5%)
	TrailStop    float64 // computed/updated trail stop price (changes each bar)

	// Order validity
	ValidType ValidType // GTC / DAY / GTD
	ValidTime time.Time // expiry time for GTD orders

	// OCO (One-Cancels-Other) group
	OcoRef int // if > 0, all orders with same OcoRef cancel each other on fill

	// Bracket / parent ordering
	ParentRef int  // if > 0, this is a child order (stop-loss or take-profit)
	Transmit  bool // if false, hold until parent transmits; new orders default true

	// Status and execution
	Status OrderStatus

	// Execution summary
	ExecSize  float64 // total executed size so far
	ExecPrice float64 // volume-weighted average executed price
	ExecValue float64 // total executed notional value
	ExecComm  float64 // total commission paid
	ExecPnL   float64 // realized PnL on this order

	// Partial fill history
	ExecBits []*OrderExecutionBit

	// Timestamps
	CreatedAt   time.Time // wall-clock time of order creation
	BarCreatedAt time.Time // bar datetime when order was submitted (used for DAY expiry)
	ExecutedAt  time.Time

	// Internal broker tracking
	pannotated interface{} // used by bracket logic
}

// NewOrder creates an order with a globally unique Ref.
func NewOrder(data *core.DataSeries, side OrderSide, ot OrderType, size, price, price2 float64) *Order {
	ref := int(atomic.AddInt64(&orderRefSeq, 1))
	return &Order{
		Ref:       ref,
		Data:      data,
		Side:      side,
		Type:      ot,
		Size:      size,
		Price:     price,
		Price2:    price2,
		ValidType: ValidGTC, // default: no expiry
		Transmit:  true,     // default: transmit immediately
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

// recordFill records a partial or full fill execution bit.
func (o *Order) recordFill(t time.Time, size, price, value, comm, pnl float64) {
	bit := &OrderExecutionBit{
		Time:       t,
		Size:       size,
		Price:      price,
		Value:      value,
		Commission: comm,
		PnL:        pnl,
	}
	o.ExecBits = append(o.ExecBits, bit)

	// Update running averages
	oldTotal := o.ExecSize * o.ExecPrice
	o.ExecSize += size
	if o.ExecSize > 0 {
		o.ExecPrice = (oldTotal + size*price) / o.ExecSize
	}
	o.ExecValue += value
	o.ExecComm += comm
	o.ExecPnL += pnl
}

func (o *Order) String() string {
	return fmt.Sprintf("Order[%d] %s %s %.4g @ %.4g (%s)",
		o.Ref, o.Side, o.Type, o.Size, o.Price, o.Status)
}
