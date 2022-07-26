package indicator

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type PivotLow
type PivotLow struct {
	types.IntervalWindow
	types.SeriesBase

	Lows    types.Float64Slice
	Values  types.Float64Slice
	EndTime time.Time

	updateCallbacks []func(value float64)
}

func (inc *PivotLow) Update(value float64) {
	if len(inc.Lows) == 0 {
		inc.SeriesBase.Series = inc
	}

	inc.Lows.Push(value)

	if len(inc.Lows) < inc.Window {
		return
	}

	low, err := calculatePivotLow(inc.Lows, inc.Window)
	if err != nil {
		log.WithError(err).Errorf("can not calculate pivot low")
		return
	}

	if low > 0.0 {
		inc.Values.Push(low)
	}
}

func (inc *PivotLow) PushK(k types.KLine) {
	if k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(k.Low.Float64())
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Last())
}


func calculatePivotLow(lows types.Float64Slice, window int) (float64, error) {
	length := len(lows)
	if length == 0 || length < window {
		return 0., fmt.Errorf("insufficient elements for calculating with window = %d", window)
	}

	var pv types.Float64Slice
	for _, low := range lows {
		pv.Push(low)
	}

	pl := 0.
	if lows.Min() == lows.Index(int(window/2.)-1) {
		pl = lows.Min()
	}

	return pl, nil
}
