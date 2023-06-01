package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

/*
ad implements accumulation/distribution indicator

Accumulation/Distribution Indicator (A/D)
- https://www.investopedia.com/terms/a/accumulationdistribution.asp
*/
//go:generate callbackgen -type AD
type AD struct {
	types.SeriesBase
	types.IntervalWindow
	Values   floats.Slice
	PrePrice float64

	EndTime         time.Time
	UpdateCallbacks []func(value float64)
}

func (inc *AD) Update(high, low, cloze, volume float64) {
	if len(inc.Values) == 0 {
		inc.SeriesBase.Series = inc
	}
	var moneyFlowVolume float64
	if high == low {
		moneyFlowVolume = 0
	} else {
		moneyFlowVolume = ((2*cloze - high - low) / (high - low)) * volume
	}

	ad := inc.Last(0) + moneyFlowVolume
	inc.Values.Push(ad)
}

func (inc *AD) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *AD) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *AD) Length() int {
	return len(inc.Values)
}

var _ types.SeriesExtend = &AD{}

func (inc *AD) CalculateAndUpdate(kLines []types.KLine) {
	for _, k := range kLines {
		if inc.EndTime != zeroTime && !k.EndTime.After(inc.EndTime) {
			continue
		}
		inc.Update(k.High.Float64(), k.Low.Float64(), k.Close.Float64(), k.Volume.Float64())
	}

	inc.EmitUpdate(inc.Last(0))
	inc.EndTime = kLines[len(kLines)-1].EndTime.Time()
}
