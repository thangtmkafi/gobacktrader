// Package engine contains the backtesting engine components.
// This file defines the Strategy interface, StrategyContext, and all
// order creation helpers and lifecycle hooks — mirroring backtrader's strategy.py.
package engine

import (
	"fmt"
	"math"
	"time"

	"github.com/thangtmkafi/gobacktrader/core"
)

// ─── Strategy interface ────────────────────────────────────────────────────

// Strategy is the interface users implement to define trading logic.
type Strategy interface {
	// Init is called once before the backtest loop starts.
	// Use it to initialise indicators, state, etc.
	Init(ctx *StrategyContext)

	// Next is called once per bar after all data feeds have advanced.
	Next()
}

// ─── Optional lifecycle hooks (detected via type assertion) ───────────────

// Starter is implemented by strategies that want a Start() call.
type Starter interface{ Start() }

// Stopper is implemented by strategies that want a Stop() call.
type Stopper interface{ Stop() }

// PreNexter is called each bar while the strategy is still in warmup (prenext).
type PreNexter interface{ PreNext() }

// NextStarter is called exactly once when warmup ends (nextstart).
type NextStarter interface{ NextStart() }

// NotifyOrderer receives order status change notifications.
type NotifyOrderer interface{ NotifyOrder(order *Order) }

// NotifyTrader receives trade open/close notifications.
type NotifyTrader interface{ NotifyTrade(trade *Trade) }

// CashValueNotifier receives cash/value each bar.
type CashValueNotifier interface{ NotifyCashValue(cash, value float64) }

// ─── StrategyContext ───────────────────────────────────────────────────────

// StrategyContext is handed to the strategy in Init and provides
// access to data, broker, and helper trading methods.
type StrategyContext struct {
	// Datas is the list of all live data feeds (advances bar-by-bar during Run).
	Datas []*core.DataSeries

	// PreloadedDatas holds fully pre-populated DataSeries (all bars loaded).
	// Use these to construct indicators in Init(); use Datas in Next().
	PreloadedDatas []*core.DataSeries

	broker BrokerBase
}

// Data returns the first (primary) live data feed.
func (ctx *StrategyContext) Data() *core.DataSeries {
	if len(ctx.Datas) == 0 {
		return nil
	}
	return ctx.Datas[0]
}

// PreloadedData returns the first pre-populated DataSeries (for indicator init).
func (ctx *StrategyContext) PreloadedData() *core.DataSeries {
	if len(ctx.PreloadedDatas) == 0 {
		return nil
	}
	return ctx.PreloadedDatas[0]
}

// GetBroker returns the broker (for advanced use).
func (ctx *StrategyContext) GetBroker() BrokerBase { return ctx.broker }

// GetPosition returns the current position for the given data series.
func (ctx *StrategyContext) GetPosition(data *core.DataSeries) *Position {
	return ctx.broker.GetPosition(data)
}

// GetCash returns available cash.
func (ctx *StrategyContext) GetCash() float64 { return ctx.broker.GetCash() }

// GetValue returns total portfolio value.
func (ctx *StrategyContext) GetValue() float64 { return ctx.broker.GetValue(ctx.Datas) }

// GetOrdersOpen returns all currently pending orders.
func (ctx *StrategyContext) GetOrdersOpen() []*Order { return ctx.broker.GetOrdersOpen() }

// AddCash dynamically adds (or withdraws if negative) cash during the backtest.
func (ctx *StrategyContext) AddCash(delta float64) { ctx.broker.AddCash(delta) }

// ─── Order helpers ─────────────────────────────────────────────────────────

// Buy creates a buy order on the primary data feed and submits it to the broker.
func (ctx *StrategyContext) Buy(size float64, opts ...OrderOption) *Order {
	return ctx.BuyData(ctx.Data(), size, opts...)
}

// Sell creates a sell order on the primary data feed and submits it to the broker.
func (ctx *StrategyContext) Sell(size float64, opts ...OrderOption) *Order {
	return ctx.SellData(ctx.Data(), size, opts...)
}

// Close closes the entire current position on the primary data feed.
func (ctx *StrategyContext) Close(opts ...OrderOption) *Order {
	return ctx.CloseData(ctx.Data(), opts...)
}

// CloseData closes the position on a specific data feed.
func (ctx *StrategyContext) CloseData(data *core.DataSeries, opts ...OrderOption) *Order {
	pos := ctx.broker.GetPosition(data)
	if !pos.IsOpen() {
		return nil
	}
	if pos.Size > 0 {
		return ctx.SellData(data, pos.Size, opts...)
	}
	return ctx.BuyData(data, -pos.Size, opts...)
}

