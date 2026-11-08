package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Aroon Indicator
// - https://www.investopedia.com/terms/a/aroon.asp
//
// The Aroon indicator was developed by Tushar Chande in 1995 to identify trend changes
// and determine trend strength over a period n (standard default 14 or 25).
//
// Formulas:
// Aroon Up = ((period - periods_since_highest_high) / period) * 100
// Aroon Down = ((period - periods_since_lowest_low) / period) * 100
// Aroon Oscillator = Aroon Up - Aroon Down
//
// Values for Aroon Up and Aroon Down range from 0 to 100.
// Values for Aroon Oscillator range from -100 to +100.

//go:generate callbackgen -type Aroon
type Aroon struct {
	types.SeriesBase
	types.IntervalWindow

	Up         floats.Slice
	Down       floats.Slice
	Oscillator floats.Slice

	HighValues floats.Slice
	LowValues  floats.Slice

	EndTime         time.Time
	UpdateCallbacks []func(up float64, down float64)
}

func (inc *Aroon) Update(high, low float64) {
	if len(inc.Up) == 0 {
		inc.SeriesBase.Series = inc
		if inc.Window <= 0 {
			inc.Window = 14
		}
	}

	inc.HighValues.Push(high)
	inc.LowValues.Push(low)

	up, down := inc.Calculate()
	inc.Up.Push(up)
	inc.Down.Push(down)
	inc.Oscillator.Push(up - down)
}

func (inc *Aroon) Calculate() (up, down float64) {
	if inc.Window <= 0 || len(inc.HighValues) == 0 {
		return 0.0, 0.0
	}

	tailHigh := inc.HighValues.Tail(inc.Window + 1)
	tailLow := inc.LowValues.Tail(inc.Window + 1)
	length := len(tailHigh)
	if length == 0 {
		return 0.0, 0.0
	}

	// Current bar is at index length - 1 (offset 0 from current bar).
	// When multiple bars have the same extreme value, the most recent bar is selected.
	highest := tailHigh[length-1]
	highestIdx := length - 1
	for i := length - 2; i >= 0; i-- {
		if tailHigh[i] > highest {
			highest = tailHigh[i]
			highestIdx = i
		}
	}

	lowest := tailLow[length-1]
	lowestIdx := length - 1
	for i := length - 2; i >= 0; i-- {
		if tailLow[i] < lowest {
			lowest = tailLow[i]
			lowestIdx = i
		}
	}

	periodsSinceHigh := float64(length - 1 - highestIdx)
	periodsSinceLow := float64(length - 1 - lowestIdx)

	period := float64(inc.Window)
	up = ((period - periodsSinceHigh) / period) * 100.0
	down = ((period - periodsSinceLow) / period) * 100.0
	return up, down
}

func (inc *Aroon) Last(i int) float64 {
	return inc.Oscillator.Last(i)
}

func (inc *Aroon) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *Aroon) Length() int {
	return len(inc.Oscillator)
}

var _ types.SeriesExtend = &Aroon{}

func (inc *Aroon) LastUp() float64 {
	if len(inc.Up) == 0 {
		return 0.0
	}
	return inc.Up[len(inc.Up)-1]
}

func (inc *Aroon) LastDown() float64 {
	if len(inc.Down) == 0 {
		return 0.0
	}
	return inc.Down[len(inc.Down)-1]
}

func (inc *Aroon) LastOscillator() float64 {
	if len(inc.Oscillator) == 0 {
		return 0.0
	}
	return inc.Oscillator[len(inc.Oscillator)-1]
}

func (inc *Aroon) AroonUp() *AroonUp {
	out := &AroonUp{Aroon: inc}
	out.SeriesBase.Series = out
	return out
}

func (inc *Aroon) AroonDown() *AroonDown {
	out := &AroonDown{Aroon: inc}
	out.SeriesBase.Series = out
	return out
}

func (inc *Aroon) AroonOscillator() *AroonOscillator {
	out := &AroonOscillator{Aroon: inc}
	out.SeriesBase.Series = out
	return out
}

func (inc *Aroon) GetUp() types.SeriesExtend {
	return inc.AroonUp()
}

func (inc *Aroon) GetDown() types.SeriesExtend {
	return inc.AroonDown()
}

func (inc *Aroon) GetOscillator() types.SeriesExtend {
	return inc.AroonOscillator()
}

type AroonUp struct {
	types.SeriesBase
	*Aroon
}

func (inc *AroonUp) Last(i int) float64 {
	return inc.Up.Last(i)
}

func (inc *AroonUp) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *AroonUp) Length() int {
	return len(inc.Up)
}

var _ types.SeriesExtend = &AroonUp{}

type AroonDown struct {
	types.SeriesBase
	*Aroon
}

func (inc *AroonDown) Last(i int) float64 {
	return inc.Down.Last(i)
}

func (inc *AroonDown) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *AroonDown) Length() int {
	return len(inc.Down)
}

var _ types.SeriesExtend = &AroonDown{}

type AroonOscillator struct {
	types.SeriesBase
	*Aroon
}

func (inc *AroonOscillator) Last(i int) float64 {
	return inc.Oscillator.Last(i)
}

func (inc *AroonOscillator) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *AroonOscillator) Length() int {
	return len(inc.Oscillator)
}

var _ types.SeriesExtend = &AroonOscillator{}

func (inc *Aroon) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
		return
	}

	inc.Update(k.High.Float64(), k.Low.Float64())
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.LastUp(), inc.LastDown())
}

func (inc *Aroon) CalculateAndUpdate(kLines []types.KLine) {
	for _, k := range kLines {
		if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
			continue
		}

		inc.PushK(k)
	}

	if len(inc.Up) > 0 {
		inc.EmitUpdate(inc.LastUp(), inc.LastDown())
	}
	if len(kLines) > 0 {
		inc.EndTime = kLines[len(kLines)-1].EndTime.Time()
	}
}

func (inc *Aroon) handleKLineWindowUpdate(interval types.Interval, window types.KLineWindow) {
	if inc.Interval != interval {
		return
	}

	inc.CalculateAndUpdate(window)
}

func (inc *Aroon) Bind(updater KLineWindowUpdater) {
	updater.OnKLineWindowUpdate(inc.handleKLineWindowUpdate)
}
