package feeds

import (
	"path/filepath"
	"runtime"
	"testing"
)

func testdataPath(file string) string {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..")
	return filepath.Join(root, "testdata", file)
}

func TestCSVFeedLoad(t *testing.T) {
	cfg := DefaultYahooConfig(testdataPath("sample.csv"))
	feed := NewCSVFeed(cfg)

	if err := feed.Load(); err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if feed.TotalBars() != 50 {
		t.Fatalf("expected 50 bars, got %d", feed.TotalBars())
	}

	// First Next()
	if !feed.Next() {
		t.Fatal("expected Next()=true for first bar")
	}
	bar := feed.Data().Bar()
	if bar.Open != 125.07 {
		t.Errorf("expected Open=125.07, got %f", bar.Open)
	}
	if bar.Close != 130.73 {
		t.Errorf("expected Close=130.73, got %f", bar.Close)
	}

	// Advance through all remaining bars
	count := 1
	for feed.Next() {
		count++
	}
	if count != 50 {
		t.Errorf("expected 50 bars total, advanced %d", count)
	}

	// Additional Next() after exhaustion should return false
	if feed.Next() {
		t.Error("expected Next()=false after all bars consumed")
	}
}
