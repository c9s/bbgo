package indicator

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type Float64Updater
type Float64Updater struct {
	updateCallbacks []func(v float64)
}

type Float64Series struct {
	types.SeriesBase
	Float64Updater
	slice floats.Slice
}

func NewFloat64Series(v ...float64) Float64Series {
	s := Float64Series{}
	s.slice = v
	s.SeriesBase.Series = s.slice
	return s
}

func (f *Float64Series) Last(i int) float64 {
	return f.slice.Last(i)
}

func (f *Float64Series) Index(i int) float64 {
	return f.slice.Last(i)
}

func (f *Float64Series) Length() int {
	return len(f.slice)
}
