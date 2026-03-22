// Package engine contains the backtesting engine components.
// This file defines the Strategy interface and StrategyContext,
// mirroring backtrader's strategy.py.
package engine

import (
	"fmt"
	"time"

	"github.com/thangtmkafi/gobacktrader/core"
)

// Strategy is the interface users implement to define trading logic.
type Strategy interface {
	// Init is called once before the backtest loop starts.
	// Use it to initialise indicators, state, etc.
	Init(ctx *StrategyContext)

	// Next is called once per bar after all data feeds have advanced.
	Next()
}

// NotifyOrderer is an optional interface strategies can implement
// to receive order status change notifications.
type NotifyOrderer interface {
	NotifyOrder(order *Order)
}

// NotifyTrader is an optional interface strategies can implement
// to receive trade open/close notifications.
type NotifyTrader interface {
	NotifyTrade(trade *Trade)
}

// StrategyContext is handed to the strategy in Init and provides
// access to data, broker, and helper trading methods.
type StrategyContext struct {
	// Datas is the list of all live data feeds (advances bar-by-bar during Run).
	Datas []*core.DataSeries

	// PreloadedDatas holds fully pre-populated DataSeries (all bars loaded).
	// Use these to construct indicators in Init(); use Datas in Next().
	PreloadedDatas []*core.DataSeries

	broker *Broker
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
func (ctx *StrategyContext) GetBroker() *Broker { return ctx.broker }

// GetPosition returns the current position for the given data series.
func (ctx *StrategyContext) GetPosition(data *core.DataSeries) *Position {
	return ctx.broker.GetPosition(data)
}

// GetCash returns available cash.
func (ctx *StrategyContext) GetCash() float64 { return ctx.broker.GetCash() }

// GetValue returns total portfolio value.
func (ctx *StrategyContext) GetValue() float64 { return ctx.broker.GetValue(ctx.Datas) }

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
	data := ctx.Data()
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
	order := newOrder(data, OrderSideBuy, cfg.ot, size, cfg.price, cfg.price2)
	ctx.broker.Submit(order)
	return order
}

// SellData creates a sell order on the given data series.
func (ctx *StrategyContext) SellData(data *core.DataSeries, size float64, opts ...OrderOption) *Order {
	cfg := defaultOrderCfg()
	for _, o := range opts {
		o(&cfg)
	}
	order := newOrder(data, OrderSideSell, cfg.ot, size, cfg.price, cfg.price2)
	ctx.broker.Submit(order)
	return order
}

// Cancel cancels a pending order.
func (ctx *StrategyContext) Cancel(order *Order) { ctx.broker.Cancel(order) }

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

// ─── Order configuration ───────────────────────────────────────────

type orderCfg struct {
	ot     OrderType
	price  float64
	price2 float64
}

func defaultOrderCfg() orderCfg {
	return orderCfg{ot: OrderTypeMarket}
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

// WithClose creates a close-of-bar order.
func WithClose() OrderOption {
	return func(c *orderCfg) { c.ot = OrderTypeClose }
}
