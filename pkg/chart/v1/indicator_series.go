package v1

import (
	"github.com/c9s/bbgo/pkg/types"
	"github.com/wcharczuk/go-chart/v2"
)

var (
	_ chart.Series = &IndicatorSeries{}
)

type Indicator struct {
	Name               string
	Options            AnnotationOptions
	AnnotationProvider AnnotationProvider
}

type IndicatorSeries struct {
	Name    string
	Options AnnotationOptions

	klines             []types.KLine
	annotationProvider AnnotationProvider
}

func NewIndicatorSeries(klines []types.KLine, name string, annotationProvider AnnotationProvider, options AnnotationOptions) *IndicatorSeries {
	return &IndicatorSeries{
		Name:    name,
		Options: options,

		klines:             klines,
		annotationProvider: annotationProvider,
	}
}

// Implement chart.Series interface for IndicatorSeries.
func (is *IndicatorSeries) GetName() string {
	return is.Name
}

func (is *IndicatorSeries) GetStyle() chart.Style {
	return chart.Style{
		StrokeWidth: 1.0,
	}
}

func (is *IndicatorSeries) GetYAxis() chart.YAxisType {
	return chart.YAxisPrimary
}

func (is *IndicatorSeries) Validate() error {
	return nil
}

func (is *IndicatorSeries) Render(r chart.Renderer, b chart.Box, xRange, yRange chart.Range, style chart.Style) {
	annotateFunc := is.annotationProvider.GetAnnotation(is.klines, &is.Options)
	annotateFunc(r, b, xRange, yRange, style)
}
