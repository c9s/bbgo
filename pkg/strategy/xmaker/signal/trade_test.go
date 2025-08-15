package signal

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

var tradeId = 0

// newFakeTrade creates a test trade.
func newFakeTrade(symbol string, side types.SideType, price, quantity fixedpoint.Value, t time.Time) types.Trade {
	tradeId++
	return types.Trade{
		ID:       uint64(tradeId),
		Symbol:   symbol,
		Side:     side,
		Price:    price,
		IsBuyer:  side == types.SideTypeBuy,
		Quantity: quantity,
		Time:     types.Time(t),
	}
}

func TestMarketTradeWindowSignal_NoDecay(t *testing.T) {
	now := time.Now()
	symbol := "BTCUSDT"
	sig := &TradeVolumeWindowSignal{
		symbol:    symbol,
		Threshold: fixedpoint.NewFromFloat(0.10),
		Window:    types.Duration(time.Minute),
	}
	sig.SetLogger(logrus.New())

	t.Run("mid strength long", func(t *testing.T) {
		// Setup ring buffer manually
		sig.trades = make([]types.Trade, tradeSliceCapacityLimit)
		// Directly insert test trades into ring buffer positions
		sig.trades[0] = newFakeTrade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), now.Add(-2*time.Minute))
		sig.trades[1] = newFakeTrade(symbol, types.SideTypeSell, Number(18000.0), Number(0.5), now.Add(-2*time.Second))
		sig.trades[2] = newFakeTrade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), now.Add(-1*time.Second))
		sig.start = 0
		sig.count = 3

		ctx := context.Background()
		sigNum, err := sig.CalculateSignal(ctx)
		if assert.NoError(t, err) {
			assert.InDelta(t, 0.6666666, sigNum, 0.0001)
		}

		// instead of checking len(sig.trades), verify the internal count
		assert.Equal(t, 2, sig.count) // changed check to validate the ring buffer count
	})

	t.Run("strong long", func(t *testing.T) {
		// Setup ring buffer manually
		sig.trades = make([]types.Trade, tradeSliceCapacityLimit)
		// Directly insert test trades into ring buffer positions
		sig.trades[0] = newFakeTrade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), now.Add(-2*time.Minute))
		sig.trades[1] = newFakeTrade(symbol, types.SideTypeBuy, Number(18000.0), Number(0.5), now.Add(-2*time.Second))
		sig.trades[2] = newFakeTrade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), now.Add(-1*time.Second))
		sig.start = 0
		sig.count = 3

		ctx := context.Background()
		sigNum, err := sig.CalculateSignal(ctx)
		if assert.NoError(t, err) {
			assert.InDelta(t, 2.0, sigNum, 0.0001)
		}

		// instead of checking len(sig.trades), verify the internal count
		assert.Equal(t, 2, sig.count) // changed check to validate the ring buffer count
	})

	t.Run("strong short", func(t *testing.T) {
		// Setup ring buffer manually
		sig.trades = make([]types.Trade, tradeSliceCapacityLimit)
		// Directly insert test trades into ring buffer positions
		sig.trades[0] = newFakeTrade(symbol, types.SideTypeSell, Number(18000.0), Number(1.0), now.Add(-2*time.Minute))
		sig.trades[1] = newFakeTrade(symbol, types.SideTypeSell, Number(18000.0), Number(0.5), now.Add(-2*time.Second))
		sig.trades[2] = newFakeTrade(symbol, types.SideTypeSell, Number(18000.0), Number(1.0), now.Add(-1*time.Second))
		sig.start = 0
		sig.count = 3

		ctx := context.Background()
		sigNum, err := sig.CalculateSignal(ctx)
		if assert.NoError(t, err) {
			assert.InDelta(t, -2.0, sigNum, 0.0001)
		}

		// instead of checking len(sig.trades), verify the internal count
		assert.Equal(t, 2, sig.count) // changed check to validate the ring buffer count
	})
}

