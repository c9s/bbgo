package bollmaker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/types"
)

func TestDynamicSpreadAmpSettings_update(t *testing.T) {
	ds := &DynamicSpreadAmpSettings{
		IntervalWindow: types.IntervalWindow{Interval: types.Interval1m, Window: 3},
	}
	ds.dynamicAskSpread = indicatorv2.SMA(nil, ds.Window)
	ds.dynamicBidSpread = indicatorv2.SMA(nil, ds.Window)

	upKLine := func(amp float64) types.KLine {
		return types.KLine{
			Open:  fixedpoint.NewFromFloat(100),
			Close: fixedpoint.NewFromFloat(101),
			High:  fixedpoint.NewFromFloat(100 + amp*100),
			Low:   fixedpoint.NewFromFloat(100),
		}
	}

	amps := []float64{0.01, 0.02, 0.03, 0.04, 0.05}
	for i, amp := range amps {
		ds.update(upKLine(amp))

		// the legacy SMA pushed a value only when the window was full,
		// and the spread was available only when it had pushed `window` values: 2 * window - 1 updates
		ready := ds.numOfFullWindowValues(ds.dynamicAskSpread) >= ds.Window
		assert.Equal(t, i+1 >= 2*ds.Window-1, ready, "update %d", i)
	}

	// the ask spread is the mean of the last 3 amplitudes, the bid spread is zero for the up klines
	assert.InDelta(t, (0.03+0.04+0.05)/3.0, ds.dynamicAskSpread.Last(0), 1e-9)
	assert.InDelta(t, 0.0, ds.dynamicBidSpread.Last(0), 1e-9)
}
