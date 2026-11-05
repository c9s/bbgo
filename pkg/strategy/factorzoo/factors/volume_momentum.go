package factorzoo

import (
	"fmt"
	"time"

	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

// quarterly volume momentum
// assume that the quotient of volume SMA over latest volume will dynamically revert into one.
// so this fraction value is our alpha, PMR

type VMOM struct {
	*types.Float64Series

	window  int
	volumes *types.Queue

	EndTime time.Time
}

var _ types.SeriesExtend = &VMOM{}

// NewVMOM creates the volume momentum factor stream from the given kline source.
// No value is emitted until window klines have been received.
func NewVMOM(source indicatorv2.KLineSubscription, window int) *VMOM {
	inc := &VMOM{
		Float64Series: types.NewFloat64Series(),
		window:        window,
		volumes:       types.NewQueue(window),
	}
	source.AddSubscriber(inc.PushK)
	return inc
}

func (inc *VMOM) Update(volume float64) {
	inc.volumes.Update(volume)
	if inc.volumes.Length() >= inc.window {
		v := inc.volumes.Last(0) / inc.volumes.Mean()
		inc.PushAndEmit(v)
		inc.Slice = types.ShrinkSlice(inc.Slice, indicatorv2.MaxSliceSize, indicatorv2.TruncateSize)
	}
}

func (inc *VMOM) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(k.Volume.Float64())
	inc.EndTime = k.EndTime.Time()
}

func calculateVolumeMomentum(klines []types.KLine, window int, valV KLineValueMapper, valP KLineValueMapper) (float64, error) {
	length := len(klines)
	if length == 0 || length < window {
		return 0.0, fmt.Errorf("insufficient elements for calculating VOL with window = %d", window)
	}

	vma := 0.
	for _, p := range klines[length-window : length-1] {
		vma += valV(p)
	}
	vma /= float64(window)
	momentum := valV(klines[length-1]) / vma // * (valP(klines[length-1-2]) / valP(klines[length-1]))

	return momentum, nil
}
