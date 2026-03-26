// Package livebrokers provides live and paper trading broker implementations
// for gobacktrader. It defines the BrokerBase interface that allows plugging in
// any order routing backend — from paper trading (LogRouter) to real exchange
// connections (RESTRouter or custom implementations).
//
// Architecture:
//
//	LiveBroker (implements engine.BrokerBase)
//	    └── OrderRouter (interface)
//	            ├── LogRouter      — prints orders to stdout (paper trading)
//	            ├── RESTRouter     — sends orders to a REST endpoint
//	            └── (your own)    — IBBroker, Oanda, Binance, etc.
package livebrokers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/thangtmkafi/gobacktrader/core"
	"github.com/thangtmkafi/gobacktrader/engine"
)

// (BrokerBase is now defined in the engine package)

// ─── OrderRouter interface ────────────────────────────────────────────────────

// OrderRouter is the pluggable backend that actually sends orders to an exchange.
// Implement this interface to connect gobacktrader to any live brokerage.
type OrderRouter interface {
	// PlaceOrder sends a buy/sell order to the exchange.
	// Returns the exchange-assigned order ID.
	PlaceOrder(ctx context.Context, order *engine.Order) (string, error)

	// CancelOrder cancels an order by its exchange reference.
	CancelOrder(ctx context.Context, exchangeRef string) error

	// GetPositions fetches current live positions from the exchange.
	GetPositions(ctx context.Context) ([]LivePosition, error)

	// GetCash fetches current cash/balance from the exchange.
	GetCash(ctx context.Context) (float64, error)
}

// LivePosition represents a position returned from a live exchange.
type LivePosition struct {
	Symbol string
	Size   float64
	Price  float64
}

// ─── LiveBroker ───────────────────────────────────────────────────────────────

// LiveBroker wraps an engine.Broker (for simulation-side tracking) and routes
// all buy/sell orders through an OrderRouter to a real or paper exchange.
//
// Usage:
//
//	router := livebrokers.NewLogRouter()
//	lb := livebrokers.NewLiveBroker(router, 100_000)
//	cerebro.SetBroker(lb.Broker()) // use the underlying sim broker for equity tracking
type LiveBroker struct {
	broker *engine.Broker
	router OrderRouter
	refs   map[int]string // orderRef → exchangeRef
	ctx    context.Context
}

// NewLiveBroker creates a LiveBroker backed by the given OrderRouter.
// startingCash is used for the internal simulation broker (position/equity tracking).
func NewLiveBroker(router OrderRouter, startingCash float64) *LiveBroker {
	return &LiveBroker{
		broker: engine.NewBroker(startingCash),
		router: router,
		refs:   make(map[int]string),
		ctx:    context.Background(),
	}
}

// Broker returns the underlying simulation broker (for use with Cerebro.SetBroker).
func (lb *LiveBroker) Broker() *engine.Broker { return lb.broker }

// SetContext sets the context for all order router calls.
func (lb *LiveBroker) SetContext(ctx context.Context) { lb.ctx = ctx }

// ─── BrokerBase Wrapper Methods ──────────────────────────────────────────────

func (lb *LiveBroker) GetCash() float64 { return lb.broker.GetCash() }
func (lb *LiveBroker) GetValue(datas []*core.DataSeries) float64 { return lb.broker.GetValue(datas) }
func (lb *LiveBroker) GetPosition(data *core.DataSeries) *engine.Position { return lb.broker.GetPosition(data) }
func (lb *LiveBroker) GetOrdersOpen() []*engine.Order { return lb.broker.GetOrdersOpen() }
func (lb *LiveBroker) Next(datas []*core.DataSeries) { lb.broker.Next(datas) }
func (lb *LiveBroker) DrainNotifications() ([]*engine.Order, []*engine.Trade) { return lb.broker.DrainNotifications() }
func (lb *LiveBroker) AddCash(delta float64) { lb.broker.AddCash(delta) }
func (lb *LiveBroker) SetCash(cash float64) { lb.broker.SetCash(cash) }
func (lb *LiveBroker) SetCommission(comm engine.CommissionInfo) { lb.broker.SetCommission(comm) }
func (lb *LiveBroker) GetTrades() []*engine.Trade { return lb.broker.GetTrades() }

