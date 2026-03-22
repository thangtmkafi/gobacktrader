// Package core provides the fundamental data structures for gobacktrader.
package core

import "time"

// Bar represents a single OHLCV bar snapshot.
type Bar struct {
	DateTime    time.Time
	Open        float64
	High        float64
	Low         float64
	Close       float64
	Volume      float64
	OpenInterest float64
}

// DataSeries holds the historical OHLCV lines for a single instrument.
// It mirrors backtrader's DataSeries / OHLCDateTime classes.
type DataSeries struct {
	Name string

	DateTime    *Line
	Open        *Line
	High        *Line
	Low         *Line
	Close       *Line
	Volume      *Line
	OpenInterest *Line
}

// NewDataSeries creates a DataSeries with all lines initialised.
func NewDataSeries(name string) *DataSeries {
	return &DataSeries{
		Name:         name,
		DateTime:     NewLine(),
		Open:         NewLine(),
		High:         NewLine(),
		Low:          NewLine(),
		Close:        NewLine(),
		Volume:       NewLine(),
		OpenInterest: NewLine(),
	}
}

// Forward advances all lines by one bar (must be followed by AppendBar or
// individual Set calls on each line).
func (ds *DataSeries) Forward() {
	ds.DateTime.Forward()
	ds.Open.Forward()
	ds.High.Forward()
	ds.Low.Forward()
	ds.Close.Forward()
	ds.Volume.Forward()
	ds.OpenInterest.Forward()
}

// AppendBar writes a single bar into the current (already-forwarded) slot.
func (ds *DataSeries) AppendBar(b Bar) {
	// store DateTime as unix seconds float so Line only holds float64
	ds.DateTime.Set(float64(b.DateTime.Unix()))
	ds.Open.Set(b.Open)
	ds.High.Set(b.High)
	ds.Low.Set(b.Low)
	ds.Close.Set(b.Close)
	ds.Volume.Set(b.Volume)
	ds.OpenInterest.Set(b.OpenInterest)
}

// Bar returns a snapshot of the current bar values.
func (ds *DataSeries) Bar() Bar {
	return Bar{
		DateTime:    time.Unix(int64(ds.DateTime.Get(0)), 0).UTC(),
		Open:        ds.Open.Get(0),
		High:        ds.High.Get(0),
		Low:         ds.Low.Get(0),
		Close:       ds.Close.Get(0),
		Volume:      ds.Volume.Get(0),
		OpenInterest: ds.OpenInterest.Get(0),
	}
}

// BarAgo returns a snapshot of the bar 'ago' bars back (ago <= 0).
func (ds *DataSeries) BarAgo(ago int) Bar {
	return Bar{
		DateTime:    time.Unix(int64(ds.DateTime.Get(ago)), 0).UTC(),
		Open:        ds.Open.Get(ago),
		High:        ds.High.Get(ago),
		Low:         ds.Low.Get(ago),
		Close:       ds.Close.Get(ago),
		Volume:      ds.Volume.Get(ago),
		OpenInterest: ds.OpenInterest.Get(ago),
	}
}

// Len returns the number of bars loaded so far (same for all lines).
func (ds *DataSeries) Len() int {
	return ds.Close.Len()
}
