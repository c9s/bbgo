package indicator

import (
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type ATR
type ATR struct {
	types.SeriesBase
	types.IntervalWindow
	PercentageVolatility floats.Slice

	PreviousClose float64
	RMA           *RMA

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

var _ types.SeriesExtend = &ATR{}

func (inc *ATR) Clone() *ATR {
	out := &ATR{
		IntervalWindow:       inc.IntervalWindow,
		PercentageVolatility: inc.PercentageVolatility[:],
		PreviousClose:        inc.PreviousClose,
		RMA:                  inc.RMA.Clone().(*RMA),
		EndTime:              inc.EndTime,
	}
	out.SeriesBase.Series = out
	return out
}

func (inc *ATR) TestUpdate(high, low, cloze float64) *ATR {
	c := inc.Clone()
	c.Update(high, low, cloze)
	return c
}

func (inc *ATR) Update(high, low, cloze float64) {
	if inc.Window <= 0 {
		panic("window must be greater than 0")
	}

	if inc.RMA == nil {
		inc.SeriesBase.Series = inc
		inc.RMA = &RMA{
			IntervalWindow: types.IntervalWindow{Window: inc.Window},
			Adjust:         true,
		}
		inc.PreviousClose = cloze
		return
	}

	// calculate true range
	trueRange := high - low
	hc := math.Abs(high - inc.PreviousClose)
	lc := math.Abs(low - inc.PreviousClose)
	if trueRange < hc {
		trueRange = hc
	}
	if trueRange < lc {
		trueRange = lc
	}

	inc.PreviousClose = cloze

	// apply rolling moving average
	inc.RMA.Update(trueRange)
	atr := inc.RMA.Last()
	inc.PercentageVolatility.Push(atr / cloze)
}

func (inc *ATR) Last() float64 {
	if inc.RMA == nil {
		return 0
	}
	return inc.RMA.Last()
}

func (inc *ATR) Index(i int) float64 {
	if inc.RMA == nil {
		return 0
	}
	return inc.RMA.Index(i)
}

func (inc *ATR) Length() int {
	if inc.RMA == nil {
		return 0
	}

	return inc.RMA.Length()
}

func (inc *ATR) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
		return
	}

	inc.Update(k.High.Float64(), k.Low.Float64(), k.Close.Float64())
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last())
}
