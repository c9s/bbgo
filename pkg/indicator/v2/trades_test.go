package indicatorv2

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

func TestTradeStream_Length(t *testing.T) {
	s := &TradeStream{}
	assert.Equal(t, 0, s.Length())

	s.trades = append(s.trades, makeTrade(1, "BTCUSDT", 100))
	assert.Equal(t, 1, s.Length())

	s.trades = append(s.trades, makeTrade(2, "BTCUSDT", 200))
	assert.Equal(t, 2, s.Length())
}

func TestTradeStream_Last(t *testing.T) {
	s := &TradeStream{}

	// empty stream
	assert.Nil(t, s.Last(0))
	assert.Nil(t, s.Last(-1))

	s.trades = append(s.trades,
		makeTrade(1, "BTCUSDT", 100),
		makeTrade(2, "BTCUSDT", 200),
		makeTrade(3, "BTCUSDT", 300),
	)

	assert.Equal(t, uint64(3), s.Last(0).ID)
	assert.Equal(t, uint64(2), s.Last(1).ID)
	assert.Equal(t, uint64(1), s.Last(2).ID)

	// out of range
	assert.Nil(t, s.Last(3))
	assert.Nil(t, s.Last(-1))
}

func TestTradeStream_Tail(t *testing.T) {
	s := &TradeStream{}

	s.trades = append(s.trades,
		makeTrade(1, "BTCUSDT", 100),
		makeTrade(2, "BTCUSDT", 200),
		makeTrade(3, "BTCUSDT", 300),
	)

	// n < len: returns last n elements
	tail := s.Tail(2)
	assert.Len(t, tail, 2)
	assert.Equal(t, uint64(2), tail[0].ID)
	assert.Equal(t, uint64(3), tail[1].ID)

	// n == len: returns all
	tail = s.Tail(3)
	assert.Len(t, tail, 3)

	// n > len: returns all
	tail = s.Tail(100)
	assert.Len(t, tail, 3)
}

func TestTradeStream_BackFill(t *testing.T) {
	s := &TradeStream{}

	var received []types.Trade
	s.OnUpdate(func(trade types.Trade) {
		received = append(received, trade)
	})

	trades := []types.Trade{
		makeTrade(1, "BTCUSDT", 100),
		makeTrade(2, "BTCUSDT", 200),
	}
	s.BackFill(trades)

	assert.Equal(t, 2, s.Length())
	assert.Len(t, received, 2)
	assert.Equal(t, uint64(1), received[0].ID)
	assert.Equal(t, uint64(2), received[1].ID)
}

func TestTradeStream_AddSubscriber(t *testing.T) {
	s := &TradeStream{}

	var received []types.Trade
	s.AddSubscriber(func(trade types.Trade) {
		received = append(received, trade)
	})

	// no history — nothing replayed
	assert.Len(t, received, 0)

	// backfill existing trades and subscribe a second subscriber to verify history replay
	s.trades = append(s.trades,
		makeTrade(1, "BTCUSDT", 100),
		makeTrade(2, "BTCUSDT", 200),
	)

	var historical []types.Trade
	s.AddSubscriber(func(trade types.Trade) {
		historical = append(historical, trade)
	})

	// historical trades replayed to the new subscriber immediately
	assert.Len(t, historical, 2)
	assert.Equal(t, uint64(1), historical[0].ID)
	assert.Equal(t, uint64(2), historical[1].ID)

	// future updates arrive to all subscribers
	s.EmitUpdate(makeTrade(3, "BTCUSDT", 300))
	assert.Len(t, received, 1)
	assert.Equal(t, uint64(3), received[0].ID)
	assert.Len(t, historical, 3)
	assert.Equal(t, uint64(3), historical[2].ID)
}

func TestTrades_filtersSymbol(t *testing.T) {
	stream := &types.StandardStream{}
	ts := Trades(stream, "BTCUSDT")

	stream.EmitMarketTrade(makeTrade(1, "BTCUSDT", 100))
	stream.EmitMarketTrade(makeTrade(2, "ETHUSDT", 200)) // different symbol — should be ignored
	stream.EmitMarketTrade(makeTrade(3, "BTCUSDT", 300))

	assert.Equal(t, 2, ts.Length())
	assert.Equal(t, uint64(1), ts.Last(1).ID)
	assert.Equal(t, uint64(3), ts.Last(0).ID)
}

func TestTrades_emitsUpdates(t *testing.T) {
	stream := &types.StandardStream{}
	ts := Trades(stream, "BTCUSDT")

	var received []types.Trade
	ts.OnUpdate(func(trade types.Trade) {
		received = append(received, trade)
	})

	stream.EmitMarketTrade(makeTrade(1, "BTCUSDT", 100))
	stream.EmitMarketTrade(makeTrade(2, "BTCUSDT", 200))

	assert.Len(t, received, 2)
	assert.Equal(t, uint64(1), received[0].ID)
	assert.Equal(t, uint64(2), received[1].ID)
}

func TestTrades_shrinksSlice(t *testing.T) {
	stream := &types.StandardStream{}
	ts := Trades(stream, "BTCUSDT")

	// push MaxNumOfTrades + 1 trades to trigger the shrink
	for i := uint64(1); i <= MaxNumOfTrades+1; i++ {
		stream.EmitMarketTrade(makeTrade(i, "BTCUSDT", float64(i)))
	}

	// after shrinking, the length should be at most MaxNumOfTrades-(MaxNumOfTrades/5)
	maxAfterShrink := MaxNumOfTrades - MaxNumOfTrades/5 + 1
	assert.LessOrEqual(t, ts.Length(), maxAfterShrink)
}
