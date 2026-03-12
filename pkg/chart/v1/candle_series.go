package v1

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/wcharczuk/go-chart/v2"
)

func init() {
	_ = chart.Series(&CandlestickSeries{})
}

type Candle struct {
	Time   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

type CandlestickSeries struct {
	Candles []Candle
}

func YValueToCanvas(r chart.Range, b chart.Box, v float64) int {
	yRange := r.GetMax() - r.GetMin()
	if yRange == 0 {
		return b.Bottom
	}

	percent := (v - r.GetMin()) / yRange
	return b.Bottom - int(percent*float64(b.Height()))
}

func XValueToCanvas(r chart.Range, b chart.Box, v float64) int {
	xRange := r.GetMax() - r.GetMin()
	if xRange == 0 {
		return b.Left
	}
	percent := (v - r.GetMin()) / xRange
	return b.Left + int(percent*float64(b.Width()))
}

func (cs CandlestickSeries) Len() int {
	return len(cs.Candles)
}

func (ts CandlestickSeries) GetFirstValues() (x, y float64) {
	x = chart.TimeToFloat64(ts.Candles[0].Time)
	y = ts.Candles[0].Close
	return
}

func (ts CandlestickSeries) GetLastValues() (x, y float64) {
	x = chart.TimeToFloat64(ts.Candles[len(ts.Candles)-1].Time)
	y = ts.Candles[len(ts.Candles)-1].Close
	return
}

func (cs CandlestickSeries) GetValue(index int) float64 {
	return cs.Candles[index].Close
}

func (cs CandlestickSeries) GetXAxisValue(index int) float64 {
	return chart.TimeToFloat64(cs.Candles[index].Time)
}

func (cs CandlestickSeries) GetValueFormatters() (x, y chart.ValueFormatter) {
	x = chart.TimeValueFormatter
	y = chart.FloatValueFormatter
	return
}

/*
Implement chart.Series interface for CandlestickSeries.
*/
func (cs CandlestickSeries) GetName() string {
	return "KLine"
}

func (cs CandlestickSeries) GetYAxis() chart.YAxisType {
	return chart.YAxisPrimary
}

func (cs CandlestickSeries) GetStyle() chart.Style {
	return chart.Style{
		StrokeWidth: 1.0,
	}
}

// Validate implements chart.Series interface for CandlestickSeries.
func (cs CandlestickSeries) Validate() error {
	if len(cs.Candles) == 0 {
		return nil
	}
	return nil
}

func (cs CandlestickSeries) Render(
	r chart.Renderer, canvasBox chart.Box, xrange, yrange chart.Range, style chart.Style,
) {
	totalWidth := canvasBox.Width()
	barWidth := totalWidth / len(cs.Candles)

	for _, candle := range cs.Candles {
		x := chart.TimeToFloat64(candle.Time)
		xp := XValueToCanvas(xrange, canvasBox, x)

		yHigh := YValueToCanvas(yrange, canvasBox, candle.High)
		yLow := YValueToCanvas(yrange, canvasBox, candle.Low)
		yOpen := YValueToCanvas(yrange, canvasBox, candle.Open)
		yClose := YValueToCanvas(yrange, canvasBox, candle.Close)

		color := chart.ColorGreen
		if yClose > yOpen {
			color = chart.ColorRed
		}

		// Draw wick (high-low line)
		r.SetStrokeColor(color)
		r.SetStrokeWidth(1.0)
		r.MoveTo(xp, yHigh)
		r.LineTo(xp, yLow)
		r.Stroke()

		// Draw body (open-close rectangle)
		top := yOpen
		bottom := yClose
		if yClose < yOpen {
			top, bottom = yClose, yOpen
		}

		left := xp - barWidth/2
		right := xp + barWidth/2

		r.SetFillColor(color)
		r.MoveTo(left, top)
		r.LineTo(right, top)
		r.LineTo(right, bottom)
		r.LineTo(left, bottom)
		r.LineTo(left, top)
		r.Fill()
	}

	// Add labels for highest and lowest prices
	addLabels(r, canvasBox, xrange, yrange, cs.Candles)
}

// addLabels adds labels for the highest and lowest prices on the chart.
func addLabels(r chart.Renderer, canvasBox chart.Box, xrange, yrange chart.Range, candles []Candle) {
	if len(candles) == 0 {
		return
	}

	highest := candles[0]
	lowest := candles[0]

	for _, c := range candles {
		if c.High > highest.High {
			highest = c
		}
		if c.Low < lowest.Low {
			lowest = c
		}
	}

	highX := XValueToCanvas(xrange, canvasBox, chart.TimeToFloat64(highest.Time))
	highY := YValueToCanvas(yrange, canvasBox, highest.High)
	lowX := XValueToCanvas(xrange, canvasBox, chart.TimeToFloat64(lowest.Time))
	lowY := YValueToCanvas(yrange, canvasBox, lowest.Low)

	r.SetFontColor(chart.ColorBlack)
	r.SetFontSize(10)

	// Draw label for the highest price
	r.Text(fmt.Sprintf("%.2f", highest.High), highX, highY-10)

	// Draw label for the lowest price
	r.Text(fmt.Sprintf("%.2f", lowest.Low), lowX, lowY+10)
}

// XValues returns all X axis values for the series.
func (cs CandlestickSeries) XValues() []float64 {
	values := make([]float64, len(cs.Candles))
	for i, c := range cs.Candles {
		values[i] = chart.TimeToFloat64(c.Time)
	}

	logrus.Infof("XValues: %v", values)
	return values
}
