package bars

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func makeTrade(id uint64, symbol string, price float64) types.Trade {
	return types.Trade{
		ID:     id,
		Symbol: symbol,
		Price:  fixedpoint.NewFromFloat(price),
		Side:   types.SideTypeBuy,
	}
}

func TestTickBars_closesEveryNumTicks(t *testing.T) {
	stream := &types.StandardStream{}
	tb := TickBars(stream, "BTCUSDT", 3)

	var received []types.KLine
	tb.OnUpdate(func(k types.KLine) {
		received = append(received, k)
	})

	stream.EmitMarketTrade(makeTrade(1, "BTCUSDT", 100))
	stream.EmitMarketTrade(makeTrade(2, "BTCUSDT", 110))
	assert.Len(t, received, 0, "bar should not close before numTicks trades")

	stream.EmitMarketTrade(makeTrade(3, "BTCUSDT", 90))
	assert.Len(t, received, 1, "bar should close exactly at numTicks trades")

	k := received[0]
	assert.Equal(t, uint64(3), k.NumberOfTrades)
	assert.True(t, k.Open.Eq(fixedpoint.NewFromFloat(100)))
	assert.True(t, k.High.Eq(fixedpoint.NewFromFloat(110)))
	assert.True(t, k.Low.Eq(fixedpoint.NewFromFloat(90)))
	assert.True(t, k.Close.Eq(fixedpoint.NewFromFloat(90)))
	assert.True(t, k.Closed)
	assert.Equal(t, uint64(3), k.LastTradeID)

	// next bar starts fresh
	stream.EmitMarketTrade(makeTrade(4, "BTCUSDT", 200))
	assert.Len(t, received, 1, "new bar should not close after only 1 trade")
	assert.Equal(t, 1, tb.Length())
}

func TestTickBars_filtersSymbol(t *testing.T) {
	stream := &types.StandardStream{}
	tb := TickBars(stream, "BTCUSDT", 2)

	stream.EmitMarketTrade(makeTrade(1, "BTCUSDT", 100))
	stream.EmitMarketTrade(makeTrade(2, "ETHUSDT", 200)) // different symbol — should be ignored
	stream.EmitMarketTrade(makeTrade(3, "BTCUSDT", 300))

	assert.Equal(t, 1, tb.Length())
	assert.Equal(t, "BTCUSDT", tb.Last(0).Symbol)
}

func TestTickBars_accumulatesVolume(t *testing.T) {
	stream := &types.StandardStream{}
	tb := TickBars(stream, "BTCUSDT", 2)

	buy := makeTrade(1, "BTCUSDT", 100)
	buy.Quantity = fixedpoint.NewFromFloat(1.5)
	buy.QuoteQuantity = fixedpoint.NewFromFloat(150)
	buy.IsBuyer = true

	sell := makeTrade(2, "BTCUSDT", 100)
	sell.Quantity = fixedpoint.NewFromFloat(0.5)
	sell.QuoteQuantity = fixedpoint.NewFromFloat(50)
	sell.IsBuyer = false

	stream.EmitMarketTrade(buy)
	stream.EmitMarketTrade(sell)

	k := tb.Last(0)
	assert.True(t, k.Volume.Eq(fixedpoint.NewFromFloat(2)))
	assert.True(t, k.QuoteVolume.Eq(fixedpoint.NewFromFloat(200)))
	assert.True(t, k.TakerBuyBaseAssetVolume.Eq(fixedpoint.NewFromFloat(1.5)))
	assert.True(t, k.TakerBuyQuoteAssetVolume.Eq(fixedpoint.NewFromFloat(150)))
}

func TestTickBars_AddSubscriber(t *testing.T) {
	stream := &types.StandardStream{}
	tb := TickBars(stream, "BTCUSDT", 1)

	stream.EmitMarketTrade(makeTrade(1, "BTCUSDT", 100))

	var historical []types.KLine
	tb.AddSubscriber(func(k types.KLine) {
		historical = append(historical, k)
	})

	// existing bar replayed immediately to the new subscriber
	assert.Len(t, historical, 1)

	stream.EmitMarketTrade(makeTrade(2, "BTCUSDT", 200))
	assert.Len(t, historical, 2)
}

func TestTickBars_shrinksSlice(t *testing.T) {
	stream := &types.StandardStream{}
	tb := TickBars(stream, "BTCUSDT", 1)

	for i := uint64(1); i <= MaxNumOfTickBars+1; i++ {
		stream.EmitMarketTrade(makeTrade(i, "BTCUSDT", float64(i)))
	}

	maxAfterShrink := MaxNumOfTickBars - MaxNumOfTickBars/5 + 1
	assert.LessOrEqual(t, tb.Length(), maxAfterShrink)
}
