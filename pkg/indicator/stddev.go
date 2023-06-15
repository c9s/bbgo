package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

const MaxNumOfStdev = 600
const MaxNumOfStdevTruncateSize = 300

//go:generate callbackgen -type StdDev
type StdDev struct {
	types.SeriesBase
	types.IntervalWindow
	Values    floats.Slice
	rawValues *types.Queue

	EndTime         time.Time
	updateCallbacks []func(value float64)
}

func (inc *StdDev) Last(i int) float64 {
	return inc.Values.Last(i)
}

func (inc *StdDev) Index(i int) float64 {
	return inc.Last(i)
}

func (inc *StdDev) Length() int {
	return inc.Values.Length()
}

var _ types.SeriesExtend = &StdDev{}

func (inc *StdDev) Update(value float64) {
	if inc.rawValues == nil {
		inc.rawValues = types.NewQueue(inc.Window)
		inc.SeriesBase.Series = inc
	}

	inc.rawValues.Update(value)

	var std = inc.rawValues.Stdev()
	inc.Values.Push(std)
	if len(inc.Values) > MaxNumOfStdev {
		inc.Values = inc.Values[MaxNumOfStdevTruncateSize-1:]
	}
}

func (inc *StdDev) PushK(k types.KLine) {
	inc.Update(k.Close.Float64())
	inc.EndTime = k.EndTime.Time()
}
