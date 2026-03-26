// Package engine contains the backtesting engine components.
// This file implements the simulated Broker, mirroring backtrader's bbroker.py
// with full feature parity: slippage, trailing stops, OCO, bracket orders,
// order expiry, margin checking, and dynamic cash management.
package engine

import (
	"fmt"
	"math"
	"time"

	"github.com/thangtmkafi/gobacktrader/core"
)

// ─── Slippage config ─────────────────────────────────────────────────────────

// SlippageMode selects which type of slippage to apply.
type SlippageMode int

const (
	SlippageNone    SlippageMode = iota
	SlippagePercent              // percentage of price
	SlippageFixed                // fixed tick amount
)

// ─── Broker ──────────────────────────────────────────────────────────────────

// Broker simulates a realistic backtesting broker with full feature parity to
// Backtrader's BackBroker (bbroker.py).
type Broker struct {
	// Cash management
	cash         float64
	startingCash float64
	addCashQueue []float64

	// Commission
	commission    CommissionInfo
	commInfoPerFeed map[string]CommissionInfo // per-asset commission override

	// Position tracking
	positions map[string]*Position // keyed by DataSeries.Name

	// Order queues
	pending    []*Order
	toActivate []*Order // children waiting for parent fill

	// Trade tracking
	Trades []*Trade

	// Notifications
	orderNotifications []*Order
	tradeNotifications []*Trade

	// ── Slippage ──
	slippageMode  SlippageMode
	slippageValue float64 // perc: 0.001 = 0.1%; fixed: 2.0 = $2 per share
	slipOpen      bool    // apply slippage to market/open-price orders
	slipLimit     bool    // apply slippage to limit orders
	slipMatch     bool    // cap slippage at bar high/low
	slipOut       bool    // allow slippage outside high/low bounds

	// ── Special modes ──
	coc       bool // Cheat-On-Close: fill market at same bar's close
	coo       bool // Cheat-On-Open: fill market at open before next bar
	shortCash bool // if true, short sales increase cash

	// ── OCO (One-Cancels-Other) ──
	// ocoGroups: canonical ref → []all refs in group
	ocoGroups map[int][]int
	// ocoParent: orderRef → canonical (group-leader) ref
	ocoParent map[int]int

	// ── Bracket / parent-child ──
	// childQueue: parentRef → slice of child orders held until parent fills
	childQueue map[int][]*Order
}

// NewBroker creates a Broker with the given starting cash.
func NewBroker(cash float64) *Broker {
	b := &Broker{
		cash:            cash,
		startingCash:    cash,
		positions:       make(map[string]*Position),
		commission:      ZeroCommission{},
		commInfoPerFeed: make(map[string]CommissionInfo),
		ocoGroups:       make(map[int][]int),
		ocoParent:       make(map[int]int),
		childQueue:      make(map[int][]*Order),
		slipMatch:       true, // default: cap slippage at bar bounds
		shortCash:       true, // default: short sales increase cash
	}
	return b
}

// ── Cash management ────────────────────────────────────────────────────────

// SetCash sets the current cash balance.
func (b *Broker) SetCash(cash float64) {
	b.startingCash = cash
	b.cash = cash
}

// GetCash returns current cash.
func (b *Broker) GetCash() float64 { return b.cash }

// AddCash queues a cash addition (or subtraction if negative) for the next bar.
func (b *Broker) AddCash(delta float64) { b.addCashQueue = append(b.addCashQueue, delta) }

// ── Commission ─────────────────────────────────────────────────────────────

// SetCommission sets the default commission scheme for all instruments.
func (b *Broker) SetCommission(c CommissionInfo) { b.commission = c }

// SetCommissionForFeed sets a per-feed commission scheme.
func (b *Broker) SetCommissionForFeed(feedName string, c CommissionInfo) {
	b.commInfoPerFeed[feedName] = c
}

// commFor returns the commission scheme for the given feed name.
func (b *Broker) commFor(feedName string) CommissionInfo {
	if c, ok := b.commInfoPerFeed[feedName]; ok {
		return c
	}
	return b.commission
}

// ── Slippage ───────────────────────────────────────────────────────────────