func TestMarketTradeWindowSignal_ExceedCapacity(t *testing.T) {
	symbol := "BTCUSDT"
	sig := &TradeVolumeWindowSignal{
		symbol:    symbol,
		Threshold: fixedpoint.NewFromFloat(0.65),
		Window:    types.Duration(time.Minute),
	}
	sig.SetLogger(logrus.New())

	// Preallocate the buffer and simulate a full ring buffer.
	sig.trades = make([]types.Trade, tradeSliceCapacityLimit)
	sig.start = 0
	sig.count = tradeSliceCapacityLimit
	// Fill the ring buffer with a dummy trade.
	dummyTrade := newFakeTrade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), time.Now().Add(-2*time.Minute))
	for i := 0; i < tradeSliceCapacityLimit; i++ {
		sig.trades[i] = dummyTrade
	}
	// Add one more trade to exceed the capacity.
	newTrade := newFakeTrade(symbol, types.SideTypeSell, Number(18005.0), Number(0.5), time.Now())
	sig.handleTrade(newTrade)
	// After overwriting, start should have advanced by 1, and count remains at capacity.
	assert.Equal(t, tradeSliceCapacityLimit, sig.count)
	assert.Equal(t, 1, sig.start)
	// Verify that the overwritten position contains the new trade.
	assert.Equal(t, newTrade.Time, sig.trades[0].Time)
}

func TestTradeVolumeWindowSignal_FilterTrades(t *testing.T) {
	now := time.Now()
	symbol := "BTCUSDT"
	sig := &TradeVolumeWindowSignal{
		symbol:    symbol,
		Threshold: fixedpoint.NewFromFloat(0.65),
		Window:    types.Duration(time.Minute),
	}
	sig.SetLogger(logrus.New())

	// Preallocate fixed capacity buffer.
	sig.trades = make([]types.Trade, tradeSliceCapacityLimit)
	// Insert trades with sequential timestamps.
	tradeOld := newFakeTrade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), now.Add(-3*time.Minute))
	tradeMid := newFakeTrade(symbol, types.SideTypeSell, Number(18000.0), Number(0.5), now.Add(-90*time.Second))
	tradeNew := newFakeTrade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), now)
	sig.trades[0] = tradeOld
	sig.trades[1] = tradeMid
	sig.trades[2] = tradeNew
	sig.start = 0
	sig.count = 3

	// Filter trades from the last 2 minutes.
	filtered := sig.filterTrades(now.Add(-2 * time.Minute))
	// Expect tradeOld (3 minutes ago) to be removed.
	assert.Equal(t, 2, len(filtered))
	assert.Equal(t, tradeMid.Time, filtered[0].Time)
	assert.Equal(t, tradeNew.Time, filtered[1].Time)
	// Verify the ring buffer internal state is updated.
	assert.Equal(t, 1, sig.start)
	assert.Equal(t, 2, sig.count)

	// Subtest: simulate wrap-around when ring buffer exceeds capacity limit.
	t.Run("wrap-around case", func(t *testing.T) {
		// Use a smaller test scenario with wrap-around.
		// We'll simulate a ring buffer scenario where start is near the end and count spans the end and beginning.
		sig = &TradeVolumeWindowSignal{
			symbol:    symbol,
			Threshold: fixedpoint.NewFromFloat(0.65),
			Window:    types.Duration(time.Minute),
		}
		// Preallocate fixed capacity buffer.
		sig.trades = make([]types.Trade, tradeSliceCapacityLimit)
		// Manually set start and count to simulate wrap-around.
		// For example, let start = tradeSliceCapacityLimit - 3 and count = 5.
		sig.start = tradeSliceCapacityLimit - 3
		sig.count = 5
		// Fill trades with wrap-around indices.
		// Index: [tradeSliceCapacityLimit-3, tradeSliceCapacityLimit-2, tradeSliceCapacityLimit-1, 0, 1]
		tradeA := newFakeTrade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), now.Add(-3*time.Minute))   // old trade (to be filtered out)
		tradeB := newFakeTrade(symbol, types.SideTypeSell, Number(18000.0), Number(0.5), now.Add(-90*time.Second)) // qualifies
		tradeC := newFakeTrade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), now.Add(-30*time.Second))  // qualifies
		tradeD := newFakeTrade(symbol, types.SideTypeSell, Number(18000.0), Number(0.5), now.Add(-20*time.Second)) // qualifies
		tradeE := newFakeTrade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), now.Add(-10*time.Second))  // qualifies
		sig.trades[sig.start] = tradeA
		sig.trades[(sig.start+1)%tradeSliceCapacityLimit] = tradeB
		sig.trades[(sig.start+2)%tradeSliceCapacityLimit] = tradeC
		sig.trades[(sig.start+3)%tradeSliceCapacityLimit] = tradeD
		sig.trades[(sig.start+4)%tradeSliceCapacityLimit] = tradeE

		// Set cutoff to now - 2 minutes; tradeA should be filtered out.
		filteredWrap := sig.filterTrades(now.Add(-2 * time.Minute))
		// Expect remaining trades: tradeB, tradeC, tradeD, tradeE
		assert.Equal(t, 4, len(filteredWrap))
		assert.Equal(t, tradeB.Time, filteredWrap[0].Time)
		assert.Equal(t, tradeC.Time, filteredWrap[1].Time)
		assert.Equal(t, tradeD.Time, filteredWrap[2].Time)
		assert.Equal(t, tradeE.Time, filteredWrap[3].Time)
		// Verify the ring buffer internal state: new start should equal (original start+1) mod capacity, and count equals 4.
		expectedStart := (tradeSliceCapacityLimit - 3 + 1) % tradeSliceCapacityLimit
		assert.Equal(t, expectedStart, sig.start)
		assert.Equal(t, 4, sig.count)
	})
}

