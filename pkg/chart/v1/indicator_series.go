package v1

import (
	"sort"
	"time"

	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
)

var (
	_ IndicatorSeries = &BandIndicatorSeries{}
	_ IndicatorSeries = &LineIndicatorSeries{}
)

type LegendKind string

var (
	LegendTop  = LegendKind("top")
	LegendThin = LegendKind("thin")
	LegendLeft = LegendKind("left")
)

type IndicatorSeries interface {
	chart.Series
	GetTimeRange() (time.Time, time.Time)
	GetValueRange() (float64, float64)
}

type BandSample struct {
	Time                          time.Time
	UpperBound, LowerBound, Value *float64
}
type BandIndicatorSeries struct {
	Name    string
	Options *PanelOptions

	samples []BandSample
}

func (bs *BandIndicatorSeries) GetTimeRange() (time.Time, time.Time) {
	if len(bs.samples) == 0 {
		return time.Time{}, time.Time{}
	}
	return bs.samples[0].Time, bs.samples[len(bs.samples)-1].Time
}

func (bs *BandIndicatorSeries) GetValueRange() (float64, float64) {
	if len(bs.samples) == 0 {
		return 0., 0.
	}
	minValue, maxValue := 0.0, 0.0
	for _, s := range bs.samples {
		if s.Value == nil {
			continue
		}
		if minValue == 0.0 && maxValue == 0.0 {
			minValue = *s.Value
			maxValue = *s.Value
		} else if s.Value != nil {
			minValue = min(minValue, *s.Value)
			maxValue = max(maxValue, *s.Value)
		}
	}
	return minValue, maxValue
}

func NewBandIndicatorSeries(name string, samples []BandSample, options *PanelOptions) *BandIndicatorSeries {
	if options == nil {
		options = &PanelOptions{
			UpperBoundColor: chart.ColorGreen.String(),
			LowerBoundColor: chart.ColorRed.String(),
			ValueColor:      chart.ColorAlternateGray.String(),
		}
	}
	return &BandIndicatorSeries{
		Name:    name,
		Options: options,
		samples: samples,
	}
}

func (s *BandIndicatorSeries) AddSamples(samples ...BandSample) {
	s.samples = append(s.samples, samples...)
	if len(s.samples) > len(samples) {
		// sort samples by time
		sort.Slice(s.samples, func(i, j int) bool {
			return s.samples[i].Time.Before(s.samples[j].Time)
		})
	}
}

// Implement chart.Series interface for BandIndicatorSeries.
func (bs *BandIndicatorSeries) GetName() string {
	return bs.Name
}

func (bs *BandIndicatorSeries) GetStyle() chart.Style {
	return chart.Style{
		StrokeWidth: 1.0,
	}
}

func (bs *BandIndicatorSeries) GetYAxis() chart.YAxisType {
	return chart.YAxisPrimary
}

func (bs *BandIndicatorSeries) Validate() error {
	return nil
}

func (bs *BandIndicatorSeries) Render(r chart.Renderer, b chart.Box, xRange, yRange chart.Range, style chart.Style) {
	drawLine := func(color drawing.Color, getValue func(BandSample) *float64) {
		started := false
		for _, s := range bs.samples {
			v := getValue(s)
			if v == nil {
				// If the value is nil, it means the line is broken at this point.
				// We should stroke the current path if it's started, and then move to the next point without drawing a line.
				if started {
					r.Stroke()
					started = false
				}
				continue
			}
			x := XValueToCanvas(xRange, b, chart.TimeToFloat64(s.Time))
			y := YValueToCanvas(yRange, b, *v)
			if !started {
				r.SetStrokeColor(color)
				r.SetStrokeWidth(style.StrokeWidth)
				r.MoveTo(x, y)
				started = true
			} else {
				r.LineTo(x, y)
			}
		}
		if started {
			r.Stroke()
		}
	}
	upperColor := drawing.ParseColor(bs.Options.UpperBoundColor)
	lowerColor := drawing.ParseColor(bs.Options.LowerBoundColor)
	valueColor := drawing.ParseColor(bs.Options.ValueColor)
	drawLine(upperColor, func(s BandSample) *float64 { return s.UpperBound })
	drawLine(lowerColor, func(s BandSample) *float64 { return s.LowerBound })
	drawLine(valueColor, func(s BandSample) *float64 { return s.Value })
}

type PointSample struct {
	Time  time.Time
	Value *float64
}
type LineIndicatorSeries struct {
	Name string

	points  []PointSample
	options *PanelOptions
}

func NewLineIndicatorSeries(name string, points []PointSample, options *PanelOptions) *LineIndicatorSeries {
	if options == nil {
		options = &PanelOptions{
			ValueColor: chart.ColorAlternateGray.String(),
		}
	}
	return &LineIndicatorSeries{
		Name:    name,
		points:  points,
		options: options,
	}
}

func (ls *LineIndicatorSeries) GetTimeRange() (time.Time, time.Time) {
	if len(ls.points) == 0 {
		return time.Time{}, time.Time{}
	}
	return ls.points[0].Time, ls.points[len(ls.points)-1].Time
}

func (ls *LineIndicatorSeries) GetValueRange() (float64, float64) {
	if len(ls.points) == 0 {
		return 0., 0.
	}
	minValue, maxValue := 0.0, 0.0
	for _, p := range ls.points {
		if p.Value == nil {
			continue
		}
		if minValue == 0.0 && maxValue == 0.0 {
			minValue = *p.Value
			maxValue = *p.Value
		} else if p.Value != nil {
			minValue = min(minValue, *p.Value)
			maxValue = max(maxValue, *p.Value)
		}
	}
	return minValue, maxValue
}

func (ls *LineIndicatorSeries) AddPoints(points ...PointSample) {
	ls.points = append(ls.points, points...)
	if len(ls.points) > len(points) {
		sort.Slice(ls.points, func(i, j int) bool {
			return ls.points[i].Time.Before(ls.points[j].Time)
		})
	}
}

// Implement chart.Series interface for LineIndicatorSeries.
func (ls *LineIndicatorSeries) GetName() string {
	return ls.Name
}

func (ls *LineIndicatorSeries) GetStyle() chart.Style {
	return chart.Style{
		StrokeWidth: 1.0,
	}
}

func (ls *LineIndicatorSeries) GetYAxis() chart.YAxisType {
	return chart.YAxisPrimary
}

func (ls *LineIndicatorSeries) Validate() error {
	return nil
}

func (ls *LineIndicatorSeries) Render(r chart.Renderer, b chart.Box, xRange, yRange chart.Range, style chart.Style) {
	color := drawing.ParseColor(ls.options.ValueColor)
	started := false
	var xs, ys []int
	for _, p := range ls.points {
		if p.Value == nil {
			if started {
				r.Stroke()
				started = false
			}
			continue
		}
		x := XValueToCanvas(xRange, b, chart.TimeToFloat64(p.Time))
		y := YValueToCanvas(yRange, b, *p.Value)
		if !started {
			r.SetStrokeColor(color)
			r.SetStrokeWidth(style.StrokeWidth)
			r.MoveTo(x, y)
			started = true
		} else {
			r.LineTo(x, y)
		}
		xs = append(xs, x)
		ys = append(ys, y)
	}
	if started {
		r.Stroke()
	}
}