// SetSlippagePercent configures percentage-based slippage.
// perc: 0.001 = 0.1%.  slipOpen = apply to open-price orders.
func (b *Broker) SetSlippagePercent(perc float64, slipOpen, slipLimit, slipMatch, slipOut bool) {
	b.slippageMode = SlippagePercent
	b.slippageValue = perc
	b.slipOpen = slipOpen
	b.slipLimit = slipLimit
	b.slipMatch = slipMatch
	b.slipOut = slipOut
}

// SetSlippageFixed configures fixed-amount slippage (per share/contract).
func (b *Broker) SetSlippageFixed(fixed float64, slipOpen, slipLimit, slipMatch, slipOut bool) {
	b.slippageMode = SlippageFixed
	b.slippageValue = fixed
	b.slipOpen = slipOpen
	b.slipLimit = slipLimit
	b.slipMatch = slipMatch
	b.slipOut = slipOut
}

// applySlippage adjusts a fill price by the configured slippage.
// isBuy = true means price adverse = up; isBuy = false = price adverse = down.
// barHigh/barLow are the bar bounds used when slipMatch = true.
func (b *Broker) applySlippage(price, barHigh, barLow float64, isBuy bool) float64 {
	if b.slippageMode == SlippageNone {
		return price
	}
	var slip float64
	switch b.slippageMode {
	case SlippagePercent:
		slip = price * b.slippageValue
	case SlippageFixed:
		slip = b.slippageValue
	}
	if isBuy {
		price += slip
		if b.slipMatch && !b.slipOut && price > barHigh {
			price = barHigh
		}
	} else {
		price -= slip
		if b.slipMatch && !b.slipOut && price < barLow {
			price = barLow
		}
	}
	return price
}

// ── Special modes ──────────────────────────────────────────────────────────

func (b *Broker) SetCOC(coc bool) { b.coc = coc }
func (b *Broker) SetCOO(coo bool) { b.coo = coo }
func (b *Broker) SetShortCash(sc bool) { b.shortCash = sc }

// ── Portfolio value ────────────────────────────────────────────────────────

// GetValue returns total portfolio value: cash + open position values.
func (b *Broker) GetValue(datas []*core.DataSeries) float64 {
	total := b.cash
	for _, data := range datas {
		pos := b.positions[data.Name]
		if pos == nil || !pos.IsOpen() {
			continue
		}
		if data.Len() == 0 {
			continue
		}
		close := data.Close.Get(0)
		if pos.Size > 0 {
			total += pos.Size * close
		} else if b.shortCash {
			// Short positions: already got cash on open, subtract current cost
			total += pos.Size * close // negative size * positive price = negative adjustment
		}
	}
	return total
}

// ── Position ───────────────────────────────────────────────────────────────

// GetPosition returns (or creates) the position for the given data.
func (b *Broker) GetPosition(data *core.DataSeries) *Position {
	p := b.positions[data.Name]
	if p == nil {
		p = &Position{}
		b.positions[data.Name] = p
	}
	return p
}

// GetOrdersOpen returns a snapshot of all currently pending orders.
func (b *Broker) GetOrdersOpen() []*Order {
	result := make([]*Order, len(b.pending))
	copy(result, b.pending)
	return result
}

// ── Order submission ───────────────────────────────────────────────────────

// Submit queues an order. Handles OCO registration and bracket child holding.
func (b *Broker) Submit(order *Order) {
	// Set bar creation time from the order's data feed (used for DAY expiry)
	if order.Data != nil && order.Data.Len() > 0 {
		order.BarCreatedAt = order.Data.Bar().DateTime
	} else {
		order.BarCreatedAt = order.CreatedAt
	}
	// Register into OCO group if requested
	if order.OcoRef > 0 {
		b.registerOCO(order)
	}

	// Bracket child: hold until parent fills (don't add to pending yet)
	if order.ParentRef > 0 {
		b.childQueue[order.ParentRef] = append(b.childQueue[order.ParentRef], order)
		order.Status = OrderStatusSubmitted
		b.orderNotifications = append(b.orderNotifications, order)
		return
	}

	// For trailing stops: compute initial trail stop from current bar price
	b.initTrailStop(order)

	order.Status = OrderStatusSubmitted
	b.orderNotifications = append(b.orderNotifications, order)
	order.Status = OrderStatusAccepted
	b.pending = append(b.pending, order)
	b.orderNotifications = append(b.orderNotifications, order)
}