// BuyData creates a buy order on the given data series.
func (ctx *StrategyContext) BuyData(data *core.DataSeries, size float64, opts ...OrderOption) *Order {
	cfg := defaultOrderCfg()
	for _, o := range opts {
		o(&cfg)
	}
	order := NewOrder(data, OrderSideBuy, cfg.ot, size, cfg.price, cfg.price2)
	applyOrderCfg(order, cfg)
	ctx.broker.Submit(order)
	return order
}

// SellData creates a sell order on the given data series.
func (ctx *StrategyContext) SellData(data *core.DataSeries, size float64, opts ...OrderOption) *Order {
	cfg := defaultOrderCfg()
	for _, o := range opts {
		o(&cfg)
	}
	order := NewOrder(data, OrderSideSell, cfg.ot, size, cfg.price, cfg.price2)
	applyOrderCfg(order, cfg)
	ctx.broker.Submit(order)
	return order
}

// applyOrderCfg copies cfg fields into the order (beyond what newOrder accepts).
func applyOrderCfg(order *Order, cfg orderCfg) {
	order.TrailAmount = cfg.trailAmount
	order.TrailPercent = cfg.trailPercent
	order.ValidType = cfg.validType
	order.ValidTime = cfg.validTime
	order.OcoRef = cfg.ocoRef
	order.ParentRef = cfg.parentRef
	order.Transmit = cfg.transmit
}

// Cancel cancels a pending order.
func (ctx *StrategyContext) Cancel(order *Order) { ctx.broker.Cancel(order) }

// ─── Order target helpers ──────────────────────────────────────────────────

// OrderTargetSize adjusts the position on the primary data to reach targetSize.
// Returns the order placed, or nil if already at the target.
func (ctx *StrategyContext) OrderTargetSize(target float64, opts ...OrderOption) *Order {
	return ctx.OrderTargetSizeData(ctx.Data(), target, opts...)
}

// OrderTargetSizeData adjusts position on a specific data to reach targetSize.
func (ctx *StrategyContext) OrderTargetSizeData(data *core.DataSeries, target float64, opts ...OrderOption) *Order {
	pos := ctx.broker.GetPosition(data)
	current := pos.Size
	diff := target - current
	if math.Abs(diff) < 1e-10 {
		return nil
	}
	if diff > 0 {
		return ctx.BuyData(data, diff, opts...)
	}
	return ctx.SellData(data, -diff, opts...)
}

// OrderTargetValue adjusts position to reach a target notional value.
func (ctx *StrategyContext) OrderTargetValue(targetValue float64, opts ...OrderOption) *Order {
	data := ctx.Data()
	if data == nil || data.Len() == 0 {
		return nil
	}
	price := data.Close.Get(0)
	if price <= 0 {
		return nil
	}
	targetSize := targetValue / price
	return ctx.OrderTargetSizeData(data, targetSize, opts...)
}

// OrderTargetPercent adjusts position so its value is pct fraction of total portfolio.
// e.g. 0.10 = 10% of portfolio.
func (ctx *StrategyContext) OrderTargetPercent(pct float64, opts ...OrderOption) *Order {
	totalValue := ctx.GetValue()
	return ctx.OrderTargetValue(totalValue*pct, opts...)
}

// BuyBracket places a bracket order: entry buy + stop-loss sell + take-profit sell.
// stopPrice: stop-loss trigger. limitPrice: take-profit limit.
func (ctx *StrategyContext) BuyBracket(size, limitPrice, stopPrice float64, opts ...OrderOption) (*Order, *Order, *Order) {
	entry := ctx.BuyData(ctx.Data(), size, opts...)

	takeProfit := NewOrder(ctx.Data(), OrderSideSell, OrderTypeLimit, size, limitPrice, 0)
	takeProfit.ParentRef = entry.Ref
	stopLoss := NewOrder(ctx.Data(), OrderSideSell, OrderTypeStop, size, stopPrice, 0)
	stopLoss.ParentRef = entry.Ref

	ocoRef := takeProfit.Ref
	takeProfit.OcoRef = ocoRef
	stopLoss.OcoRef = ocoRef

	ctx.broker.Submit(takeProfit)
	ctx.broker.Submit(stopLoss)

	return entry, takeProfit, stopLoss
}

