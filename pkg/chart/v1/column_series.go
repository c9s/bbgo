package v1

import (
	"sort"
	"time"

	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
)

var _ IndicatorSeries = &ColumnIndicatorSeries{}

// ColumnSample represents a single column in a histogram chart.
type ColumnSample struct {
	Time  time.Time
	Value float64
	Color drawing.Color
}

// ColumnIndicatorSeries draws histogram columns along the x-axis.
// Useful for momentum indicators like TTM Squeeze momentum.
type ColumnIndicatorSeries struct {
	Name    string
	Options *PanelOptions

	samples []ColumnSample
}

func NewColumnIndicatorSeries(name string, samples []ColumnSample, options *PanelOptions) *ColumnIndicatorSeries {
	return &ColumnIndicatorSeries{
		Name:    name,
		Options: options,
		samples: samples,
	}
}

func (cs *ColumnIndicatorSeries) GetTimeRange() (time.Time, time.Time) {
	if len(cs.samples) == 0 {
		return time.Time{}, time.Time{}
	}
	return cs.samples[0].Time, cs.samples[len(cs.samples)-1].Time
}

func (cs *ColumnIndicatorSeries) GetValueRange() (float64, float64) {
	if len(cs.samples) == 0 {
		return 0., 0.
	}
	minValue, maxValue := cs.samples[0].Value, cs.samples[0].Value
	for _, s := range cs.samples {
		minValue = min(minValue, s.Value)
		maxValue = max(maxValue, s.Value)
	}
	return minValue, maxValue
}

func (cs *ColumnIndicatorSeries) AddSamples(samples ...ColumnSample) {
	cs.samples = append(cs.samples, samples...)
	if len(cs.samples) > len(samples) {
		sort.Slice(cs.samples, func(i, j int) bool {
			return cs.samples[i].Time.Before(cs.samples[j].Time)
		})
	}
}

// Implement chart.Series interface

func (cs *ColumnIndicatorSeries) GetName() string {
	return cs.Name
}

func (cs *ColumnIndicatorSeries) GetStyle() chart.Style {
	return chart.Style{
		StrokeWidth: 1.0,
	}
}

func (cs *ColumnIndicatorSeries) GetYAxis() chart.YAxisType {
	return chart.YAxisPrimary
}

func (cs *ColumnIndicatorSeries) Validate() error {
	return nil
}

func (cs *ColumnIndicatorSeries) Render(r chart.Renderer, b chart.Box, xRange, yRange chart.Range, style chart.Style) {
	if len(cs.samples) == 0 {
		return
	}

	var barWidth float64
	if cs.Options.ColumnWidth <= 0 {
		barWidth = float64(b.Width()) / float64(len(cs.samples))
	} else {
		barWidth = cs.Options.ColumnWidth
	}

	// Apply gap between columns (default 15% gap)
	gapRatio := cs.Options.ColumnGap
	if gapRatio <= 0 {
		gapRatio = 0.15
	}
	drawWidth := barWidth * (1 - gapRatio)

	zeroY := YValueToCanvas(yRange, b, 0)

	lastIdx := len(cs.samples) - 1
	for i, sample := range cs.samples {
		x := chart.TimeToFloat64(sample.Time)
		xp := XValueToCanvas(xRange, b, x)
		yp := YValueToCanvas(yRange, b, sample.Value)

		// First and last columns get half width to avoid extending beyond chart boundaries
		var left, right int
		switch i {
		case 0:
			// First column: only extend to the right
			left = xp
			right = int(float64(xp) + drawWidth/2)
		case lastIdx:
			// Last column: only extend to the left
			left = int(float64(xp) - drawWidth/2)
			right = xp
		default:
			// Middle columns: full width centered on time
			left = int(float64(xp) - drawWidth/2)
			right = int(float64(xp) + drawWidth/2)
		}
		if right == left {
			right = left + 1
		}

		// Draw column from zero line to value
		top := yp
		bottom := zeroY
		if sample.Value < 0 {
			top = zeroY
			bottom = yp
		}

		r.SetFillColor(sample.Color)
		r.MoveTo(left, top)
		r.LineTo(right, top)
		r.LineTo(right, bottom)
		r.LineTo(left, bottom)
		r.LineTo(left, top)
		r.Fill()
	}
}