func TestMarketTradeWindowSignal_WithDecay(t *testing.T) {
	symbol := "BTCUSDT"

	t.Run("3min with 0.0001", func(t *testing.T) {
		// set up the signal with a 3-minute window so all trades are within range
		now := time.Now()
		sig := &TradeVolumeWindowSignal{
			symbol:    symbol,
			Threshold: fixedpoint.NewFromFloat(0.10),
			Window:    types.Duration(3 * time.Minute),
			DecayRate: 0.02, // decay rate per second
		}
		sig.SetLogger(logrus.New())

		// Preallocate the buffer
		sig.trades = make([]types.Trade, tradeSliceCapacityLimit)

		trade1 := newFakeTrade(symbol, types.SideTypeSell, Number(18000.0), Number(1.0), now.Add(-59*3*time.Second))
		trade2 := newFakeTrade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), now.Add(-20*time.Second))
		trade3 := newFakeTrade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), now.Add(-10*time.Second))
		trade4 := newFakeTrade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), now.Add(-1*time.Second))
		sig.trades[0] = trade1
		sig.trades[1] = trade2
		sig.trades[2] = trade3
		sig.trades[3] = trade4
		sig.start = 0
		sig.count = 4

		ctx := context.Background()
		sigNum, err := sig.CalculateSignal(ctx)
		assert.NoError(t, err)

		assert.InDelta(t, 1.953, sigNum, 0.001)
		assert.Equal(t, 4, sig.count)
	})

	t.Run("3min", func(t *testing.T) {
		// set up the signal with a 3-minute window so all trades are within range
		now := time.Now()
		sig := &TradeVolumeWindowSignal{
			symbol:    symbol,
			Threshold: fixedpoint.NewFromFloat(0.10),
			Window:    types.Duration(3 * time.Minute),
			DecayRate: 0.05, // decay rate per second
		}
		sig.SetLogger(logrus.New())

		// Preallocate the buffer
		sig.trades = make([]types.Trade, tradeSliceCapacityLimit)
		// Insert three trades:
		// trade1: buy trade 2 minutes ago, weight = exp(-0.05*120) ~ 0.00248
		// trade2: sell trade 20 seconds ago, weight = exp(-0.05*20) ~ 0.36788
		// trade3: buy trade 1 second ago, weight = exp(-0.05*1) ~ 0.95123
		trade1 := newFakeTrade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), now.Add(-2*time.Minute))
		trade2 := newFakeTrade(symbol, types.SideTypeSell, Number(18000.0), Number(0.5), now.Add(-20*time.Second))
		trade3 := newFakeTrade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), now.Add(-1*time.Second))
		sig.trades[0] = trade1
		sig.trades[1] = trade2
		sig.trades[2] = trade3
		sig.start = 0
		sig.count = 3

		ctx := context.Background()
		sigNum, err := sig.CalculateSignal(ctx)
		assert.NoError(t, err)

		// Expected decayed volumes:
		// buyVolume  = 1.0*exp(-6) + 1.0*exp(-0.05) ≈ 0.00248 + 0.95123 = 0.95371
		// sellVolume = 0.5*exp(-1) ≈ 0.18394
		// signal = (0.95371 - 0.18394) / (0.95371 + 0.18394) = 0.677 (approx)
		// final signal = 0.677 * 2 = 1.354 (approx)
		assert.InDelta(t, 1.354, sigNum, 0.01)
		// Verify ring buffer internal state updated (from filterTrades)
		assert.Equal(t, 3, sig.count)
	})

}

