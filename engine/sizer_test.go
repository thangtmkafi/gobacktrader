package engine

import (
	"testing"

	"github.com/thangtmkafi/gobacktrader/core"
)

func TestFixedSizer(t *testing.T) {
	s := FixedSizer{Amount: 25}
	got := s.Size(nil, nil)
	if got != 25 {
		t.Errorf("FixedSizer.Size() = %f, want 25", got)
	}
}

func TestPercentSizer(t *testing.T) {
	// Build a minimal ctx with a broker worth $10,000
	broker := NewBroker(10_000)
	ds := core.NewDataSeries("test")
	ds.Forward()
	ds.AppendBar(core.Bar{Close: 100})
	ctx := &StrategyContext{
		Datas:  []*core.DataSeries{ds},
		broker: broker,
	}

	s := PercentSizer{Percent: 0.10} // 10% of $10,000 = $1,000 / $100 = 10 shares
	got := s.Size(ctx, ds)
	if got != 10 {
		t.Errorf("PercentSizer.Size() = %f, want 10", got)
	}
}

func TestAllInSizer(t *testing.T) {
	broker := NewBroker(5_000)
	ds := core.NewDataSeries("test")
	ds.Forward()
	ds.AppendBar(core.Bar{Close: 50})
	ctx := &StrategyContext{
		Datas:  []*core.DataSeries{ds},
		broker: broker,
	}

	s := AllInSizer{}
	got := s.Size(ctx, ds) // $5,000 / $50 = 100 shares
	if got != 100 {
		t.Errorf("AllInSizer.Size() = %f, want 100", got)
	}
}
