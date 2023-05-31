package indicator

import (
	"github.com/c9s/bbgo/pkg/types"
)

// Refer: Ease of Movement
// Refer URL: https://www.investopedia.com/terms/e/easeofmovement.asp
// The Ease of Movement (EOM) is a technical analysis indicator that is used to measure the relationship between the volume of a security
// and its price movement. It is calculated by dividing the difference between the high and low prices of the security by the total
// volume of the security over a specified period of time, and then multiplying the result by a constant factor. This resulting value is
// then plotted on the price chart as a line, which can be used to make predictions about future price movements. The EOM is typically
// used to identify periods of high or low trading activity, and can be used to confirm other technical analysis signals.

//go:generate callbackgen -type EMV
type EMV struct {
	types.SeriesBase
	types.IntervalWindow

	prevH    float64
	prevL    float64
	Values   *SMA
	EMVScale float64

	UpdateCallbacks []func(value float64)
}

const DefaultEMVScale float64 = 100000000.

func (inc *EMV) Update(high, low, vol float64) {
	if inc.EMVScale == 0 {
		inc.EMVScale = DefaultEMVScale
	}

	if inc.prevH == 0 || inc.Values == nil {
		inc.SeriesBase.Series = inc
		inc.prevH = high
		inc.prevL = low
		inc.Values = &SMA{IntervalWindow: inc.IntervalWindow}
		return
	}

	distanceMoved := (high+low)/2. - (inc.prevH+inc.prevL)/2.
	boxRatio := vol / inc.EMVScale / (high - low)
	result := distanceMoved / boxRatio
	inc.prevH = high
	inc.prevL = low
	inc.Values.Update(result)
}

func (inc *EMV) Last(i int) float64 {
	if inc.Values == nil {
		return 0
	}

	return inc.Values.Last(i)
}

func (inc *EMV) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *EMV) Length() int {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Length()
}

var _ types.SeriesExtend = &EMV{}

func (inc *EMV) PushK(k types.KLine) {
	inc.Update(k.High.Float64(), k.Low.Float64(), k.Volume.Float64())
}
