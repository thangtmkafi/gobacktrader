package livebrokers_test

import (
	"bytes"
	"testing"

	"github.com/thangtmkafi/gobacktrader/core"
	"github.com/thangtmkafi/gobacktrader/engine"
	"github.com/thangtmkafi/gobacktrader/livebrokers"
)

// TestLiveBrokerInterface ensures that LiveBroker fully satisfies BrokerBase
func TestLiveBrokerInterface(t *testing.T) {
	var _ engine.BrokerBase = (*livebrokers.LiveBroker)(nil)
}

func TestLogRouter(t *testing.T) {
	var buf bytes.Buffer
	router := livebrokers.NewLogRouterTo(&buf)

	lb := livebrokers.NewLiveBroker(router, 100000)

	data := core.NewDataSeries("TEST")

	order := engine.NewOrder(data, engine.OrderSideBuy, engine.OrderTypeMarket, 10, 0, 0)

	lb.Submit(order)

	output := buf.String()
	if output == "" {
		t.Fatal("Expected LogRouter to produce output, got empty string")
	}

	if !bytes.Contains([]byte(output), []byte("ORDER PLACED: TEST")) {
		t.Errorf("Expected LogRouter output to contain 'ORDER PLACED: TEST', got: %s", output)
	}
}
