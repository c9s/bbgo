package bbgo

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

func TestIndicatorSet_closeCache(t *testing.T) {
	symbol := "BTCUSDT"
	store := NewMarketDataStore(symbol)
	store.KLineWindows[types.Interval1m] = &types.KLineWindow{
		{Open: number(19000.0), Close: number(19100.0)},
		{Open: number(19100.0), Close: number(19200.0)},
		{Open: number(19200.0), Close: number(19300.0)},
		{Open: number(19300.0), Close: number(19200.0)},
		{Open: number(19200.0), Close: number(19100.0)},
	}

	stream := types.NewStandardStream()

	indicatorSet := NewIndicatorSet(symbol, &stream, store)
	
	close1m := indicatorSet.CLOSE(types.Interval1m)
	assert.NotNil(t, close1m)

	close1m2 := indicatorSet.CLOSE(types.Interval1m)
	assert.Equal(t, close1m, close1m2)
}
