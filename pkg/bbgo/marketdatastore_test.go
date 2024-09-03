package bbgo

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestMarketDataStore_AddKLineAndTruncateWindow(t *testing.T) {
	store := NewMarketDataStore("BTCUSD")

	interval := types.Interval1s

	var maxCap int = 0
	capFixed := false

	var gid uint64 = 0
	// insert 1.5 * CapacityOfKLineWindowLimit KLine into window
	for ; gid < CapacityOfKLineWindowLimit+(CapacityOfKLineWindowLimit/2); gid++ {
		store.AddKLine(types.KLine{
			Interval: interval,
			GID:      gid,
		})

		// if the capacity is > CapacityOfKLineWindowLimit, the capacity should be fixed. We use this if expression to verify it then.
		if !capFixed && cap(*store.KLineWindows[interval]) > CapacityOfKLineWindowLimit {
			maxCap = cap(*store.KLineWindows[interval])
			capFixed = true
		}
	}

	window := store.KLineWindows[interval]

	// make sure the capacity is fixed
	assert.Equal(t, maxCap, cap(*window))

	// after truncate, it will remain (CapacityOfKLineWindowLimit / 2) KLine in the window
	// so the first GIC will be the maxCap - (CapacityOfKLineWindowLimit / 2)
	truncatedGID := uint64(maxCap - (CapacityOfKLineWindowLimit / 2))
	for _, kline := range *window {
		assert.Equal(t, truncatedGID, kline.GID)
		truncatedGID++
	}
}
