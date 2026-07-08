package xfundingv2

import (
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

func TestArbitrageRound_TradePnL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nextFundingTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)

	t.Run("returns zero profit stats when no trades", func(t *testing.T) {
		pnl := round.RealizedPnL()
		assert.Equal(t, fixedpoint.Zero, pnl.SpotProfitStats.AccumulatedPnL)
		assert.Equal(t, fixedpoint.Zero, pnl.SpotProfitStats.AccumulatedNetProfit)
		assert.Equal(t, fixedpoint.Zero, pnl.FuturesProfitStats.AccumulatedPnL)
		assert.Equal(t, fixedpoint.Zero, pnl.FuturesProfitStats.AccumulatedNetProfit)
	})

	t.Run("returns zero profit when position is only opened", func(t *testing.T) {
		// Add orders first so AddTrade accepts them
		spotExecutor := round.spotWorker.Executor()
		spotExecutor.syncState.Orders[1] = types.OrderQuery{OrderID: "1"}

		futuresExecutor := round.futuresWorker.Executor()
		futuresExecutor.syncState.Orders[2] = types.OrderQuery{OrderID: "2"}

		// Opening trades: buy spot at 40000, sell futures at 40100
		spotExecutor.AddTrade(types.Trade{
			ID:            100,
			OrderID:       1,
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeBuy,
			Price:         Number(40000),
			Quantity:      Number(1),
			QuoteQuantity: Number(40000),
			Fee:           Number(0.001),
			FeeCurrency:   "BTC",
			Time:          types.Time(time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)),
		})

		futuresExecutor.AddTrade(types.Trade{
			ID:            200,
			OrderID:       2,
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeSell,
			Price:         Number(40100),
			Quantity:      Number(1),
			QuoteQuantity: Number(40100),
			Fee:           Number(10),
			FeeCurrency:   "USDT",
			IsFutures:     true,
			Time:          types.Time(time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)),
		})

		// Only opening trades — no realized profit yet
		pnl := round.RealizedPnL()
		assert.Equal(t, fixedpoint.Zero, pnl.SpotProfitStats.AccumulatedPnL)
		assert.Equal(t, fixedpoint.Zero, pnl.FuturesProfitStats.AccumulatedPnL)
	})

	t.Run("calculates realized profit after closing trades", func(t *testing.T) {
		spotExecutor := round.spotWorker.Executor()
		spotExecutor.syncState.Orders[3] = types.OrderQuery{OrderID: "3"}

		futuresExecutor := round.futuresWorker.Executor()
		futuresExecutor.syncState.Orders[4] = types.OrderQuery{OrderID: "4"}

		// Closing trades: sell spot at 41000 (profit), buy futures at 39900 (profit)
		spotExecutor.AddTrade(types.Trade{
			ID:            101,
			OrderID:       3,
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeSell,
			Price:         Number(41000),
			Quantity:      Number(1),
			QuoteQuantity: Number(41000),
			Fee:           Number(10),
			FeeCurrency:   "USDT",
			Time:          types.Time(time.Date(2024, 1, 2, 1, 0, 0, 0, time.UTC)),
		})

		futuresExecutor.AddTrade(types.Trade{
			ID:            201,
			OrderID:       4,
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeBuy,
			Price:         Number(39900),
			Quantity:      Number(1),
			QuoteQuantity: Number(39900),
			Fee:           Number(10),
			FeeCurrency:   "USDT",
			IsFutures:     true,
			Time:          types.Time(time.Date(2024, 1, 2, 1, 0, 0, 0, time.UTC)),
		})

		pnl := round.RealizedPnL()

		// Spot: bought 1 BTC at 40000 with 0.001 BTC fee → effective qty = 0.999
		// Sold 1 BTC at 41000, fee 10 USDT
		// Profit = (41000 - 40000) * 0.999 = 999 (fee reduces effective position)
		// NetProfit = profit - close fee in quote = 999 - 10 = 989
		assert.True(t, pnl.SpotProfitStats.AccumulatedPnL.Compare(fixedpoint.Zero) > 0,
			"spot PnL should be positive, got: %s", pnl.SpotProfitStats.AccumulatedPnL)
		assert.True(t, pnl.SpotProfitStats.AccumulatedNetProfit.Compare(fixedpoint.Zero) > 0,
			"spot net profit should be positive, got: %s", pnl.SpotProfitStats.AccumulatedNetProfit)

		// Futures: sold at 40100, bought at 39900 with 10 USDT fee on each side
		// Profit = (40100 - 39900) * 1 = 200
		// NetProfit = 200 - 10 (close fee) = 190
		// Note: open fee (10 USDT) is accounted in position average cost
		assert.True(t, pnl.FuturesProfitStats.AccumulatedPnL.Compare(fixedpoint.Zero) > 0,
			"futures PnL should be positive, got: %s", pnl.FuturesProfitStats.AccumulatedPnL)
		assert.True(t, pnl.FuturesProfitStats.AccumulatedNetProfit.Compare(fixedpoint.Zero) > 0,
			"futures net profit should be positive, got: %s", pnl.FuturesProfitStats.AccumulatedNetProfit)

		// Combined PnL should be substantial (both legs profitable)
		combinedPnL := pnl.SpotProfitStats.AccumulatedPnL.Add(pnl.FuturesProfitStats.AccumulatedPnL)
		assert.True(t, combinedPnL.Compare(Number(1000)) > 0,
			"combined PnL should exceed 1000, got: %s", combinedPnL)
	})
}

