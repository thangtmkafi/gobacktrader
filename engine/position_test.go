package engine

import (
	"math"
	"testing"
)

func TestPositionOpenLong(t *testing.T) {
	p := &Position{}
	pnl := p.Update(100, 50.0) // buy 100 shares at 50
	if pnl != 0 {
		t.Errorf("expected pnl=0 on open, got %f", pnl)
	}
	if p.Size != 100 {
		t.Errorf("expected Size=100, got %f", p.Size)
	}
	if p.Price != 50.0 {
		t.Errorf("expected Price=50, got %f", p.Price)
	}
}

func TestPositionAverageIn(t *testing.T) {
	p := &Position{}
	p.Update(100, 50.0) // first buy at 50
	p.Update(100, 60.0) // second buy at 60 → avg = 55
	if math.Abs(p.Price-55.0) > 1e-9 {
		t.Errorf("expected avg price=55, got %f", p.Price)
	}
	if p.Size != 200 {
		t.Errorf("expected Size=200, got %f", p.Size)
	}
}

func TestPositionCloseLong(t *testing.T) {
	p := &Position{}
	p.Update(100, 50.0)      // buy 100 @ 50
	pnl := p.Update(-100, 60.0) // sell 100 @ 60
	if math.Abs(pnl-1000.0) > 1e-9 {
		t.Errorf("expected pnl=1000, got %f", pnl)
	}
	if p.IsOpen() {
		t.Error("expected position closed")
	}
}

func TestPositionPartialClose(t *testing.T) {
	p := &Position{}
	p.Update(100, 50.0)
	pnl := p.Update(-50, 60.0) // close half
	if math.Abs(pnl-500.0) > 1e-9 {
		t.Errorf("expected pnl=500, got %f", pnl)
	}
	if p.Size != 50 {
		t.Errorf("expected remaining Size=50, got %f", p.Size)
	}
}

func TestPositionPnL(t *testing.T) {
	p := &Position{}
	p.Update(100, 50.0)
	unrealised := p.PnL(55.0)
	if math.Abs(unrealised-500.0) > 1e-9 {
		t.Errorf("expected unrealised PnL=500, got %f", unrealised)
	}
}
