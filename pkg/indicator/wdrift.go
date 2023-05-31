package indicator

import (
	"math"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: https://tradingview.com/script/aDymGrFx-Drift-Study-Inspired-by-Monte-Carlo-Simulations-with-BM-KL/
// Brownian Motion's drift factor
// could be used in Monte Carlo Simulations
//
//go:generate callbackgen -type WeightedDrift
type WeightedDrift struct {
	types.SeriesBase
	types.IntervalWindow
	chng            *types.Queue
	Values          floats.Slice
	MA              types.UpdatableSeriesExtend
	Weight          *types.Queue
	LastValue       float64
	UpdateCallbacks []func(value float64)
}

func (inc *WeightedDrift) Update(value float64, weight float64) {
	if weight == 0 {
		inc.LastValue = value
		return
	}
	if inc.chng == nil {
		inc.SeriesBase.Series = inc
		if inc.MA == nil {
			inc.MA = &SMA{IntervalWindow: types.IntervalWindow{Interval: inc.Interval, Window: inc.Window}}
		}
		inc.Weight = types.NewQueue(inc.Window)
		inc.chng = types.NewQueue(inc.Window)
		inc.LastValue = value
		inc.Weight.Update(weight)
		return
	}
	inc.Weight.Update(weight)
	base := inc.Weight.Lowest(inc.Window)
	multiplier := int(weight / base)
	var chng float64
	if value == 0 {
		chng = 0
	} else {
		chng = math.Log(value/inc.LastValue) / weight * base
		inc.LastValue = value
	}
	for i := 0; i < multiplier; i++ {
		inc.MA.Update(chng)
		inc.chng.Update(chng)
	}
	if inc.chng.Length() >= inc.Window {
		stdev := types.Stdev(inc.chng, inc.Window)
		drift := inc.MA.Last(0) - stdev*stdev*0.5
		inc.Values.Push(drift)
	}
}

// Assume that MA is SMA
func (inc *WeightedDrift) ZeroPoint() float64 {
	window := float64(inc.Window)
	stdev := types.Stdev(inc.chng, inc.Window)
	chng := inc.chng.Index(inc.Window - 1)
	/*b := -2 * inc.MA.Last() - 2
	c := window * stdev * stdev - chng * chng + 2 * chng * (inc.MA.Last() + 1) - 2 * inc.MA.Last() * window

	root := math.Sqrt(b*b - 4*c)
	K1 := (-b + root)/2
	K2 := (-b - root)/2
	N1 := math.Exp(K1) * inc.LastValue
	N2 := math.Exp(K2) * inc.LastValue
	if math.Abs(inc.LastValue-N1) < math.Abs(inc.LastValue-N2) {
		return N1
	} else {
		return N2
	}*/
	return inc.LastValue * math.Exp(window*(0.5*stdev*stdev)+chng-inc.MA.Last(0)*window)
}

func (inc *WeightedDrift) Clone() (out *WeightedDrift) {
	out = &WeightedDrift{
		IntervalWindow: inc.IntervalWindow,
		chng:           inc.chng.Clone(),
		Values:         inc.Values[:],
		MA:             types.Clone(inc.MA),
		Weight:         inc.Weight.Clone(),
		LastValue:      inc.LastValue,
	}
	out.SeriesBase.Series = out
	return out
}

func (inc *WeightedDrift) TestUpdate(value float64, weight float64) *WeightedDrift {
	out := inc.Clone()
	out.Update(value, weight)
	return out
}

func (inc *WeightedDrift) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *WeightedDrift) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *WeightedDrift) Length() int {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Length()
}

var _ types.SeriesExtend = &Drift{}

func (inc *WeightedDrift) PushK(k types.KLine) {
	inc.Update(k.Close.Float64(), k.Volume.Abs().Float64())
}

func (inc *WeightedDrift) CalculateAndUpdate(allKLines []types.KLine) {
	if inc.chng == nil {
		for _, k := range allKLines {
			inc.PushK(k)
			inc.EmitUpdate(inc.Last(0))
		}
	} else {
		k := allKLines[len(allKLines)-1]
		inc.PushK(k)
		inc.EmitUpdate(inc.Last(0))
	}
}

func (inc *WeightedDrift) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *WeightedDrift) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
