package engine

import "github.com/thangtmkafi/gobacktrader/core"

// BrokerBase is the interface for all broker implementations (simulated and live).
type BrokerBase interface {
	// Portfolio state
	GetCash() float64
	GetValue(datas []*core.DataSeries) float64
	GetPosition(data *core.DataSeries) *Position
	GetOrdersOpen() []*Order

	// Order management
	Submit(order *Order)
	Cancel(order *Order)

	// Bar processing (called once per bar)
	Next(datas []*core.DataSeries)

	// Notification drain
	DrainNotifications() ([]*Order, []*Trade)

	// Dynamic cash
	AddCash(delta float64)
	SetCash(cash float64)
	SetCommission(comm CommissionInfo)
	GetTrades() []*Trade
}