// registerOCO adds the order to its OCO group.
func (b *Broker) registerOCO(order *Order) {
	ref := order.OcoRef
	b.ocoParent[order.Ref] = ref
	b.ocoGroups[ref] = append(b.ocoGroups[ref], order.Ref)
}

// initTrailStop computes the initial trail stop price when an order is submitted.
func (b *Broker) initTrailStop(order *Order) {
	if order.Type != OrderTypeStopTrail && order.Type != OrderTypeStopTrailLimit {
		return
	}
	if order.Price != 0 {
		// Explicit initial stop price given
		order.TrailStop = order.Price
		return
	}
	// Use TrailAmount or TrailPercent relative to current price (data.Close[0])
	if order.Data == nil || order.Data.Len() == 0 {
		return
	}
	curPrice := order.Data.Close.Get(0)
	if order.TrailPercent > 0 {
		order.TrailAmount = curPrice * order.TrailPercent
	}
	if order.IsBuy() {
		order.TrailStop = curPrice + order.TrailAmount
	} else {
		order.TrailStop = curPrice - order.TrailAmount
	}
	order.Price = order.TrailStop
}

// Cancel cancels a pending order (and any bracket children).
func (b *Broker) Cancel(order *Order) {
	b.cancelOrder(order, true)
}

func (b *Broker) cancelOrder(order *Order, cancelChildren bool) {
	if order.IsCompleted() {
		return
	}
	order.Status = OrderStatusCanceled
	b.orderNotifications = append(b.orderNotifications, order)

	// Remove from pending
	for i, o := range b.pending {
		if o.Ref == order.Ref {
			b.pending = append(b.pending[:i], b.pending[i+1:]...)
			break
		}
	}

	// Cancel any bracket children
	if cancelChildren {
		if children, ok := b.childQueue[order.Ref]; ok {
			for _, child := range children {
				b.cancelOrder(child, true)
			}
			delete(b.childQueue, order.Ref)
		}
	}

	// Trigger OCO cancellation
	b.processOCO(order.Ref)
}

// ── Notifications ──────────────────────────────────────────────────────────

// DrainNotifications returns and clears pending order/trade notifications.
func (b *Broker) DrainNotifications() ([]*Order, []*Trade) {
	orders := b.orderNotifications
	trades := b.tradeNotifications
	b.orderNotifications = nil
	b.tradeNotifications = nil
	return orders, trades
}

// ── Main simulation loop ───────────────────────────────────────────────────

// Next processes all pending orders against the current bar data.
// Should be called once per bar, BEFORE strategy Next().
func (b *Broker) Next(datas []*core.DataSeries) {
	// 1. Apply queued cash additions
	for _, delta := range b.addCashQueue {
		b.cash += delta
	}
	b.addCashQueue = b.addCashQueue[:0]

	// 2. Activate bracket children that were released by parent fills
	for _, order := range b.toActivate {
		b.initTrailStop(order)
		order.Status = OrderStatusAccepted
		b.pending = append(b.pending, order)
		b.orderNotifications = append(b.orderNotifications, order)
	}
	b.toActivate = b.toActivate[:0]

	// 3. Process each pending order
	var remaining []*Order
	for _, order := range b.pending {
		if order.IsCompleted() {
			continue
		}

		data := order.Data
		if data == nil || data.Len() == 0 {
			remaining = append(remaining, order)
			continue
		}
		barTime := data.Bar().DateTime

		// 3a. Check expiry
		if b.checkExpiry(order, barTime) {
			// order was expired and notified; don't keep in pending
			continue
		}

		bar := data.Bar()

		// 3b. Update trailing stop if applicable
		b.updateTrailStop(order, bar)

		// 3c. Try to fill
		filled, fillPrice := b.tryFill(order, bar)
		if !filled {
			remaining = append(remaining, order)
			continue
		}

		// 3d. Apply slippage
		isMarketFill := (order.Type == OrderTypeMarket || order.Type == OrderTypeClose)
		if isMarketFill && b.slipOpen {
			fillPrice = b.applySlippage(fillPrice, bar.High, bar.Low, order.IsBuy())
		} else if order.Type == OrderTypeLimit && b.slipLimit {
			fillPrice = b.applySlippage(fillPrice, bar.High, bar.Low, order.IsBuy())
		} else if !isMarketFill && order.Type != OrderTypeLimit {
			// Stop/StopTrail fills
			fillPrice = b.applySlippage(fillPrice, bar.High, bar.Low, order.IsBuy())
		}

		// 3e. Execute fill
		b.executeFill(order, fillPrice, order.RemSize(), barTime, datas)
	}
	b.pending = remaining
}

