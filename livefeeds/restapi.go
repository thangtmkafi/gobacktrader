package livefeeds

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/thangtmkafi/gobacktrader/core"
)

// RESTConfig configures a REST API polling live feed.
type RESTConfig struct {
	// URL is the HTTP endpoint to poll, e.g. "https://api.example.com/bars/AAPL".
	URL string
	// Symbol is used as the DataSeries name.
	Symbol string
	// PollInterval is the delay between polls (e.g. 1*time.Minute for 1m bars).
	PollInterval time.Duration
	// Headers are added to every request (e.g. Authorization).
	Headers map[string]string
	// ParseFunc converts the response body into one or more bars.
	// The function should return only NEW bars (the caller does not de-duplicate).
	ParseFunc func(body []byte) ([]core.Bar, error)
}

// RESTFeed polls a REST API for new bars at a fixed interval.
type RESTFeed struct {
	*liveFeedBase
	cfg    RESTConfig
	client *http.Client
}

// NewRESTFeed creates a REST polling live feed.
func NewRESTFeed(cfg RESTConfig) *RESTFeed {
	name := cfg.Symbol
	if name == "" {
		name = "rest"
	}
	return &RESTFeed{
		liveFeedBase: newLiveFeedBase(name, 256),
		cfg:          cfg,
		client:       &http.Client{Timeout: 30 * time.Second},
	}
}

// Start begins polling in a background goroutine.
func (f *RESTFeed) Start(ctx context.Context) error {
	f.ctx, f.cancel = context.WithCancel(ctx)
	if f.cfg.ParseFunc == nil {
		return fmt.Errorf("restfeed: ParseFunc is required")
	}
	interval := f.cfg.PollInterval
	if interval <= 0 {
		interval = 1 * time.Minute
	}

	go f.pollLoop(interval)
	return nil
}

func (f *RESTFeed) pollLoop(interval time.Duration) {
	defer close(f.barCh)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Do an initial poll immediately
	f.poll()

	for {
		select {
		case <-f.ctx.Done():
			return
		case <-ticker.C:
			f.poll()
		}
	}
}

func (f *RESTFeed) poll() {
	req, err := http.NewRequestWithContext(f.ctx, "GET", f.cfg.URL, nil)
	if err != nil {
		return
	}
	for k, v := range f.cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	bars, err := f.cfg.ParseFunc(body)
	if err != nil {
		return
	}

	for _, bar := range bars {
		select {
		case f.barCh <- bar:
		case <-f.ctx.Done():
			return
		}
	}
}

// Stop cancels the polling goroutine.
func (f *RESTFeed) Stop() error {
	if f.cancel != nil {
		f.cancel()
	}
	return nil
}
