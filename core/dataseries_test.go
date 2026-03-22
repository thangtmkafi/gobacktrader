package core

import (
	"testing"
	"time"
)

func TestDataSeriesBarAppend(t *testing.T) {
	ds := NewDataSeries("TEST")
	now := time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)

	ds.Forward()
	ds.AppendBar(Bar{
		DateTime: now,
		Open:     100, High: 105, Low: 99, Close: 103,
		Volume: 1000, OpenInterest: 0,
	})

	if ds.Len() != 1 {
		t.Fatalf("expected Len=1, got %d", ds.Len())
	}
	bar := ds.Bar()
	if bar.Close != 103 {
		t.Errorf("expected Close=103, got %f", bar.Close)
	}
	if !bar.DateTime.Equal(now) {
		t.Errorf("expected DateTime=%v, got %v", now, bar.DateTime)
	}

	// Second bar
	ds.Forward()
	ds.AppendBar(Bar{
		DateTime: now.Add(24 * time.Hour),
		Open:     103, High: 110, Low: 102, Close: 108,
		Volume: 2000,
	})

	if ds.Len() != 2 {
		t.Fatalf("expected Len=2, got %d", ds.Len())
	}
	// BarAgo(-1) should be the first bar
	prev := ds.BarAgo(-1)
	if prev.Close != 103 {
		t.Errorf("expected BarAgo(-1).Close=103, got %f", prev.Close)
	}
}
