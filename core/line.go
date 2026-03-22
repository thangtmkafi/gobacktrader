// Package core provides the fundamental data structures for gobacktrader.
// It mirrors backtrader's linebuffer.py and lineseries.py concepts,
// but uses idiomatic Go instead of Python metaclasses and operator overloading.
package core

import "math"

// Line is a time-indexed series of float64 values.
// It grows forward as bars are added. Indexing uses "ago" semantics:
//
//	Get(0)  → current bar value
//	Get(-1) → previous bar value
//	Get(-n) → n bars ago
//
// This mirrors backtrader's LineBuffer.__getitem__ negative-offset convention.
type Line struct {
	data   []float64
	cursor int // index of the current bar in data
}

// NewLine creates an empty Line.
func NewLine() *Line {
	return &Line{
		data:   make([]float64, 0, 256),
		cursor: -1,
	}
}

// Forward advances the line by one bar, appending a NaN placeholder.
// The caller should call Set(val) immediately after on the current bar.
func (l *Line) Forward() {
	l.data = append(l.data, math.NaN())
	l.cursor = len(l.data) - 1
}

// Set sets the value of the current bar.
func (l *Line) Set(val float64) {
	if l.cursor < 0 {
		panic("line: Set called before Forward")
	}
	l.data[l.cursor] = val
}

// SetAgo sets the value at 'ago' bars back from the current bar.
// ago must be <= 0 (0 = current, -1 = previous, etc.).
func (l *Line) SetAgo(ago int, val float64) {
	idx := l.cursor + ago
	if idx < 0 || idx >= len(l.data) {
		panic("line: SetAgo index out of range")
	}
	l.data[idx] = val
}

// Get returns the value at 'ago' bars back from the current bar.
// ago must be <= 0. Returns NaN if the index is out of range.
func (l *Line) Get(ago int) float64 {
	idx := l.cursor + ago
	if idx < 0 || idx >= len(l.data) {
		return math.NaN()
	}
	return l.data[idx]
}

// Len returns the total number of bars stored (including current).
func (l *Line) Len() int {
	return len(l.data)
}

// Cursor returns the index of the current bar (0-based from start of data).
func (l *Line) Cursor() int {
	return l.cursor
}

// Array returns a copy of all stored values up to and including the current bar.
func (l *Line) Array() []float64 {
	if l.cursor < 0 {
		return []float64{}
	}
	out := make([]float64, l.cursor+1)
	copy(out, l.data[:l.cursor+1])
	return out
}
