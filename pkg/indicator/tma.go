package indicator

import (
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Triangular Moving Average
// Refer URL: https://ja.wikipedia.org/wiki/移動平均
//
//go:generate callbackgen -type TMA
type TMA struct {
	types.SeriesBase
	types.IntervalWindow
	s1              *SMA
	s2              *SMA
	UpdateCallbacks []func(value float64)
}

func (inc *TMA) Update(value float64) {
	if inc.s1 == nil {
		inc.SeriesBase.Series = inc
		w := (inc.Window + 1) / 2
		inc.s1 = &SMA{IntervalWindow: types.IntervalWindow{Interval: inc.Interval, Window: w}}
		inc.s2 = &SMA{IntervalWindow: types.IntervalWindow{Interval: inc.Interval, Window: w}}
	}

	inc.s1.Update(value)
	inc.s2.Update(inc.s1.Last(0))
}

func (inc *TMA) Last(i int) float64 {
	return inc.s2.Last(i)
}

func (inc *TMA) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *TMA) Length() int {
	if inc.s2 == nil {
		return 0
	}
	return inc.s2.Length()
}

var _ types.SeriesExtend = &TMA{}

func (inc *TMA) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
}
