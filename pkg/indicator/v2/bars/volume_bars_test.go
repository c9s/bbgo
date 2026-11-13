package bars

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func makeVolumeTrade(id uint64, symbol string, price, qty float64) types.Trade {
	p := fixedpoint.NewFromFloat(price)
	q := fixedpoint.NewFromFloat(qty)
	return types.Trade{
		ID:            id,
		Symbol:        symbol,
		Price:         p,
		Quantity:      q,
		QuoteQuantity: p.Mul(q),
		Side:          types.SideTypeBuy,
	}
}

func TestVolumeBars_closesAtBarVolume(t *testing.T) {
	stream := &types.StandardStream{}
	vb := VolumeBars(stream, "BTCUSDT", fixedpoint.NewFromFloat(10))

	var received []types.KLine
	vb.OnUpdate(func(k types.KLine) {
		received = append(received, k)
	})

	stream.EmitMarketTrade(makeVolumeTrade(1, "BTCUSDT", 100, 4))
	stream.EmitMarketTrade(makeVolumeTrade(2, "BTCUSDT", 110, 4))
	assert.Len(t, received, 0, "bar should not close before barVolume is reached")

	// exactly fills the bar: 4 + 4 + 2 == 10
	stream.EmitMarketTrade(makeVolumeTrade(3, "BTCUSDT", 90, 2))
	assert.Len(t, received, 1, "bar should close exactly at barVolume")

	k := received[0]
	assert.True(t, k.Volume.Eq(fixedpoint.NewFromFloat(10)))
	assert.Equal(t, uint64(3), k.NumberOfTrades)
	assert.True(t, k.Open.Eq(fixedpoint.NewFromFloat(100)))
	assert.True(t, k.High.Eq(fixedpoint.NewFromFloat(110)))
	assert.True(t, k.Low.Eq(fixedpoint.NewFromFloat(90)))
	assert.True(t, k.Close.Eq(fixedpoint.NewFromFloat(90)))
	assert.True(t, k.Closed)
	assert.Equal(t, uint64(3), k.LastTradeID)
}

func TestVolumeBars_splitsCrossingTrade(t *testing.T) {
	stream := &types.StandardStream{}
	vb := VolumeBars(stream, "BTCUSDT", fixedpoint.NewFromFloat(10))

	var received []types.KLine
	vb.OnUpdate(func(k types.KLine) {
		received = append(received, k)
	})

	stream.EmitMarketTrade(makeVolumeTrade(1, "BTCUSDT", 100, 6))
	assert.Len(t, received, 0)

	// this trade (qty 8) crosses the 10 threshold: 4 fills bar 1, 4 opens bar 2
	stream.EmitMarketTrade(makeVolumeTrade(2, "BTCUSDT", 200, 8))
	assert.Len(t, received, 1, "crossing trade should close exactly one bar")

	bar1 := received[0]
	assert.True(t, bar1.Volume.Eq(fixedpoint.NewFromFloat(10)), "bar1 volume should be capped at barVolume")
	assert.Equal(t, uint64(2), bar1.NumberOfTrades, "the splitting trade still counts toward bar1")
	assert.True(t, bar1.Close.Eq(fixedpoint.NewFromFloat(200)))
	assert.True(t, bar1.High.Eq(fixedpoint.NewFromFloat(200)))

	current := vb.current
	assert.True(t, current.Volume.Eq(fixedpoint.NewFromFloat(4)), "current bar should have the remaining volume from the split trade")
	assert.True(t, current.Open.Eq(fixedpoint.NewFromFloat(200)), "current bar should open with the price of the splitting trade")
	assert.True(t, current.High.Eq(fixedpoint.NewFromFloat(200)), "current bar high should reflect the splitting trade price")
	assert.True(t, current.Low.Eq(fixedpoint.NewFromFloat(200)), "current bar low should reflect the splitting trade price")
	assert.False(t, current.Closed, "current bar should not be closed yet")
	assert.Equal(t, uint64(1), current.NumberOfTrades, "current bar should have 1 trades (split remainder)")

	// expected quote volume: 6*100 + 4*200 = 1400
	assert.True(t, bar1.QuoteVolume.Eq(fixedpoint.NewFromFloat(1400)), "quote volume for bar1: %s", bar1.QuoteVolume.String())

	assert.Equal(t, 1, vb.Length(), "second bar should still be open, not emitted yet")

	// close bar 2: needs 6 more volume (4 already in from the split + 6 = 10)
	stream.EmitMarketTrade(makeVolumeTrade(3, "BTCUSDT", 300, 6))
	assert.Len(t, received, 2)

	bar2 := received[1]
	assert.True(t, bar2.Volume.Eq(fixedpoint.NewFromFloat(10)))
	assert.Equal(t, uint64(2), bar2.NumberOfTrades, "split remainder (trade 2) + trade 3 == 2 trades")
	assert.True(t, bar2.Open.Eq(fixedpoint.NewFromFloat(200)), "bar2 opens with the price of the splitting trade")
	assert.Equal(t, uint64(3), bar2.LastTradeID)
}