// Submit queues an order for simulation tracker and sends it to the real exchange.
func (lb *LiveBroker) Submit(order *engine.Order) {
	lb.broker.Submit(order)
	if order.Status == engine.OrderStatusAccepted || order.Status == engine.OrderStatusSubmitted {
		exchangeRef, err := lb.router.PlaceOrder(lb.ctx, order)
		if err != nil {
			fmt.Printf("[LiveBroker] Failed to place order %d: %v\n", order.Ref, err)
			lb.broker.Cancel(order)
			return
		}
		lb.refs[order.Ref] = exchangeRef
	}
}

// Cancel cancels an order both locally and on the exchange.
func (lb *LiveBroker) Cancel(order *engine.Order) {
	exchangeRef, ok := lb.refs[order.Ref]
	if ok {
		if err := lb.router.CancelOrder(lb.ctx, exchangeRef); err != nil {
			fmt.Printf("[LiveBroker] Failed to cancel order %d: %v\n", order.Ref, err)
		}
		delete(lb.refs, order.Ref)
	}
	lb.broker.Cancel(order)
}

// SyncPositions fetches live positions from the exchange and reconciles them
// with the internal simulation state. Call this on startup or after reconnect.
func (lb *LiveBroker) SyncPositions() error {
	positions, err := lb.router.GetPositions(lb.ctx)
	if err != nil {
		return fmt.Errorf("liveBroker: GetPositions failed: %w", err)
	}
	for _, p := range positions {
		fmt.Printf("[LiveBroker] Synced position: %s size=%.4f price=%.4f\n",
			p.Symbol, p.Size, p.Price)
	}
	cash, err := lb.router.GetCash(lb.ctx)
	if err != nil {
		return fmt.Errorf("liveBroker: GetCash failed: %w", err)
	}
	lb.broker.SetCash(cash)
	return nil
}

// ─── LogRouter ────────────────────────────────────────────────────────────────

// LogRouter is a paper-trading OrderRouter that prints orders to stdout.
// It always succeeds and generates sequential fake exchange IDs.
// Ideal for testing strategies without a real broker connection.
type LogRouter struct {
	nextID int
	log    io.Writer
}

// NewLogRouter creates a LogRouter that prints to stdout.
func NewLogRouter() *LogRouter {
	return &LogRouter{log: nil}
}

// NewLogRouterTo creates a LogRouter that writes to a custom writer.
func NewLogRouterTo(w io.Writer) *LogRouter {
	return &LogRouter{log: w}
}

func (r *LogRouter) printf(format string, args ...any) {
	msg := fmt.Sprintf("[%s] "+format+"\n", append([]any{time.Now().Format("15:04:05.000")}, args...)...)
	if r.log != nil {
		_, _ = fmt.Fprint(r.log, msg)
	} else {
		fmt.Print(msg)
	}
}

func (r *LogRouter) PlaceOrder(_ context.Context, order *engine.Order) (string, error) {
	r.nextID++
	ref := fmt.Sprintf("PAPER-%06d", r.nextID)
	r.printf("ORDER PLACED: %s | ref=%s | side=%s | size=%.4g | price=%.4f | type=%s",
		order.Data.Name, ref, order.Side, order.Size, order.Price, order.Type)
	return ref, nil
}

func (r *LogRouter) CancelOrder(_ context.Context, exchangeRef string) error {
	r.printf("ORDER CANCELLED: ref=%s", exchangeRef)
	return nil
}

