package v1

import (
	"sort"
	"time"

	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
)

var _ IndicatorSeries = &DotIndicatorSeries{}

// DotSample represents a single dot on the chart.
type DotSample struct {
	Time  time.Time
	Y     float64
	Color drawing.Color
}

// DotIndicatorSeries draws dots at given x-y coordinates.
// Useful for visualizing squeeze states in TTM Squeeze indicator.
type DotIndicatorSeries struct {
	Name    string
	Options *PanelOptions

	samples []DotSample
}

func NewDotIndicatorSeries(name string, samples []DotSample, options *PanelOptions) *DotIndicatorSeries {
	if options == nil {
		options = &PanelOptions{
			DotRadius: 3.0,
		}
	}
	return &DotIndicatorSeries{
		Name:    name,
		Options: options,
		samples: samples,
	}
}

func (ds *DotIndicatorSeries) GetTimeRange() (time.Time, time.Time) {
	if len(ds.samples) == 0 {
		return time.Time{}, time.Time{}
	}
	return ds.samples[0].Time, ds.samples[len(ds.samples)-1].Time
}

func (ds *DotIndicatorSeries) GetValueRange() (float64, float64) {
	if len(ds.samples) == 0 {
		return 0., 0.
	}
	minValue, maxValue := ds.samples[0].Y, ds.samples[0].Y
	for _, s := range ds.samples {
		minValue = min(minValue, s.Y)
		maxValue = max(maxValue, s.Y)
	}
	return minValue, maxValue
}

func (ds *DotIndicatorSeries) AddSamples(samples ...DotSample) {
	ds.samples = append(ds.samples, samples...)
	if len(ds.samples) > len(samples) {
		sort.Slice(ds.samples, func(i, j int) bool {
			return ds.samples[i].Time.Before(ds.samples[j].Time)
		})
	}
}

// Implement chart.Series interface

func (ds *DotIndicatorSeries) GetName() string {
	return ds.Name
}

func (ds *DotIndicatorSeries) GetStyle() chart.Style {
	return chart.Style{
		StrokeWidth: 1.0,
	}
}

func (ds *DotIndicatorSeries) GetYAxis() chart.YAxisType {
	return chart.YAxisPrimary
}

func (ds *DotIndicatorSeries) Validate() error {
	return nil
}

func (ds *DotIndicatorSeries) Render(r chart.Renderer, b chart.Box, xRange, yRange chart.Range, style chart.Style) {
	if len(ds.samples) == 0 {
		return
	}

	radius := 3.0
	if ds.Options != nil && ds.Options.DotRadius > 0 {
		radius = ds.Options.DotRadius
	}

	for _, sample := range ds.samples {
		x := chart.TimeToFloat64(sample.Time)
		xp := XValueToCanvas(xRange, b, x)
		yp := YValueToCanvas(yRange, b, sample.Y)

		// Draw a filled circle (dot)
		r.SetFillColor(sample.Color)
		r.Circle(radius, xp, yp)
		r.Fill()
	}
}