// tryFill attempts to fill an order against the current bar.
// Returns (filled, fillPrice).
func (b *Broker) tryFill(order *Order, bar core.Bar) (bool, float64) {
	switch order.Type {
	case OrderTypeMarket:
		if b.coo {
			return true, bar.Open // Cheat-On-Open: fill at open of current bar
		}
		return true, bar.Open

	case OrderTypeClose:
		if b.coc {
			return true, bar.Close // Cheat-On-Close: fill at close of current bar
		}
		return true, bar.Close

	case OrderTypeLimit:
		if order.IsBuy() && bar.Low <= order.Price {
			return true, order.Price
		} else if !order.IsBuy() && bar.High >= order.Price {
			return true, order.Price
		}

	case OrderTypeStop:
		if order.IsBuy() && bar.High >= order.Price {
			return true, bar.Open // stop triggered → market fill
		} else if !order.IsBuy() && bar.Low <= order.Price {
			return true, bar.Open
		}

	case OrderTypeStopLimit:
		stopTriggered := false
		if order.IsBuy() && bar.High >= order.Price {
			stopTriggered = true
		} else if !order.IsBuy() && bar.Low <= order.Price {
			stopTriggered = true
		}
		if stopTriggered {
			limitPrice := order.Price2
			if order.IsBuy() && bar.Low <= limitPrice {
				return true, limitPrice
			} else if !order.IsBuy() && bar.High >= limitPrice {
				return true, limitPrice
			}
		}

	case OrderTypeStopTrail:
		if order.IsBuy() && bar.High >= order.TrailStop {
			return true, bar.Open
		} else if !order.IsBuy() && bar.Low <= order.TrailStop {
			return true, bar.Open
		}

	case OrderTypeStopTrailLimit:
		triggered := false
		if order.IsBuy() && bar.High >= order.TrailStop {
			triggered = true
		} else if !order.IsBuy() && bar.Low <= order.TrailStop {
			triggered = true
		}
		if triggered {
			// Price2 is limit offset from TrailStop
			limitPrice := order.TrailStop + order.Price2
			if order.IsBuy() && bar.Low <= limitPrice {
				return true, limitPrice
			} else if !order.IsBuy() && bar.High >= limitPrice {
				return true, limitPrice
			}
		}
	}
	return false, 0
}

// updateTrailStop adjusts trailing stop price when price moves favorably.
func (b *Broker) updateTrailStop(order *Order, bar core.Bar) {
	if order.Type != OrderTypeStopTrail && order.Type != OrderTypeStopTrailLimit {
		return
	}
	amount := order.TrailAmount
	if order.IsBuy() {
		// For a sell side trailing stop (protecting a long): trail below price
		// Move stop UP when price rises
		newStop := bar.Close - amount
		if newStop > order.TrailStop {
			order.TrailStop = newStop
		}
	} else {
		// For a buy side trailing stop (protecting a short): trail above price
		// Move stop DOWN when price falls
		newStop := bar.Close + amount
		if newStop < order.TrailStop {
			order.TrailStop = newStop
		}
	}
}

// checkExpiry checks if an order has expired; notifies and returns true if so.
func (b *Broker) checkExpiry(order *Order, barTime time.Time) bool {
	switch order.ValidType {
	case ValidDAY:
		// DAY orders expire on any bar whose date is AFTER the creation bar's date
		creationDay := order.BarCreatedAt.Truncate(24 * time.Hour)
		barDay := barTime.Truncate(24 * time.Hour)
		if barDay.After(creationDay) {
			order.Status = OrderStatusExpired
			b.orderNotifications = append(b.orderNotifications, order)
			return true
		}
	case ValidGTD:
		if !order.ValidTime.IsZero() && barTime.After(order.ValidTime) {
			order.Status = OrderStatusExpired
			b.orderNotifications = append(b.orderNotifications, order)
			return true
		}
	}
	return false
}

