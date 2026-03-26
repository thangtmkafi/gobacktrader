package engine_test

import (
	"testing"
	"time"

	"github.com/thangtmkafi/gobacktrader/core"
	"github.com/thangtmkafi/gobacktrader/engine"
)

// dummyFeed implements engine.PreloadedDataFeed
type dummyFeed struct {
	data      *core.DataSeries
	preloaded *core.DataSeries
	bars      []core.Bar
	pos       int
}

func (f *dummyFeed) Load() error { return nil }
func (f *dummyFeed) Next() bool {
	if f.pos >= len(f.bars) {
		return false
	}
	f.data.Forward()
	f.data.AppendBar(f.bars[f.pos])
	f.pos++
	return true
}
func (f *dummyFeed) Data() *core.DataSeries          { return f.data }
func (f *dummyFeed) PreloadedData() *core.DataSeries { return f.data }

type timerStrategy struct {
	ctx          *engine.StrategyContext
	timerFiredAt time.Time
	targetTime   time.Time
}

func (s *timerStrategy) Init(ctx *engine.StrategyContext) {
	s.ctx = ctx
	s.targetTime = time.Date(2023, 1, 3, 0, 0, 0, 0, time.UTC)
	ctx.AddTimer(s.targetTime)
}

func (s *timerStrategy) Next() {}

func (s *timerStrategy) NotifyTimer(timer *engine.Timer, t time.Time) {
	s.timerFiredAt = t
}

func TestStrategyTimer(t *testing.T) {
	c := engine.NewCerebro()

	dates := []time.Time{
		time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2023, 1, 3, 0, 0, 0, 0, time.UTC), // Target
		time.Date(2023, 1, 4, 0, 0, 0, 0, time.UTC),
	}
	bars := []core.Bar{}
	series := core.NewDataSeries("test")
	
	// Pre-populate preloaded series
	for i, dt := range dates {
		b := core.Bar{
			DateTime:     dt,
			Close:        float64(100 + i),
			Open:         float64(100 + i),
			High:         float64(100 + i),
			Low:          float64(100 + i),
			Volume:       100,
			OpenInterest: 0,
		}
		bars = append(bars, b)
		series.Forward()
		series.AppendBar(b)
	}

	liveSeries := core.NewDataSeries("test")
	feed := &dummyFeed{data: liveSeries, bars: bars, preloaded: series}
	c.AddData(feed)

	strat := &timerStrategy{}
	c.AddStrategy(func() engine.Strategy { return strat })

	_, err := c.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if strat.timerFiredAt.IsZero() {
		t.Fatal("Timer did not fire")
	}
	if !strat.timerFiredAt.Equal(dates[2]) {
		t.Fatalf("Expected timer to fire at %v, got %v", dates[2], strat.timerFiredAt)
	}
}