func TestMarketTradeWindowSignal_WithFrequency(t *testing.T) {
	now := time.Now()
	symbol := "BTCUSDT"
	// Set up the signal with a 3-minute window so all trades are within range.
	// Enable frequency consideration by setting ConsiderFreq to true.
	sig := &TradeVolumeWindowSignal{
		symbol:       symbol,
		Threshold:    fixedpoint.NewFromFloat(0.10),
		Window:       types.Duration(3 * time.Minute),
		DecayRate:    0.05, // decay rate per second,
		ConsiderFreq: true,
		Alpha:        1.0,
		Beta:         1.0,
	}
	sig.SetLogger(logrus.New())

	// Preallocate the ring buffer.
	sig.trades = make([]types.Trade, tradeSliceCapacityLimit)
	// Insert three trades with known timestamps.
	// trade1: buy 2 minutes ago, weight = exp(-0.05*120) ≈ 0.00248.
	// trade2: sell 20 seconds ago, weight = exp(-0.05*20) ≈ 0.36788.
	// trade3: buy 1 second ago, weight = exp(-0.05*1) ≈ 0.95123.
	trade1 := newFakeTrade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), now.Add(-2*time.Minute))
	trade2 := newFakeTrade(symbol, types.SideTypeSell, Number(18000.0), Number(0.5), now.Add(-20*time.Second))
	trade3 := newFakeTrade(symbol, types.SideTypeBuy, Number(18000.0), Number(1.0), now.Add(-1*time.Second))
	sig.trades[0] = trade1
	sig.trades[1] = trade2
	sig.trades[2] = trade3
	sig.start = 0
	sig.count = 3

	// The expected score calculations (with Alpha=1.0 and Beta=1.0) are:
	// trade1 score: 1*0.00247875 + 1*0.00247875 = 0.00495750.
	// trade2 score: 0.5*0.367879 + 1*0.367879 = 0.551819.
	// trade3 score: 1*0.951229 + 1*0.951229 = 1.902458.
	// Buy score = trade1 + trade3 ≈ 1.907416, Sell score = trade2 ≈ 0.551819.
	// Signal = (1.907416 - 0.551819) / (1.907416 + 0.551819) ≈ 0.551919, scaled by 2 yields ≈ 1.10384.

	ctx := context.Background()
	sigNum, err := sig.CalculateSignal(ctx)
	assert.NoError(t, err)
	assert.InDelta(t, 1.10384, sigNum, 0.01)
	// Verify that the ring buffer internal state remains unchanged after filtering.
	assert.Equal(t, 3, sig.count)
}
