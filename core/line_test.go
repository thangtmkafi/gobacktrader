package core

import (
	"math"
	"testing"
)

func TestLineForwardAndGet(t *testing.T) {
	l := NewLine()

	// Before any Forward, Len should be 0
	if l.Len() != 0 {
		t.Fatalf("expected Len=0, got %d", l.Len())
	}

	// First bar
	l.Forward()
	l.Set(10.0)
	if l.Len() != 1 {
		t.Fatalf("expected Len=1, got %d", l.Len())
	}
	if l.Get(0) != 10.0 {
		t.Errorf("expected Get(0)=10, got %f", l.Get(0))
	}

	// Second bar
	l.Forward()
	l.Set(20.0)
	if l.Get(0) != 20.0 {
		t.Errorf("expected Get(0)=20, got %f", l.Get(0))
	}
	if l.Get(-1) != 10.0 {
		t.Errorf("expected Get(-1)=10, got %f", l.Get(-1))
	}

	// Third bar
	l.Forward()
	l.Set(30.0)
	if l.Get(0) != 30.0 {
		t.Errorf("expected Get(0)=30, got %f", l.Get(0))
	}
	if l.Get(-1) != 20.0 {
		t.Errorf("expected Get(-1)=20, got %f", l.Get(-1))
	}
	if l.Get(-2) != 10.0 {
		t.Errorf("expected Get(-2)=10, got %f", l.Get(-2))
	}

	// Out-of-range Get returns NaN
	if !math.IsNaN(l.Get(-10)) {
		t.Errorf("expected NaN for out-of-range ago, got %f", l.Get(-10))
	}
}

func TestLineArray(t *testing.T) {
	l := NewLine()
	for _, v := range []float64{1, 2, 3, 4, 5} {
		l.Forward()
		l.Set(v)
	}
	arr := l.Array()
	if len(arr) != 5 {
		t.Fatalf("expected array len 5, got %d", len(arr))
	}
	for i, want := range []float64{1, 2, 3, 4, 5} {
		if arr[i] != want {
			t.Errorf("arr[%d]: expected %f, got %f", i, want, arr[i])
		}
	}
}

func TestLineSetAgo(t *testing.T) {
	l := NewLine()
	l.Forward()
	l.Set(1.0)
	l.Forward()
	l.Set(2.0)
	l.SetAgo(-1, 99.0)
	if l.Get(-1) != 99.0 {
		t.Errorf("expected SetAgo(-1) to write 99, got %f", l.Get(-1))
	}
}