func (r *LogRouter) GetPositions(_ context.Context) ([]LivePosition, error) {
	r.printf("GET_POSITIONS: (paper trading — no live positions)")
	return nil, nil
}

func (r *LogRouter) GetCash(_ context.Context) (float64, error) {
	r.printf("GET_CASH: (paper trading — using simulated cash)")
	return 0, nil
}

// ─── RESTRouter ───────────────────────────────────────────────────────────────

// RESTRouterConfig configures the RESTRouter.
type RESTRouterConfig struct {
	BaseURL     string        // e.g. "https://api.mybrokerage.com/v1"
	AuthToken   string        // Bearer token
	Timeout     time.Duration // HTTP timeout (default 10s)
	OrderPath   string        // e.g. "/orders"
	CancelPath  string        // e.g. "/orders/{ref}/cancel"
	PositionPath string       // e.g. "/positions"
	CashPath    string        // e.g. "/account/cash"
}

// RESTOrderRequest is the JSON body sent to the exchange for order placement.
type RESTOrderRequest struct {
	Symbol    string  `json:"symbol"`
	Side      string  `json:"side"`
	Size      float64 `json:"size"`
	Price     float64 `json:"price,omitempty"`
	OrderType string  `json:"order_type"`
}

// RESTOrderResponse is the JSON response from the exchange.
type RESTOrderResponse struct {
	Ref    string `json:"ref"`
	Status string `json:"status"`
}

// RESTRouter routes orders to a configurable REST API endpoint.
// Implement your own endpoint or adapter on the server side.
type RESTRouter struct {
	cfg    RESTRouterConfig
	client *http.Client
}

// NewRESTRouter creates a RESTRouter with the given configuration.
func NewRESTRouter(cfg RESTRouterConfig) *RESTRouter {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &RESTRouter{
		cfg:    cfg,
		client: &http.Client{Timeout: timeout},
	}
}

func (r *RESTRouter) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, r.cfg.BaseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.cfg.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.cfg.AuthToken)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("RESTRouter: HTTP %d: %s", resp.StatusCode, respBody)
	}
	return respBody, nil
}

func (r *RESTRouter) PlaceOrder(ctx context.Context, order *engine.Order) (string, error) {
	req := RESTOrderRequest{
		Symbol:    order.Data.Name,
		Side:      order.Side.String(),
		Size:      order.Size,
		Price:     order.Price,
		OrderType: order.Type.String(),
	}
	path := r.cfg.OrderPath
	if path == "" {
		path = "/orders"
	}
	data, err := r.do(ctx, http.MethodPost, path, req)
	if err != nil {
		return "", err
	}
	var resp RESTOrderResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("RESTRouter: parse response: %w", err)
	}
	return resp.Ref, nil
}

func (r *RESTRouter) CancelOrder(ctx context.Context, exchangeRef string) error {
	path := fmt.Sprintf("%s/%s/cancel", r.cfg.CancelPath, exchangeRef)
	if r.cfg.CancelPath == "" {
		path = fmt.Sprintf("/orders/%s/cancel", exchangeRef)
	}
	_, err := r.do(ctx, http.MethodPost, path, nil)
	return err
}

func (r *RESTRouter) GetPositions(ctx context.Context) ([]LivePosition, error) {
	path := r.cfg.PositionPath
	if path == "" {
		path = "/positions"
	}
	data, err := r.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var positions []LivePosition
	if err := json.Unmarshal(data, &positions); err != nil {
		return nil, fmt.Errorf("RESTRouter: parse positions: %w", err)
	}
	return positions, nil
}

func (r *RESTRouter) GetCash(ctx context.Context) (float64, error) {
	path := r.cfg.CashPath
	if path == "" {
		path = "/account/cash"
	}
	data, err := r.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return 0, err
	}
	var result struct {
		Cash float64 `json:"cash"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return 0, fmt.Errorf("RESTRouter: parse cash: %w", err)
	}
	return result.Cash, nil
}
