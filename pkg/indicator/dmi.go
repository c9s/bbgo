package indicator

import (
	"math"

	"github.com/c9s/bbgo/pkg/types"
)

// Refer: https://www.investopedia.com/terms/d/dmi.asp
// Refer: https://github.com/twopirllc/pandas-ta/blob/main/pandas_ta/trend/adx.py
//
// Directional Movement Index
//
// The Directional Movement Index (DMI) is a technical analysis indicator that is used to identify the direction and strength of a trend
// in a security's price. It was developed by J. Welles Wilder and is based on the concept of the +DI and -DI lines, which measure the strength
// of upward and downward price movements, respectively. The DMI is calculated by taking the difference between the +DI and -DI lines, and then
// smoothing the result using a moving average. This resulting line is called the Average Directional Index (ADX), and is used to identify whether
// a security is trending or not. If the ADX is above a certain threshold, typically 20, it indicates that the security is in a strong trend,
// and if it is below that threshold it indicates that the security is in a sideways or choppy market. The DMI can be used by traders to confirm
// the direction and strength of a trend, or to identify potential entry and exit points for trades.

//go:generate callbackgen -type DMI
type DMI struct {
	types.IntervalWindow

	ADXSmoothing      int
	atr               *ATR
	DMP               types.UpdatableSeriesExtend
	DMN               types.UpdatableSeriesExtend
	DIPlus            *types.Queue
	DIMinus           *types.Queue
	ADX               types.UpdatableSeriesExtend
	PrevHigh, PrevLow float64

	updateCallbacks []func(diplus, diminus, adx float64)
}

func (inc *DMI) Update(high, low, cloze float64) {
	if inc.DMP == nil || inc.DMN == nil {
		inc.DMP = &RMA{IntervalWindow: inc.IntervalWindow, Adjust: true}
		inc.DMN = &RMA{IntervalWindow: inc.IntervalWindow, Adjust: true}
		inc.ADX = &RMA{IntervalWindow: types.IntervalWindow{Window: inc.ADXSmoothing}, Adjust: true}
	}

	if inc.atr == nil {
		inc.atr = &ATR{IntervalWindow: inc.IntervalWindow}
		inc.atr.Update(high, low, cloze)
		inc.PrevHigh = high
		inc.PrevLow = low
		inc.DIPlus = types.NewQueue(500)
		inc.DIMinus = types.NewQueue(500)
		return
	}

	inc.atr.Update(high, low, cloze)
	up := high - inc.PrevHigh
	dn := inc.PrevLow - low
	inc.PrevHigh = high
	inc.PrevLow = low
	pos := 0.0
	if up > dn && up > 0. {
		pos = up
	}

	neg := 0.0
	if dn > up && dn > 0. {
		neg = dn
	}

	inc.DMP.Update(pos)
	inc.DMN.Update(neg)
	if inc.atr.Length() < inc.Window {
		return
	}
	k := 100. / inc.atr.Last(0)
	dmp := inc.DMP.Last(0)
	dmn := inc.DMN.Last(0)
	inc.DIPlus.Update(k * dmp)
	inc.DIMinus.Update(k * dmn)
	dx := 100. * math.Abs(dmp-dmn) / (dmp + dmn)
	inc.ADX.Update(dx)

}

func (inc *DMI) GetDIPlus() types.SeriesExtend {
	return inc.DIPlus
}

func (inc *DMI) GetDIMinus() types.SeriesExtend {
	return inc.DIMinus
}

func (inc *DMI) GetADX() types.SeriesExtend {
	return inc.ADX
}

func (inc *DMI) Length() int {
	return inc.ADX.Length()
}

func (inc *DMI) PushK(k types.KLine) {
	inc.Update(k.High.Float64(), k.Low.Float64(), k.Close.Float64())
}

func (inc *DMI) CalculateAndUpdate(allKLines []types.KLine) {
	last := allKLines[len(allKLines)-1]

	if inc.ADX == nil {
		for _, k := range allKLines {
			inc.PushK(k)
			inc.EmitUpdate(inc.DIPlus.Last(0), inc.DIMinus.Last(0), inc.ADX.Last(0))
		}
	} else {
		inc.PushK(last)
		inc.EmitUpdate(inc.DIPlus.Last(0), inc.DIMinus.Last(0), inc.ADX.Last(0))
	}
}
