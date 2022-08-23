package indicator

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// Geometric Moving Average
//go:generate callbackgen -type GMA
type GMA struct {
	types.SeriesBase
	types.IntervalWindow
	SMA             *SMA
	UpdateCallbacks []func(value float64)
}

func (inc *GMA) Last() float64 {
	if inc.SMA == nil {
		return 0.0
	}
	return math.Exp(inc.SMA.Last())
}

func (inc *GMA) Index(i int) float64 {
	if inc.SMA == nil {
		return 0.0
	}
	return math.Exp(inc.SMA.Index(i))
}

func (inc *GMA) Length() int {
	return inc.SMA.Length()
}

func (inc *GMA) Update(value float64) {
	if inc.SMA == nil {
		inc.SMA = &SMA{IntervalWindow: inc.IntervalWindow}
	}
	inc.SMA.Update(math.Log(value))
}

func (inc *GMA) Clone() (out *GMA) {
	out = &GMA{
		IntervalWindow: inc.IntervalWindow,
		SMA:            inc.SMA.Clone().(*SMA),
	}
	out.SeriesBase.Series = out
	return out
}

func (inc *GMA) TestUpdate(value float64) *GMA {
	out := inc.Clone()
	out.Update(value)
	return out
}

var _ types.SeriesExtend = &GMA{}

func (inc *GMA) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
}

func (inc *GMA) LoadK(allKLines []types.KLine) {
	for _, k := range allKLines {
		inc.PushK(k)
	}
}

func (inc *GMA) BindK(target KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}