// newTestOrderBook builds an order book with the given bid/ask price-volume text.
// Either side may be empty to simulate a one-sided book.
func newTestOrderBook(symbol, bidsText, asksText string) types.OrderBook {
	book := &types.SliceOrderBook{Symbol: symbol}
	if bidsText != "" {
		book.Bids = PriceVolumeSliceFromText(bidsText)
	}
	if asksText != "" {
		book.Asks = PriceVolumeSliceFromText(asksText)
	}
	return book
}

func TestArbitrageRound_UnrealizedPnL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nextFundingTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)

	t.Run("returns zero unrealized PnL when there are no trades", func(t *testing.T) {
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)

		spotBook := newTestOrderBook("BTCUSDT", "41000, 10", "41010, 10")
		futuresBook := newTestOrderBook("BTCUSDT", "40990, 10", "41000, 10")

		pnl := round.UnrealizedPnL(spotBook, futuresBook)
		assert.Equal(t, fixedpoint.Zero, pnl.UnrealizedSpotPnL)
		assert.Equal(t, fixedpoint.Zero, pnl.UnrealizedFuturesPnL)
		// with no realized profit and no unrealized profit, total is zero
		assert.Equal(t, pnl.NetPnL(), pnl.TotalPnL())
	})

	t.Run("marks open long spot and short futures to market", func(t *testing.T) {
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)

		// Opening trades: buy spot at 40000 (long), sell futures at 40100 (short)
		spotExecutor := round.spotWorker.Executor()
		spotExecutor.syncState.Orders[1] = types.OrderQuery{OrderID: "1"}
		spotExecutor.AddTrade(types.Trade{
			ID:            100,
			OrderID:       1,
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeBuy,
			Price:         Number(40000),
			Quantity:      Number(1),
			QuoteQuantity: Number(40000),
			Time:          types.Time(time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)),
		})

		futuresExecutor := round.futuresWorker.Executor()
		futuresExecutor.syncState.Orders[2] = types.OrderQuery{OrderID: "2"}
		futuresExecutor.AddTrade(types.Trade{
			ID:            200,
			OrderID:       2,
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeSell,
			Price:         Number(40100),
			Quantity:      Number(1),
			QuoteQuantity: Number(40100),
			IsFutures:     true,
			Time:          types.Time(time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)),
		})

		// Spot long closes at the bid (41000 > 40000 avg cost → profit).
		// Futures short closes at the ask (40000 < 40100 avg cost → profit).
		spotBook := newTestOrderBook("BTCUSDT", "41000, 10", "41010, 10")
		futuresBook := newTestOrderBook("BTCUSDT", "39990, 10", "40000, 10")

		pnl := round.UnrealizedPnL(spotBook, futuresBook)

		// realized positions are populated on the embedded struct
		assert.True(t, pnl.SpotPosition.IsLong(), "spot position should be long")
		assert.True(t, pnl.FuturesPosition.IsShort(), "futures position should be short")

		// spot: (41000 - 40000) * 1 = 1000
		assert.Equal(t, Number(1000), pnl.UnrealizedSpotPnL,
			"unrealized spot PnL should be 1000, got: %s", pnl.UnrealizedSpotPnL)
		// futures: (40000 - 40100) * -1 = 100
		assert.Equal(t, Number(100), pnl.UnrealizedFuturesPnL,
			"unrealized futures PnL should be 100, got: %s", pnl.UnrealizedFuturesPnL)

		// total = realized net + both unrealized legs
		expectedTotal := pnl.NetPnL().Add(Number(1000)).Add(Number(100))
		assert.Equal(t, expectedTotal, pnl.TotalPnL())
	})

	t.Run("returns zero for a leg when its close-side book is empty", func(t *testing.T) {
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)

		// long spot position → needs a bid to mark to market
		spotExecutor := round.spotWorker.Executor()
		spotExecutor.syncState.Orders[1] = types.OrderQuery{OrderID: "1"}
		spotExecutor.AddTrade(types.Trade{
			ID:            100,
			OrderID:       1,
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeBuy,
			Price:         Number(40000),
			Quantity:      Number(1),
			QuoteQuantity: Number(40000),
			Time:          types.Time(time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)),
		})

		// spot book has asks only (no bid) → long leg cannot be priced → zero
		spotBook := newTestOrderBook("BTCUSDT", "", "41010, 10")
		futuresBook := newTestOrderBook("BTCUSDT", "39990, 10", "40000, 10")

		pnl := round.UnrealizedPnL(spotBook, futuresBook)
		assert.Equal(t, fixedpoint.Zero, pnl.UnrealizedSpotPnL,
			"unrealized spot PnL should be zero when the bid side is empty")
	})
}