// executeFill processes a fill for the given order.
func (b *Broker) executeFill(order *Order, fillPrice, fillSize float64, barTime time.Time, datas []*core.DataSeries) {
	comm := b.commFor(order.Data.Name).GetCommission(fillSize, fillPrice)
	cost := fillSize * fillPrice

	// Margin / cash check
	if order.IsBuy() {
		required := cost + comm
		if required > b.cash {
			order.Status = OrderStatusMargin
			b.orderNotifications = append(b.orderNotifications, order)
			return
		}
		b.cash -= required
	} else {
		// Sell: receive cash
		if b.shortCash {
			b.cash += cost - comm
		} else {
			b.cash -= comm
		}
	}

	// Record position update and compute PnL
	pos := b.GetPosition(order.Data)
	signedSize := fillSize
	if !order.IsBuy() {
		signedSize = -fillSize
	}
	pnl := pos.Update(signedSize, fillPrice)

	// Record fill on order
	order.recordFill(barTime, fillSize, fillPrice, cost, comm, pnl)
	order.ExecutedAt = barTime

	if order.RemSize() <= 1e-10 {
		order.Status = OrderStatusCompleted
	} else {
		order.Status = OrderStatusPartial
	}
	b.orderNotifications = append(b.orderNotifications, order)

	// Update trade tracking
	b.updatePositionAndTrades(order, pos, signedSize, fillPrice, comm)

	// Trigger OCO cancellation (cancel sibling orders)
	if order.OcoRef > 0 {
		b.processOCO(order.Ref)
	}

	// Activate bracket children on parent fill
	if children, ok := b.childQueue[order.Ref]; ok && order.Status == OrderStatusCompleted {
		b.toActivate = append(b.toActivate, children...)
		delete(b.childQueue, order.Ref)
	}
}

// processOCO cancels all other orders in the same OCO group as triggerRef.
func (b *Broker) processOCO(triggerRef int) {
	canonRef, ok := b.ocoParent[triggerRef]
	if !ok {
		return
	}
	group := b.ocoGroups[canonRef]
	for _, ref := range group {
		if ref == triggerRef {
			continue
		}
		// Find and cancel this order in pending
		for _, o := range b.pending {
			if o.Ref == ref && !o.IsCompleted() {
				o.Status = OrderStatusCanceled
				b.orderNotifications = append(b.orderNotifications, o)
			}
		}
	}
	// Clean up OCO tracking
	for _, ref := range group {
		delete(b.ocoParent, ref)
	}
	delete(b.ocoGroups, canonRef)

	// Remove cancelled from pending
	var remaining []*Order
	for _, o := range b.pending {
		if o.Status != OrderStatusCanceled {
			remaining = append(remaining, o)
		}
	}
	b.pending = remaining
}

// updatePositionAndTrades manages Trade objects for open/close events.
func (b *Broker) updatePositionAndTrades(order *Order, pos *Position, signedSize, price, comm float64) {
	wasOpen := pos.IsOpen()

	if !wasOpen {
		// Trade just got closed — find it and mark closed
		for i := len(b.Trades) - 1; i >= 0; i-- {
			t := b.Trades[i]
			if t.IsOpen && t.DataName == order.Data.Name {
				t.close(order, price, comm)
				b.tradeNotifications = append(b.tradeNotifications, t)
				break
			}
		}
	} else if pos.IsOpen() && math.Abs(signedSize) > 0 {
		// Check if we just opened a new trade
		isNewTrade := true
		for _, t := range b.Trades {
			if t.IsOpen && t.DataName == order.Data.Name {
				isNewTrade = false
				b.tradeNotifications = append(b.tradeNotifications, t)
				break
			}
		}
		if isNewTrade {
			absSize := math.Abs(signedSize)
			t := newTrade(order, absSize, price, comm)
			b.Trades = append(b.Trades, t)
			b.tradeNotifications = append(b.tradeNotifications, t)
		}
	}
}

// ── Diagnostics ──────────────────────────────────────────────────────────────

func (b *Broker) GetTrades() []*Trade { return b.Trades }

// PendingOrders returns currently pending orders.
func (b *Broker) PendingOrders() []*Order { return b.pending }

// String returns a summary of the broker state.
func (b *Broker) String() string {
	return fmt.Sprintf("Broker[cash=%.2f, pending=%d, trades=%d]",
		b.cash, len(b.pending), len(b.Trades))
}
