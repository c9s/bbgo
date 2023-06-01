package indicator

import (
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Cumulative Moving Average, Cumulative Average
// Refer: https://en.wikipedia.org/wiki/Moving_average
//
//go:generate callbackgen -type CA
type CA struct {
	types.SeriesBase
	Interval        types.Interval
	Values          floats.Slice
	length          float64
	updateCallbacks []func(value float64)
}

func (inc *CA) Update(x float64) {
	if len(inc.Values) == 0 {
		inc.SeriesBase.Series = inc
	}

	newVal := (inc.Values.Last(0)*inc.length + x) / (inc.length + 1.)
	inc.length += 1
	inc.Values.Push(newVal)
	if len(inc.Values) > MaxNumOfEWMA {
		inc.Values = inc.Values[MaxNumOfEWMATruncateSize-1:]
		inc.length = float64(len(inc.Values))
	}
}

func (inc *CA) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *CA) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *CA) Length() int {
	return len(inc.Values)
}

var _ types.SeriesExtend = &CA{}

func (inc *CA) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
}

func (inc *CA) CalculateAndUpdate(allKLines []types.KLine) {
	for _, k := range allKLines {
		inc.PushK(k)
		inc.EmitUpdate(inc.Last(0))
	}
}
