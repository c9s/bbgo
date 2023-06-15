package indicator

import (
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

// Parabolic SAR(Stop and Reverse) / SAR
// Refer: https://www.investopedia.com/terms/p/parabolicindicator.asp
// The parabolic SAR indicator, developed by J. Wells Wilder, is used by traders to determine
// trend direction and potential reversals in price. The indicator uses a trailing stop and
// reverse method called "SAR," or stop and reverse, to identify suitable exit and entry points.
// Traders also refer to the indicator as to the parabolic stop and reverse, parabolic SAR, or PSAR.
//
// The parabolic SAR indicator appears on a chart as a series of dots, either above or below an asset's
// price, depending on the direction the price is moving. A dot is placed below the price when it is
// trending upward, and above the price when it is trending downward.

//go:generate callbackgen -type PSAR
type PSAR struct {
	types.SeriesBase
	types.IntervalWindow
	High    *types.Queue
	Low     *types.Queue
	Values  floats.Slice // Stop and Reverse
	AF      float64      // Acceleration Factor
	EP      float64
	Falling bool

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

func (inc *PSAR) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *PSAR) Length() int {
	return len(inc.Values)
}

func (inc *PSAR) falling() bool {
	up := inc.High.Last(0) - inc.High.Index(1)
	dn := inc.Low.Index(1) - inc.Low.Last(0)
	return (dn > up) && (dn > 0)
}

func (inc *PSAR) Update(high, low float64) {
	if inc.High == nil {
		inc.SeriesBase.Series = inc
		inc.High = types.NewQueue(inc.Window)
		inc.Low = types.NewQueue(inc.Window)
		inc.Values = floats.Slice{}
		inc.AF = 0.02
		inc.High.Update(high)
		inc.Low.Update(low)
		return
	}
	isFirst := inc.High.Length() < inc.Window
	inc.High.Update(high)
	inc.Low.Update(low)
	if !isFirst {
		ppsar := inc.Values.Last(0)
		if inc.Falling { // falling formula
			psar := ppsar - inc.AF*(ppsar-inc.EP)
			h := inc.High.Shift(1).Highest(2)
			inc.Values.Push(math.Max(psar, h))
			if low < inc.EP {
				inc.EP = low
				if inc.AF <= 0.18 {
					inc.AF += 0.02
				}
			}
			if high > psar { // reverse
				inc.AF = 0.02
				inc.Values[len(inc.Values)-1] = inc.EP
				inc.EP = high
				inc.Falling = false
			}
		} else { // rising formula
			psar := ppsar + inc.AF*(inc.EP-ppsar)
			l := inc.Low.Shift(1).Lowest(2)
			inc.Values.Push(math.Min(psar, l))
			if high > inc.EP {
				inc.EP = high
				if inc.AF <= 0.18 {
					inc.AF += 0.02
				}
			}
			if low < psar { // reverse
				inc.AF = 0.02
				inc.Values[len(inc.Values)-1] = inc.EP
				inc.EP = low
				inc.Falling = true
			}

		}
	} else {
		inc.Falling = inc.falling()
		if inc.Falling {
			inc.Values.Push(inc.High.Index(1))
			inc.EP = inc.Low.Index(1)
		} else {
			inc.Values.Push(inc.Low.Index(1))
			inc.EP = inc.High.Index(1)
		}
	}
}

var _ types.SeriesExtend = &PSAR{}

func (inc *PSAR) PushK(k types.KLine) {
	inc.Update(k.High.Float64(), k.Low.Float64())
}

func (inc *PSAR) BindK(target KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}