func TestVolumeBars_tradeSpansMultipleBars(t *testing.T) {
	stream := &types.StandardStream{}
	vb := VolumeBars(stream, "BTCUSDT", fixedpoint.NewFromFloat(10))

	var received []types.KLine
	vb.OnUpdate(func(k types.KLine) {
		received = append(received, k)
	})

	// a single trade of qty 25 should close 2 full bars and leave 5 open in a third
	stream.EmitMarketTrade(makeVolumeTrade(1, "BTCUSDT", 100, 25))

	assert.Len(t, received, 2)
	assert.True(t, received[0].Volume.Eq(fixedpoint.NewFromFloat(10)))
	assert.True(t, received[1].Volume.Eq(fixedpoint.NewFromFloat(10)))
	assert.Equal(t, 2, vb.Length())
}

func TestVolumeBars_filtersSymbol(t *testing.T) {
	stream := &types.StandardStream{}
	vb := VolumeBars(stream, "BTCUSDT", fixedpoint.NewFromFloat(5))

	stream.EmitMarketTrade(makeVolumeTrade(1, "BTCUSDT", 100, 5))
	stream.EmitMarketTrade(makeVolumeTrade(2, "ETHUSDT", 200, 5)) // different symbol — should be ignored
	stream.EmitMarketTrade(makeVolumeTrade(3, "BTCUSDT", 300, 5))

	assert.Equal(t, 2, vb.Length())
	assert.Equal(t, "BTCUSDT", vb.Last(0).Symbol)
}

func TestVolumeBars_AddSubscriber(t *testing.T) {
	stream := &types.StandardStream{}
	vb := VolumeBars(stream, "BTCUSDT", fixedpoint.NewFromFloat(1))

	stream.EmitMarketTrade(makeVolumeTrade(1, "BTCUSDT", 100, 1))

	var historical []types.KLine
	vb.AddSubscriber(func(k types.KLine) {
		historical = append(historical, k)
	})

	// existing bar replayed immediately to the new subscriber
	assert.Len(t, historical, 1)

	stream.EmitMarketTrade(makeVolumeTrade(2, "BTCUSDT", 200, 1))
	assert.Len(t, historical, 2)
}

func TestVolumeBars_shrinksSlice(t *testing.T) {
	stream := &types.StandardStream{}
	vb := VolumeBars(stream, "BTCUSDT", fixedpoint.NewFromFloat(1))

	for i := uint64(1); i <= MaxNumOfVolumeBars+1; i++ {
		stream.EmitMarketTrade(makeVolumeTrade(i, "BTCUSDT", float64(i), 1))
	}

	maxAfterShrink := MaxNumOfVolumeBars - MaxNumOfVolumeBars/5 + 1
	assert.LessOrEqual(t, vb.Length(), maxAfterShrink)
}
