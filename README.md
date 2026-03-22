<p align="center">
  <h1 align="center">🚀 gobacktrader</h1>
  <p align="center">
    <strong>A powerful algorithmic trading backtesting framework for Go</strong><br>
    Inspired by <a href="https://www.backtrader.com">Backtrader</a> (Python) · Built for performance · Go-native idioms
  </p>
  <p align="center">
    <a href="#features">Features</a> ·
    <a href="#quick-start">Quick Start</a> ·
    <a href="#indicators">Indicators</a> ·
    <a href="#analyzers">Analyzers</a> ·
    <a href="#live-feeds">Live Feeds</a> ·
    <a href="#examples">Examples</a>
  </p>
</p>

---

**gobacktrader** is a Go port of the popular [Backtrader](https://www.backtrader.com) Python framework. It provides a complete ecosystem for developing, backtesting, and paper-trading algorithmic trading strategies — all in pure Go with zero CGo dependencies.

```go
c := engine.NewCerebro()
c.SetCash(10_000)
c.AddCSV(feeds.DefaultYahooConfig("data.csv"))
c.AddStrategy(func() engine.Strategy { return &MyStrategy{} })
results, _ := c.Run()
```

## Features

| Category | What you get |
|----------|-------------|
| **Core Engine** | Cerebro orchestrator, simulated Broker, Order management (Market/Limit/Stop/StopLimit), Position tracking, Trade PnL |
| **Data Feeds** | CSV (Yahoo Finance format), WebSocket, REST API, NATS, Redis Streams, Kafka |
| **10 Indicators** | SMA, EMA, WMA, DEMA, TEMA, RSI, MACD, Stochastic, ATR, Bollinger Bands |
| **5 Analyzers** | Returns, Sharpe Ratio, Max Drawdown, Trade Analyzer, SQN (Van Tharp) |
| **Position Sizing** | Fixed, Percent-of-portfolio, All-in sizers |
| **Live Trading** | `RunLive(ctx)` for paper/live trading with real-time data feeds |
| **Testing** | 39 tests across 7 packages — all passing |

## Installation

```bash
go get github.com/thangtmkafi/gobacktrader
```

Requires **Go 1.21+**.

## Quick Start

### 1. Create your strategy

```go
package main

import (
    "math"
    "github.com/thangtmkafi/gobacktrader/engine"
    "github.com/thangtmkafi/gobacktrader/indicators"
)

type SMACross struct {
    ctx     *engine.StrategyContext
    fast    *indicators.SMA
    slow    *indicators.SMA
    pending *engine.Order
}

func (s *SMACross) Init(ctx *engine.StrategyContext) {
    s.ctx = ctx
    // Build indicators from PreloadedData (all bars available)
    s.fast = indicators.NewSMA(ctx.PreloadedData(), 10)
    s.slow = indicators.NewSMA(ctx.PreloadedData(), 30)
}

func (s *SMACross) Next() {
    if s.pending != nil {
        return
    }
    fast := s.fast.Line().Get(0)
    slow := s.slow.Line().Get(0)
    fastPrev := s.fast.Line().Get(-1)
    slowPrev := s.slow.Line().Get(-1)

    if math.IsNaN(fast) || math.IsNaN(slow) || math.IsNaN(fastPrev) || math.IsNaN(slowPrev) {
        return
    }

    pos := s.ctx.GetPosition(s.ctx.Data())
    if !pos.IsOpen() && fastPrev <= slowPrev && fast > slow {
        s.pending = s.ctx.Buy(10)  // buy 10 shares
    } else if pos.IsOpen() && fastPrev >= slowPrev && fast < slow {
        s.pending = s.ctx.Close()  // close position
    }
}

func (s *SMACross) NotifyOrder(o *engine.Order) {
    if o.IsCompleted() {
        s.pending = nil
    }
}
```

### 2. Run the backtest

```go
package main

import (
    "fmt"
    "github.com/thangtmkafi/gobacktrader/analyzers"
    "github.com/thangtmkafi/gobacktrader/engine"
    "github.com/thangtmkafi/gobacktrader/feeds"
)

func main() {
    c := engine.NewCerebro()
    c.SetCash(100_000)
    c.SetCommission(engine.PercentCommission{Percent: 0.001})
    c.AddCSV(feeds.DefaultYahooConfig("AAPL.csv"))
    c.AddStrategy(func() engine.Strategy { return &SMACross{} })

    // Attach analyzers
    c.AddAnalyzer(&analyzers.Returns{})
    c.AddAnalyzer(&analyzers.SharpeRatio{AnnualisationFactor: 252})
    c.AddAnalyzer(&analyzers.DrawDown{})
    c.AddAnalyzer(&analyzers.TradeAnalyzer{})
    c.AddAnalyzer(&analyzers.SQN{})

    results, err := c.Run()
    if err != nil {
        panic(err)
    }

    r := results[0]
    fmt.Printf("Final Value: $%.2f\n", r.FinalValue)
    fmt.Printf("Trades: %d\n", len(r.Trades))
}
```

## Indicators

Build indicators in `Init()` using `ctx.PreloadedData()`, then read values in `Next()`:

```go
func (s *MyStrategy) Init(ctx *engine.StrategyContext) {
    data := ctx.PreloadedData()
    s.rsi  = indicators.NewRSI(data, 14)
    s.macd = indicators.NewMACD(data, 12, 26, 9)
    s.bb   = indicators.NewBollingerBands(data, 20, 2.0)
    s.atr  = indicators.NewATR(data, 14)
}

func (s *MyStrategy) Next() {
    rsi   := s.rsi.Line().Get(0)       // current RSI
    macd  := s.macd.Line().Get(0)      // MACD line
    signal := s.macd.Signal().Get(0)   // signal line
    upper := s.bb.Upper().Get(0)       // upper Bollinger band
    atr   := s.atr.Line().Get(0)       // current ATR
}
```

| Indicator | Constructor | Lines |
|-----------|-----------|-------|
| **SMA** | `NewSMA(data, period)` | `.Line()` |
| **EMA** | `NewEMA(data, period)` | `.Line()` |
| **WMA** | `NewWMA(data, period)` | `.Line()` |
| **DEMA** | `NewDEMA(data, period)` | `.Line()` |
| **TEMA** | `NewTEMA(data, period)` | `.Line()` |
| **RSI** | `NewRSI(data, period)` | `.Line()` (0–100) |
| **MACD** | `NewMACD(data, fast, slow, signal)` | `.Line()` `.Signal()` `.Histogram()` |
| **Stochastic** | `NewStochastic(data, k, d)` | `.K()` `.D()` |
| **ATR** | `NewATR(data, period)` | `.Line()` |
| **Bollinger** | `NewBollingerBands(data, period, devs)` | `.Mid()` `.Upper()` `.Lower()` |

## Analyzers

Attach to Cerebro before `Run()` — they compute automatically:

```go
ret    := &analyzers.Returns{}
sharpe := &analyzers.SharpeRatio{AnnualisationFactor: 252}
dd     := &analyzers.DrawDown{}
ta     := &analyzers.TradeAnalyzer{}
sqn    := &analyzers.SQN{}

c.AddAnalyzer(ret)
c.AddAnalyzer(sharpe)
c.AddAnalyzer(dd)
c.AddAnalyzer(ta)
c.AddAnalyzer(sqn)

c.Run()

ret.Print()    // Total return %, PnL, best/worst trade
sharpe.Print() // Annualised Sharpe Ratio
dd.Print()     // Max drawdown %, duration
ta.Print()     // Win rate, profit factor, consec wins/losses
sqn.Print()    // System Quality Number + grade
```

## Live Feeds

Switch from backtesting to paper trading with one line change — `RunLive(ctx)`:

```go
// WebSocket
feed := livefeeds.NewWebSocketFeed(livefeeds.WebSocketConfig{
    URL:    "wss://stream.example.com/bars",
    Symbol: "BTCUSDT",
})

// REST API polling
feed := livefeeds.NewRESTFeed(livefeeds.RESTConfig{
    URL:          "https://api.example.com/bars/AAPL",
    PollInterval: 1 * time.Minute,
    ParseFunc:    myParser,
})

// NATS
feed := livefeeds.NewNATSFeed(livefeeds.NATSConfig{
    URL:     "nats://localhost:4222",
    Subject: "market.bars.AAPL",
})

// Redis Streams
feed := livefeeds.NewRedisFeed(livefeeds.RedisConfig{
    Addr:   "localhost:6379",
    Stream: "bars:AAPL",
})

// Kafka
feed := livefeeds.NewKafkaFeed(livefeeds.KafkaConfig{
    Brokers: []string{"localhost:9092"},
    Topic:   "market-bars",
})

c.AddData(feed)
c.RunLive(ctx)  // blocks until ctx is cancelled
```

## Examples

```bash
# Backtest SMA crossover
go run ./examples/sma_cross/

# Live demo (self-contained WebSocket replay)
go run ./examples/live_demo/
```

## Project Structure

```
gobacktrader/
├── core/           # Line, DataSeries — fundamental data structures
├── feeds/          # CSV data feed (Yahoo Finance format)
├── engine/         # Cerebro, Broker, Order, Position, Trade, Strategy, Sizer
├── indicators/     # 10 technical indicators
├── analyzers/      # 5 performance analyzers
├── livefeeds/      # WebSocket, REST, NATS, Redis, Kafka live feeds
├── examples/
│   ├── sma_cross/  # Backtest example
│   └── live_demo/  # Live trading demo
└── testdata/       # Sample CSV data
```

## Architecture

```
┌──────────────────────────────────────────┐
│                 Cerebro                   │
│  ┌──────────┐  ┌──────────┐  ┌────────┐ │
│  │ DataFeed │──│  Broker  │──│Strategy│ │
│  │ CSV/Live │  │  Orders  │  │  Init  │ │
│  │          │  │ Position │  │  Next  │ │
│  └──────────┘  └──────────┘  └────────┘ │
│       ↓             ↓            ↑       │
│  ┌──────────┐  ┌──────────┐  ┌────────┐ │
│  │Indicators│  │ Analyzers│  │ Sizers │ │
│  └──────────┘  └──────────┘  └────────┘ │
└──────────────────────────────────────────┘
```

## Testing

```bash
go test ./...
# 39 tests across 7 packages
```

## Backtrader Comparison

| Feature | Backtrader (Python) | gobacktrader (Go) |
|---------|-------------------|-------------------|
| Language | Python 3 | Go 1.21+ |
| Indicators | 100+ | 10 (core set) |
| Data feeds | CSV, IB, Oanda | CSV, WS, REST, NATS, Redis, Kafka |
| Live trading | Yes (IB, Oanda) | Paper trading (simulated broker) |
| Performance | ~1x | ~10-50x faster |
| Concurrency | GIL-limited | Native goroutines |

## License

MIT

## Contributing

Contributions welcome! Please open an issue first to discuss what you'd like to change.

---

<p align="center">
  Built with ❤️ in Go · Inspired by <a href="https://www.backtrader.com">Backtrader</a>
</p>
