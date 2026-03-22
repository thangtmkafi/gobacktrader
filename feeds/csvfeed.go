// Package feeds provides data feed implementations for gobacktrader.
// It mirrors backtrader's feed.py and feeds/csvgeneric.py.
package feeds

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/thangtmkafi/gobacktrader/core"
)

// CSVFeedConfig controls how the CSV is parsed.
type CSVFeedConfig struct {
	// File path to the CSV.
	FilePath string

	// DateTimeFormat is the Go time layout string for the DateTime column.
	// Defaults to "2006-01-02" (Yahoo Finance date format).
	DateTimeFormat string

	// Separator is the field delimiter. Defaults to ','.
	Separator rune

	// Column indices (0-based). Defaults match Yahoo Finance format:
	//   Date,Open,High,Low,Close,Adj Close,Volume
	DateTimeCol  int
	OpenCol      int
	HighCol      int
	LowCol       int
	CloseCol     int
	VolumeCol    int
	// OpenInterestCol is optional; -1 means not present.
	OpenInterestCol int

	// HasHeader indicates whether the first row is a header row.
	HasHeader bool

	// ReverseOrder reverses the row order after loading.
	// Yahoo Finance exports newest-first; set true to get chronological order.
	ReverseOrder bool
}

// DefaultYahooConfig returns a CSVFeedConfig pre-configured for Yahoo Finance CSVs.
//
// Yahoo Finance CSV columns: Date,Open,High,Low,Close,Adj Close,Volume
// (We use Close, not Adj Close, as the close price.)
func DefaultYahooConfig(filePath string) CSVFeedConfig {
	return CSVFeedConfig{
		FilePath:        filePath,
		DateTimeFormat:  "2006-01-02",
		Separator:       ',',
		DateTimeCol:     0,
		OpenCol:         1,
		HighCol:         2,
		LowCol:          3,
		CloseCol:        4, // "Close" not "Adj Close" (col 5)
		VolumeCol:       6,
		OpenInterestCol: -1,
		HasHeader:       true,
		ReverseOrder:    false,
	}
}

// CSVFeed loads OHLCV data from a CSV file into a DataSeries.
type CSVFeed struct {
	cfg        CSVFeedConfig
	data       *core.DataSeries // live series (advances bar-by-bar)
	preloaded  *core.DataSeries // fully pre-loaded series for indicator init
	bars       []core.Bar
	cursor     int
}

// NewCSVFeed creates a CSVFeed with the provided configuration.
// Call Load() before using the feed.
func NewCSVFeed(cfg CSVFeedConfig) *CSVFeed {
	name := cfg.FilePath
	// use just the base filename as the series name
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	return &CSVFeed{
		cfg:  cfg,
		data: core.NewDataSeries(name),
	}
}

// Load reads the CSV file and pre-loads all bars into memory.
// Must be called before Next().
func (f *CSVFeed) Load() error {
	file, err := os.Open(f.cfg.FilePath)
	if err != nil {
		return fmt.Errorf("csvfeed: open %q: %w", f.cfg.FilePath, err)
	}
	defer file.Close()

	sep := f.cfg.Separator
	if sep == 0 {
		sep = ','
	}
	layout := f.cfg.DateTimeFormat
	if layout == "" {
		layout = "2006-01-02"
	}

	r := csv.NewReader(bufio.NewReader(file))
	r.Comma = sep

	var bars []core.Bar
	rowIdx := 0
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("csvfeed: read row %d: %w", rowIdx, err)
		}
		if rowIdx == 0 && f.cfg.HasHeader {
			rowIdx++
			continue
		}
		rowIdx++

		bar, err := f.parseRecord(record, layout)
		if err != nil {
			return fmt.Errorf("csvfeed: parse row %d: %w", rowIdx, err)
		}
		bars = append(bars, bar)
	}

	if f.cfg.ReverseOrder {
		for i, j := 0, len(bars)-1; i < j; i, j = i+1, j-1 {
			bars[i], bars[j] = bars[j], bars[i]
		}
	}

	f.bars = bars
	f.cursor = 0

	// Build the fully-preloaded series for indicator construction.
	f.preloaded = core.NewDataSeries(f.data.Name)
	for _, b := range f.bars {
		f.preloaded.Forward()
		f.preloaded.AppendBar(b)
	}
	return nil
}

func (f *CSVFeed) parseRecord(record []string, layout string) (core.Bar, error) {
	parseCol := func(col int) (float64, error) {
		if col < 0 || col >= len(record) {
			return 0, nil
		}
		s := strings.TrimSpace(record[col])
		if s == "" || s == "null" || s == "N/A" {
			return 0, nil
		}
		return strconv.ParseFloat(s, 64)
	}

	dtStr := strings.TrimSpace(record[f.cfg.DateTimeCol])
	dt, err := time.Parse(layout, dtStr)
	if err != nil {
		return core.Bar{}, fmt.Errorf("parse datetime %q with layout %q: %w", dtStr, layout, err)
	}

	open, err := parseCol(f.cfg.OpenCol)
	if err != nil {
		return core.Bar{}, fmt.Errorf("parse Open: %w", err)
	}
	high, err := parseCol(f.cfg.HighCol)
	if err != nil {
		return core.Bar{}, fmt.Errorf("parse High: %w", err)
	}
	low, err := parseCol(f.cfg.LowCol)
	if err != nil {
		return core.Bar{}, fmt.Errorf("parse Low: %w", err)
	}
	close, err := parseCol(f.cfg.CloseCol)
	if err != nil {
		return core.Bar{}, fmt.Errorf("parse Close: %w", err)
	}
	vol, err := parseCol(f.cfg.VolumeCol)
	if err != nil {
		return core.Bar{}, fmt.Errorf("parse Volume: %w", err)
	}
	oi, err := parseCol(f.cfg.OpenInterestCol)
	if err != nil {
		return core.Bar{}, fmt.Errorf("parse OpenInterest: %w", err)
	}

	return core.Bar{
		DateTime:     dt,
		Open:         open,
		High:         high,
		Low:          low,
		Close:        close,
		Volume:       vol,
		OpenInterest: oi,
	}, nil
}

// Next advances the data series by one bar.
// Returns true if a new bar was loaded, false if all bars are exhausted.
func (f *CSVFeed) Next() bool {
	if f.cursor >= len(f.bars) {
		return false
	}
	bar := f.bars[f.cursor]
	f.cursor++
	f.data.Forward()
	f.data.AppendBar(bar)
	return true
}

// Data returns the underlying DataSeries (populated progressively by Next()).
func (f *CSVFeed) Data() *core.DataSeries {
	return f.data
}

// PreloadedData returns the fully-populated DataSeries (all bars loaded).
// Use this for constructing indicators in Init().
func (f *CSVFeed) PreloadedData() *core.DataSeries {
	return f.preloaded
}

// TotalBars returns the total number of pre-loaded bars.
func (f *CSVFeed) TotalBars() int {
	return len(f.bars)
}
