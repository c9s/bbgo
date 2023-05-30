package indicator

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type Float64Updater
type Float64Updater struct {
	updateCallbacks []func(v float64)

	slice floats.Slice
}

type Float64Series struct {
	types.SeriesBase
	Float64Updater
}

func (f *Float64Series) Last() float64 {
	return f.slice.Last()
}

func (f *Float64Series) Index(i int) float64 {
	length := len(f.slice)
	if length == 0 || length-i-1 < 0 {
		return 0
	}
	return f.slice[length-i-1]
}

func (f *Float64Series) Length() int {
	return len(f.slice)
}
