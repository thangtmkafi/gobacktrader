// Package analyzers provides post-run performance analysis for gobacktrader.
//
// Usage — attach analyzers to Cerebro before Run():
//
//	c := engine.NewCerebro()
//	sharpe := &analyzers.SharpeRatio{}
//	dd     := &analyzers.DrawDown{}
//	c.AddAnalyzer(sharpe)
//	c.AddAnalyzer(dd)
//	c.Run()
//	sharpe.Print()
//	dd.Print()
package analyzers

import (
	"github.com/thangtmkafi/gobacktrader/core"
	"github.com/thangtmkafi/gobacktrader/engine"
)

// Ensure package-level imports resolve (used by concrete analyzers).
var _ *core.DataSeries
var _ *engine.RunResult
