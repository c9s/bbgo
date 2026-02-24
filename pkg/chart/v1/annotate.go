package v1

import (
	"github.com/c9s/bbgo/pkg/types"
	"github.com/wcharczuk/go-chart/v2"
)

type AnnotateFunc func(chart.Renderer, chart.Box, chart.Range, chart.Range, chart.Style)

// options for future-proofing
type AnnotationOptions struct{}

type AnnotationProvider interface {
	GetAnnotation([]types.KLine, *AnnotationOptions) AnnotateFunc
}
