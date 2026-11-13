package indicatorv2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_PivotSupertrend(t *testing.T) {
	const (
		window        = 10
		pivotWindow   = 3
		atrMultiplier = 1.5
	)

	kLines := randomWalkKLines(500, 5)

	stream := &KLineStream{}
	pst := PivotSupertrend(stream, window, pivotWindow, atrMultiplier)

	legacy := &indicator.PivotSupertrend{
		IntervalWindow:   types.IntervalWindow{Window: window},
		ATRMultiplier:    atrMultiplier,
		PivotWindow:      pivotWindow,
		AverageTrueRange: &indicator.ATR{IntervalWindow: types.IntervalWindow{Window: window}},
		PivotLow:         &indicator.PivotLow{IntervalWindow: types.IntervalWindow{Window: pivotWindow}},
		PivotHigh:        &indicator.PivotHigh{IntervalWindow: types.IntervalWindow{Window: pivotWindow}},
	}

	var ups, downs int
	for i, k := range kLines {
		stream.BackFill([]types.KLine{k})
		legacy.PushK(k)

		require.Equal(t, legacy.Length(), pst.Length(), "length at kline %d", i)
		assert.InDelta(t, legacy.Last(0), pst.Last(0), 1e-9, "trend price at kline %d", i)
		assert.InDelta(t, legacy.LastSupertrendSupport(), pst.LastSupertrendSupport(), 1e-9)
		assert.InDelta(t, legacy.LastSupertrendResistance(), pst.LastSupertrendResistance(), 1e-9)
		assert.Equal(t, legacy.Direction(), pst.Direction(), "direction at kline %d", i)
		assert.Equal(t, legacy.GetSignal(), pst.GetSignal(), "signal at kline %d", i)

		switch pst.GetSignal() {
		case types.DirectionUp:
			ups++
		case types.DirectionDown:
			downs++
		}
	}

	// the first values are only emitted after the first pivot is found
	assert.Less(t, pst.Length(), len(kLines))
	assert.Greater(t, pst.Length(), 0)

	// make sure the data set actually exercises trend flips
	assert.Greater(t, ups, 0)
	assert.Greater(t, downs, 0)
}
