package bbgo

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestMarketDataStore_AddKLineAndTruncateWindow(t *testing.T) {
	store := NewMarketDataStore("BTCUSD")

	interval := types.Interval1s

	capFixed := false

	var gid uint64 = 0
	for ; gid < KLineWindowCapacityLimit*2; gid++ {
		store.AddKLine(types.KLine{
			Interval: interval,
			GID:      gid,
		})

		// if the capacity is > KLineWindowCapacityLimit, the capacity should be fixed. We use this if expression to verify it then.
		if !capFixed && cap(*store.KLineWindows[interval]) > KLineWindowCapacityLimit {
			capFixed = true
		}
	}

	window := store.KLineWindows[interval]

	// make sure the capacity is fixed
	assert.Equal(t, KLineWindowCapacityLimit, cap(*window))

	// after truncate, it will remain (KLineWindowCapacityLimit / 2) KLine in the window
	// so the first GIC will be the maxCap - (KLineWindowCapacityLimit / 2)
	expectedGID := KLineWindowCapacityLimit*2 - KLineWindowShrinkSize
	for i, kline := range *window {
		assert.Equalf(t, expectedGID, int(kline.GID), "idx: %d", i)
		expectedGID++
	}
}