// SellBracket places a bracket order: entry sell + stop-loss buy + take-profit buy.
func (ctx *StrategyContext) SellBracket(size, limitPrice, stopPrice float64, opts ...OrderOption) (*Order, *Order, *Order) {
	entry := ctx.SellData(ctx.Data(), size, opts...)

	takeProfit := NewOrder(ctx.Data(), OrderSideBuy, OrderTypeLimit, size, limitPrice, 0)
	takeProfit.ParentRef = entry.Ref
	stopLoss := NewOrder(ctx.Data(), OrderSideBuy, OrderTypeStop, size, stopPrice, 0)
	stopLoss.ParentRef = entry.Ref

	ocoRef := takeProfit.Ref
	takeProfit.OcoRef = ocoRef
	stopLoss.OcoRef = ocoRef

	ctx.broker.Submit(takeProfit)
	ctx.broker.Submit(stopLoss)

	return entry, takeProfit, stopLoss
}

// Log prints a timestamped message. Uses the current bar's datetime.
func (ctx *StrategyContext) Log(format string, args ...any) {
	data := ctx.Data()
	var dt time.Time
	if data != nil && data.Len() > 0 {
		dt = data.Bar().DateTime
	} else {
		dt = time.Now()
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("[%s] %s\n", dt.Format("2006-01-02"), msg)
}

// ─── Order configuration ───────────────────────────────────────────────────

type orderCfg struct {
	ot           OrderType
	price        float64
	price2       float64
	trailAmount  float64
	trailPercent float64
	validType    ValidType
	validTime    time.Time
	ocoRef       int
	parentRef    int
	transmit     bool
}

func defaultOrderCfg() orderCfg {
	return orderCfg{ot: OrderTypeMarket, transmit: true}
}

// OrderOption is a functional option applied when creating an order.
type OrderOption func(*orderCfg)

// WithLimit creates a limit order at the specified price.
func WithLimit(price float64) OrderOption {
	return func(c *orderCfg) {
		c.ot = OrderTypeLimit
		c.price = price
	}
}

// WithStop creates a stop order at the specified price.
func WithStop(price float64) OrderOption {
	return func(c *orderCfg) {
		c.ot = OrderTypeStop
		c.price = price
	}
}

// WithStopLimit creates a stop-limit order.
func WithStopLimit(stopPrice, limitPrice float64) OrderOption {
	return func(c *orderCfg) {
		c.ot = OrderTypeStopLimit
		c.price = stopPrice
		c.price2 = limitPrice
	}
}

// WithClose creates a close-of-bar order (fills at bar's close price).
func WithClose() OrderOption {
	return func(c *orderCfg) { c.ot = OrderTypeClose }
}

// WithStopTrail creates a trailing stop order.
// price is the initial stop price.
// trailAmount is the absolute distance to maintain from the best price.
func WithStopTrail(price, trailAmount float64) OrderOption {
	return func(c *orderCfg) {
		c.ot = OrderTypeStopTrail
		c.price = price
		c.trailAmount = trailAmount
	}
}

// WithStopTrailPercent creates a trailing stop using a percentage distance.
// pct: 0.02 = 2% from current price.
func WithStopTrailPercent(price, pct float64) OrderOption {
	return func(c *orderCfg) {
		c.ot = OrderTypeStopTrail
		c.price = price
		c.trailPercent = pct
	}
}

// WithStopTrailLimit creates a trailing stop-limit order.
// limitOffset is the limit price offset relative to the trail stop.
func WithStopTrailLimit(price, trailAmount, limitOffset float64) OrderOption {
	return func(c *orderCfg) {
		c.ot = OrderTypeStopTrailLimit
		c.price = price
		c.price2 = limitOffset
		c.trailAmount = trailAmount
	}
}

// WithValid sets the order expiry to a specific date/time (Good Till Date).
func WithValid(t time.Time) OrderOption {
	return func(c *orderCfg) {
		c.validType = ValidGTD
		c.validTime = t
	}
}

// WithDAY makes the order expire at the end of the current day.
func WithDAY() OrderOption {
	return func(c *orderCfg) { c.validType = ValidDAY }
}

// WithOCO links this order to an OCO group keyed by the reference order's Ref.
// When one order in the group fills, all others are cancelled.
func WithOCO(ref *Order) OrderOption {
	return func(c *orderCfg) {
		if ref != nil {
			c.ocoRef = ref.Ref
		}
	}
}

// WithParent marks this order as a child of a bracket parent order.
// The child will only activate when the parent is fully filled.
func WithParent(parent *Order) OrderOption {
	return func(c *orderCfg) {
		if parent != nil {
			c.parentRef = parent.Ref
			c.transmit = true
		}
	}
}

// WithTransmit controls whether the order is transmitted immediately.
func WithTransmit(transmit bool) OrderOption {
	return func(c *orderCfg) { c.transmit = transmit }
}
