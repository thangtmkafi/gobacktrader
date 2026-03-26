package plotter

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"

	"github.com/thangtmkafi/gobacktrader/core"
	"github.com/thangtmkafi/gobacktrader/engine"
)

type candleData struct {
	Time   int64   `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
}

type lineData struct {
	Time  int64   `json:"time"`
	Value float64 `json:"value"`
}

type markerData struct {
	Time     int64  `json:"time"`
	Position string `json:"position"`
	Color    string `json:"color"`
	Shape    string `json:"shape"`
	Text     string `json:"text"`
}

// chartTemplate holds the HTML structure powered by TradingView Lightweight Charts
const chartTemplate = `
<!DOCTYPE html>
<html>
<head>
    <title>gobacktrader - Strategy Results</title>
    <script src="https://unpkg.com/lightweight-charts/dist/lightweight-charts.standalone.production.js"></script>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0b0e14; color: #fff; margin:0; padding:20px; }
        .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
        .header h1 { font-size: 1.5rem; margin: 0; color: #06b6d4; }
        .stats { display: flex; gap: 20px; font-size: 0.9rem; }
        .stat-box { background: #1f2937; padding: 10px 15px; border-radius: 8px; border: 1px solid #374151; }
        #chart { width: 100%; height: 70vh; }
        #equity { width: 100%; height: 20vh; margin-top: 10px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🚀 gobacktrader</h1>
        <div class="stats">
            <div class="stat-box"><strong>Start Cash:</strong> ${{printf "%.2f" .Result.StartingCash}}</div>
            <div class="stat-box"><strong>Final Value:</strong> ${{printf "%.2f" .Result.FinalValue}}</div>
            <div class="stat-box"><strong>Trades:</strong> {{len .Result.Trades}}</div>
        </div>
    </div>
    
    <div id="chart"></div>
    <div id="equity"></div>

    <script>
        const chartOptions = { 
            layout: { textColor: '#d1d5db', background: { type: 'solid', color: '#111827' } },
            grid: { vertLines: { color: '#1f2937' }, horzLines: { color: '#1f2937' } },
            crosshair: { mode: LightweightCharts.CrosshairMode.Normal },
            timeScale: { timeVisible: true, secondsVisible: false }
        };
        
        const chart = LightweightCharts.createChart(document.getElementById('chart'), chartOptions);
        const mainSeries = chart.addCandlestickSeries({
            upColor: '#10b981', downColor: '#ef4444', borderVisible: false, wickUpColor: '#10b981', wickDownColor: '#ef4444'
        });
        
        mainSeries.setData({{.Candles}});
        mainSeries.setMarkers({{.Markers}});

        // Volume Series attached
        const volumeSeries = chart.addHistogramSeries({
            color: '#374151',
            priceFormat: { type: 'volume' },
            priceScaleId: '', // set as an overlay
        });
        chart.priceScale('').applyOptions({ scaleMargins: { top: 0.8, bottom: 0 } });
        volumeSeries.setData({{.Volumes}});

        // Equity Chart
        const eqChart = LightweightCharts.createChart(document.getElementById('equity'), {
            ...chartOptions,
            timeScale: { visible: false } // sync manually if needed or hide
        });
        const eqSeries = eqChart.addLineSeries({ color: '#8b5cf6', lineWidth: 2 });
        eqSeries.setData({{.Equity}});
        
        // Synchronize charts (basic)
        chart.timeScale().subscribeVisibleLogicalRangeChange(range => {
            eqChart.timeScale().setVisibleLogicalRange(range);
        });
    </script>
</body>
</html>
`

// Plot generates an interactive HTML chart from a single DataSeries and a RunResult.
func Plot(data *core.DataSeries, result *engine.RunResult, outputPath string) error {
	length := data.Len()
	if length == 0 {
		return fmt.Errorf("plotter: data series is empty")
	}

	candles := make([]candleData, 0, length)
	volumes := make([]struct {
		Time  int64   `json:"time"`
		Value float64 `json:"value"`
		Color string  `json:"color"`
	}, 0, length)
	equity := make([]lineData, 0, len(result.EquityCurve))

	// Extract Bar data and sync equity
	for i := length - 1; i >= 0; i-- {
		// core.Line uses 0 for newest, n-1 for oldest. So i=length-1 is oldest.
		b := data.BarAgo(-i)
		unix := b.DateTime.Unix()
		
		candles = append(candles, candleData{
			Time:   unix,
			Open:   b.Open,
			High:   b.High,
			Low:    b.Low,
			Close:  b.Close,
		})

		volColor := "#ef444455"
		if b.Close >= b.Open {
			volColor = "#10b98155"
		}
		volumes = append(volumes, struct {
			Time  int64   `json:"time"`
			Value float64 `json:"value"`
			Color string  `json:"color"`
		}{
			Time:  unix,
			Value: b.Volume,
			Color: volColor,
		})

		// Match equity curve if sizes match
		eqIdx := length - 1 - i
		if eqIdx >= 0 && eqIdx < len(result.EquityCurve) {
			equity = append(equity, lineData{
				Time:  unix,
				Value: result.EquityCurve[eqIdx],
			})
		}
	}

	// Extract trades -> markers
	markers := make([]markerData, 0)
	for _, t := range result.Trades {
		// Entry marker
		pos := "belowBar"
		color := "#10b981"
		shape := "arrowUp"
		if t.Size < 0 {
			pos = "aboveBar"
			color = "#ef4444"
			shape = "arrowDown"
		}
		markers = append(markers, markerData{
			Time:     t.EntryTime.Unix(),
			Position: pos,
			Color:    color,
			Shape:    shape,
			Text:     fmt.Sprintf("Entry @ %.2f", t.EntryPrice),
		})

		// Exit marker (if closed)
		if !t.IsOpen {
			exitPos := "aboveBar"
			exitColor := "#ef4444"
			exitShape := "arrowDown"
			if t.Size < 0 {
				exitPos = "belowBar"
				exitColor = "#10b981"
				exitShape = "arrowUp"
			}
			markers = append(markers, markerData{
				Time:     t.ExitTime.Unix(),
				Position: exitPos,
				Color:    exitColor,
				Shape:    exitShape,
				Text:     fmt.Sprintf("Exit @ %.2f", t.ExitPrice),
			})
		}
	}

	canJSON, _ := json.Marshal(candles)
	volJSON, _ := json.Marshal(volumes)
	eqJSON, _ := json.Marshal(equity)
	markJSON, _ := json.Marshal(markers)

	tmpl, err := template.New("chart").Parse(chartTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	payload := map[string]interface{}{
		"Result":  result,
		"Candles": template.JS(canJSON),
		"Volumes": template.JS(volJSON),
		"Equity":  template.JS(eqJSON),
		"Markers": template.JS(markJSON),
	}

	return tmpl.Execute(f, payload)
}
