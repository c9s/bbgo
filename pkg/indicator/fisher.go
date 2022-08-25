package indicator

import (
	"math"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type FisherTransform
type FisherTransform struct {
	types.SeriesBase
	types.IntervalWindow
	prices *types.Queue
	Values floats.Slice

	UpdateCallbacks []func(value float64)
}

func (inc *FisherTransform) Clone() types.UpdatableSeriesExtend {
	out := FisherTransform{
		IntervalWindow: inc.IntervalWindow,
		prices:         inc.prices.Clone(),
		Values:         inc.Values[:],
	}
	out.SeriesBase.Series = &out
	return &out
}

func (inc *FisherTransform) Update(value float64) {
	if inc.prices == nil {
		inc.prices = types.NewQueue(inc.Window)
		inc.SeriesBase.Series = inc
	}
	inc.prices.Update(value)
	highest := inc.prices.Highest(inc.Window)
	lowest := inc.prices.Lowest(inc.Window)
	if highest == lowest {
		inc.Values.Update(0)
		return
	}
	x := 2*((value-lowest)/(highest-lowest)) - 1
	if x == 1 {
		x = 0.9999
	} else if x == -1 {
		x = -0.9999
	}
	inc.Values.Update(0.5 * math.Log((1+x)/(1-x)))
	if len(inc.Values) > MaxNumOfEWMA {
		inc.Values = inc.Values[MaxNumOfEWMATruncateSize-1:]
	}
}

func (inc *FisherTransform) Last() float64 {
	if inc.Values == nil {
		return 0.0
	}
	return inc.Values.Last()
}

func (inc *FisherTransform) Index(i int) float64 {
	if inc.Values == nil {
		return 0.0
	}
	return inc.Values.Index(i)
}

func (inc *FisherTransform) Length() int {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Length()
}
