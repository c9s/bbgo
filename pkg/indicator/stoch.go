package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

const DPeriod int = 3

// Stochastic Oscillator
// - https://www.investopedia.com/terms/s/stochasticoscillator.asp
//
// The Stochastic Oscillator is a technical analysis indicator that is used to identify potential overbought or oversold conditions
// in a security's price. It is calculated by taking the current closing price of the security and comparing it to the high and low prices
// over a specified period of time. This comparison is then plotted as a line on the price chart, with values above 80 indicating overbought
// conditions and values below 20 indicating oversold conditions. The Stochastic Oscillator can be used by traders to identify potential
// entry and exit points for trades, or to confirm other technical analysis signals. It is typically used in conjunction with other indicators
// to provide a more comprehensive view of the security's price.

//go:generate callbackgen -type STOCH
type STOCH struct {
	types.IntervalWindow
	K floats.Slice
	D floats.Slice

	HighValues floats.Slice
	LowValues  floats.Slice

	EndTime         time.Time
	UpdateCallbacks []func(k float64, d float64)
}

func (inc *STOCH) Update(high, low, cloze float64) {
	inc.HighValues.Push(high)
	inc.LowValues.Push(low)

	lowest := inc.LowValues.Tail(inc.Window).Min()
	highest := inc.HighValues.Tail(inc.Window).Max()

	if highest == lowest {
		inc.K.Push(50.0)
	} else {
		k := 100.0 * (cloze - lowest) / (highest - lowest)
		inc.K.Push(k)
	}

	d := inc.K.Tail(DPeriod).Mean()
	inc.D.Push(d)
}

func (inc *STOCH) LastK() float64 {
	if len(inc.K) == 0 {
		return 0.0
	}
	return inc.K[len(inc.K)-1]
}

func (inc *STOCH) LastD() float64 {
	if len(inc.K) == 0 {
		return 0.0
	}
	return inc.D[len(inc.D)-1]
}

func (inc *STOCH) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
		return
	}

	inc.Update(k.High.Float64(), k.Low.Float64(), k.Close.Float64())
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.LastK(), inc.LastD())
}

func (inc *STOCH) GetD() types.Series {
	return &inc.D
}

func (inc *STOCH) GetK() types.Series {
	return &inc.K
}
