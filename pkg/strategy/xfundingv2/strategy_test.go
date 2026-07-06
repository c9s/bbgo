package xfundingv2

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

// newTestStreamOrderBook creates a StreamOrderBook with loaded order book data for testing.
// Both bids and asks must be provided for the cost estimator to calculate entry and exit costs.
func newTestStreamOrderBook(symbol string, bids, asks []types.PriceVolume) *types.StreamOrderBook {
	sob := types.NewStreamBook(symbol, types.ExchangeName("test"))
	book := types.SliceOrderBook{
		Symbol: symbol,
		Bids:   bids,
		Asks:   asks,
		Time:   time.Now(),
	}
	sob.Load(book)
	return sob
}

func newTestSession(markets types.MarketMap, balances types.BalanceMap) *bbgo.ExchangeSession {
	session := &bbgo.ExchangeSession{
		Account: types.NewAccount(),
	}
	session.Account.UpdateBalances(balances)
	session.SetMarkets(markets)
	return session
}

func newDefaultTestStrategy() *Strategy {
	return &Strategy{
		MarketSelectionConfig: &MarketSelectionConfig{
			FuturesDirection:  types.PositionShort,
			TradeBalanceRatio: Number("0.5"),
		},
		MaxPositionExposure: make(map[string]fixedpoint.Value),
		costEstimator:       NewCostEstimator(),
		logger:              logrus.StandardLogger(),
	}
}

func TestSelectMostProfitableMarket(t *testing.T) {
	btcMarket := Market("BTCUSDT")
	ethMarket := Market("ETHUSDT")

	t.Run("empty candidates returns nil", func(t *testing.T) {
		s := newDefaultTestStrategy()
		result := s.selectMostProfitableMarket(nil)
		assert.Nil(t, result)

		result = s.selectMostProfitableMarket([]MarketCandidate{})
		assert.Nil(t, result)
	})

	t.Run("short futures selects candidate with shortest breakeven interval", func(t *testing.T) {
		s := newDefaultTestStrategy()
		s.MarketSelectionConfig.FuturesDirection = types.PositionShort
		s.MarketSelectionConfig.TradeBalanceRatio = Number("1.0")

		markets := types.MarketMap{
			"BTCUSDT": btcMarket,
			"ETHUSDT": ethMarket,
		}
		balances := types.BalanceMap{
			"USDT": Balance("USDT", Number(10000)),
		}
		s.spotSession = newTestSession(markets, balances)
		s.futuresSession = newTestSession(markets, balances)

		// Both bids and asks are needed: asks for entry (buy spot), bids for exit (sell spot)
		s.spotOrderBooks = map[string]*types.StreamOrderBook{
			"BTCUSDT": newTestStreamOrderBook("BTCUSDT",
				[]types.PriceVolume{{Price: Number(49900), Volume: Number(10)}},
				[]types.PriceVolume{{Price: Number(50000), Volume: Number(10)}}),
			"ETHUSDT": newTestStreamOrderBook("ETHUSDT",
				[]types.PriceVolume{{Price: Number(1990), Volume: Number(100)}},
				[]types.PriceVolume{{Price: Number(2000), Volume: Number(100)}}),
		}

		// Both bids and asks are needed: bids for entry (short futures), asks for exit (close short)
		s.futuresOrderBooks = map[string]*types.StreamOrderBook{
			"BTCUSDT": newTestStreamOrderBook("BTCUSDT",
				[]types.PriceVolume{{Price: Number(50100), Volume: Number(10)}},
				[]types.PriceVolume{{Price: Number(50200), Volume: Number(10)}}),
			"ETHUSDT": newTestStreamOrderBook("ETHUSDT",
				[]types.PriceVolume{{Price: Number(2010), Volume: Number(100)}},
				[]types.PriceVolume{{Price: Number(2020), Volume: Number(100)}}),
		}

		candidates := []MarketCandidate{
			{
				Symbol: "BTCUSDT",
				PremiumIndex: &types.PremiumIndex{
					Symbol:          "BTCUSDT",
					LastFundingRate: Number("0.001"), // 0.1% per interval
				},
				FundingIntervalHours: 8,
			},
			{
				Symbol: "ETHUSDT",
				PremiumIndex: &types.PremiumIndex{
					Symbol:          "ETHUSDT",
					LastFundingRate: Number("0.005"), // 0.5% per interval -> higher rate, shorter breakeven
				},
				FundingIntervalHours: 8,
			},
		}

		result := s.selectMostProfitableMarket(candidates)
		assert.NotNil(t, result)
		// ETH has a higher funding rate so it should break even faster
		assert.Equal(t, "ETHUSDT", result.Symbol)
		assert.True(t, result.TargetFuturesPosition.Sign() < 0, "target futures position should be negative for short")
		assert.Greater(t, result.MinHoldingIntervals, 0)
		assert.True(t, result.MinHoldingDuration > 0)
	})

	t.Run("long futures selects candidate with shortest breakeven interval", func(t *testing.T) {
		s := newDefaultTestStrategy()
		s.MarketSelectionConfig.FuturesDirection = types.PositionLong
		s.MarketSelectionConfig.TradeBalanceRatio = Number("1.0")

		markets := types.MarketMap{
			"BTCUSDT": btcMarket,
			"ETHUSDT": ethMarket,
		}
		balances := types.BalanceMap{
			"BTC": Balance("BTC", Number("0.1")),
			"ETH": Balance("ETH", Number(5)),
		}
		s.spotSession = newTestSession(markets, balances)
		s.futuresSession = newTestSession(markets, balances)

		s.spotOrderBooks = map[string]*types.StreamOrderBook{
			"BTCUSDT": newTestStreamOrderBook("BTCUSDT",
				[]types.PriceVolume{{Price: Number(49900), Volume: Number(10)}},
				[]types.PriceVolume{{Price: Number(50000), Volume: Number(10)}}),
			"ETHUSDT": newTestStreamOrderBook("ETHUSDT",
				[]types.PriceVolume{{Price: Number(1990), Volume: Number(100)}},
				[]types.PriceVolume{{Price: Number(2000), Volume: Number(100)}}),
		}

		s.futuresOrderBooks = map[string]*types.StreamOrderBook{
			"BTCUSDT": newTestStreamOrderBook("BTCUSDT",
				[]types.PriceVolume{{Price: Number(49800), Volume: Number(10)}},
				[]types.PriceVolume{{Price: Number(49900), Volume: Number(10)}}),
			"ETHUSDT": newTestStreamOrderBook("ETHUSDT",
				[]types.PriceVolume{{Price: Number(1980), Volume: Number(100)}},
				[]types.PriceVolume{{Price: Number(1990), Volume: Number(100)}}),
		}

		candidates := []MarketCandidate{
			{
				Symbol: "BTCUSDT",
				PremiumIndex: &types.PremiumIndex{
					Symbol:          "BTCUSDT",
					LastFundingRate: Number("-0.001"),
				},
				FundingIntervalHours: 8,
			},
			{
				Symbol: "ETHUSDT",
				PremiumIndex: &types.PremiumIndex{
					Symbol:          "ETHUSDT",
					LastFundingRate: Number("-0.005"),
				},
				FundingIntervalHours: 8,
			},
		}

		result := s.selectMostProfitableMarket(candidates)
		assert.NotNil(t, result)
		assert.True(t, result.TargetFuturesPosition.Sign() > 0, "target futures position should be positive for long")
		assert.Greater(t, result.MinHoldingIntervals, 0)
	})

	t.Run("no quote balance returns nil for short futures", func(t *testing.T) {
		s := newDefaultTestStrategy()
		s.MarketSelectionConfig.FuturesDirection = types.PositionShort

		markets := types.MarketMap{"BTCUSDT": btcMarket}
		balances := types.BalanceMap{}
		s.spotSession = newTestSession(markets, balances)
		s.futuresSession = newTestSession(markets, balances)

		s.spotOrderBooks = map[string]*types.StreamOrderBook{
			"BTCUSDT": newTestStreamOrderBook("BTCUSDT",
				[]types.PriceVolume{{Price: Number(49900), Volume: Number(10)}},
				[]types.PriceVolume{{Price: Number(50000), Volume: Number(10)}}),
		}
		s.futuresOrderBooks = map[string]*types.StreamOrderBook{
			"BTCUSDT": newTestStreamOrderBook("BTCUSDT",
				[]types.PriceVolume{{Price: Number(50100), Volume: Number(10)}},
				[]types.PriceVolume{{Price: Number(50200), Volume: Number(10)}}),
		}

		candidates := []MarketCandidate{
			{
				Symbol: "BTCUSDT",
				PremiumIndex: &types.PremiumIndex{
					Symbol:          "BTCUSDT",
					LastFundingRate: Number("0.001"),
				},
				FundingIntervalHours: 8,
			},
		}

		result := s.selectMostProfitableMarket(candidates)
		assert.Nil(t, result)
	})

	t.Run("unknown market symbol is skipped", func(t *testing.T) {
		s := newDefaultTestStrategy()
		s.MarketSelectionConfig.FuturesDirection = types.PositionShort

		markets := types.MarketMap{}
		balances := types.BalanceMap{
			"USDT": Balance("USDT", Number(10000)),
		}
		s.spotSession = newTestSession(markets, balances)
		s.futuresSession = newTestSession(markets, balances)
		s.spotOrderBooks = map[string]*types.StreamOrderBook{}
		s.futuresOrderBooks = map[string]*types.StreamOrderBook{}

		candidates := []MarketCandidate{
			{
				Symbol: "BTCUSDT",
				PremiumIndex: &types.PremiumIndex{
					Symbol:          "BTCUSDT",
					LastFundingRate: Number("0.001"),
				},
				FundingIntervalHours: 8,
			},
		}

		result := s.selectMostProfitableMarket(candidates)
		assert.Nil(t, result)
	})

	t.Run("invalid futures direction returns nil", func(t *testing.T) {
		s := newDefaultTestStrategy()
		s.MarketSelectionConfig.FuturesDirection = types.PositionType("Invalid")

		markets := types.MarketMap{"BTCUSDT": btcMarket}
		balances := types.BalanceMap{
			"USDT": Balance("USDT", Number(10000)),
		}
		s.spotSession = newTestSession(markets, balances)
		s.futuresSession = newTestSession(markets, balances)
		s.spotOrderBooks = map[string]*types.StreamOrderBook{
			"BTCUSDT": newTestStreamOrderBook("BTCUSDT",
				[]types.PriceVolume{{Price: Number(49900), Volume: Number(10)}},
				[]types.PriceVolume{{Price: Number(50000), Volume: Number(10)}}),
		}
		s.futuresOrderBooks = map[string]*types.StreamOrderBook{
			"BTCUSDT": newTestStreamOrderBook("BTCUSDT",
				[]types.PriceVolume{{Price: Number(50100), Volume: Number(10)}},
				[]types.PriceVolume{{Price: Number(50200), Volume: Number(10)}}),
		}

		candidates := []MarketCandidate{
			{
				Symbol: "BTCUSDT",
				PremiumIndex: &types.PremiumIndex{
					Symbol:          "BTCUSDT",
					LastFundingRate: Number("0.001"),
				},
				FundingIntervalHours: 8,
			},
		}

		result := s.selectMostProfitableMarket(candidates)
		assert.Nil(t, result)
	})

	t.Run("max position exposure caps short futures position", func(t *testing.T) {
		s := newDefaultTestStrategy()
		s.MarketSelectionConfig.FuturesDirection = types.PositionShort
		s.MarketSelectionConfig.TradeBalanceRatio = Number("1.0")
		s.MaxPositionExposure = map[string]fixedpoint.Value{
			"ETH": Number("0.5"),
		}

		markets := types.MarketMap{"ETHUSDT": ethMarket}
		balances := types.BalanceMap{
			"USDT": Balance("USDT", Number(10000)),
		}
		s.spotSession = newTestSession(markets, balances)
		s.futuresSession = newTestSession(markets, balances)

		s.spotOrderBooks = map[string]*types.StreamOrderBook{
			"ETHUSDT": newTestStreamOrderBook("ETHUSDT",
				[]types.PriceVolume{{Price: Number(1990), Volume: Number(100)}},
				[]types.PriceVolume{{Price: Number(2000), Volume: Number(100)}}),
		}
		s.futuresOrderBooks = map[string]*types.StreamOrderBook{
			"ETHUSDT": newTestStreamOrderBook("ETHUSDT",
				[]types.PriceVolume{{Price: Number(2010), Volume: Number(100)}},
				[]types.PriceVolume{{Price: Number(2020), Volume: Number(100)}}),
		}

		candidates := []MarketCandidate{
			{
				Symbol: "ETHUSDT",
				PremiumIndex: &types.PremiumIndex{
					Symbol:          "ETHUSDT",
					LastFundingRate: Number("0.005"),
				},
				FundingIntervalHours: 8,
			},
		}

		result := s.selectMostProfitableMarket(candidates)
		assert.NotNil(t, result)
		assert.Equal(t, Number("0.5"), result.TargetFuturesPosition.Abs())
		assert.True(t, result.TargetFuturesPosition.Sign() < 0)
	})

	t.Run("max position exposure caps long futures position", func(t *testing.T) {
		s := newDefaultTestStrategy()
		s.MarketSelectionConfig.FuturesDirection = types.PositionLong
		s.MarketSelectionConfig.TradeBalanceRatio = Number("1.0")
		s.MaxPositionExposure = map[string]fixedpoint.Value{
			"ETH": Number("0.5"),
		}

		markets := types.MarketMap{"ETHUSDT": ethMarket}
		balances := types.BalanceMap{
			"ETH": Balance("ETH", Number(5)),
		}
		s.spotSession = newTestSession(markets, balances)
		s.futuresSession = newTestSession(markets, balances)

		s.spotOrderBooks = map[string]*types.StreamOrderBook{
			"ETHUSDT": newTestStreamOrderBook("ETHUSDT",
				[]types.PriceVolume{{Price: Number(1990), Volume: Number(100)}},
				[]types.PriceVolume{{Price: Number(2000), Volume: Number(100)}}),
		}
		s.futuresOrderBooks = map[string]*types.StreamOrderBook{
			"ETHUSDT": newTestStreamOrderBook("ETHUSDT",
				[]types.PriceVolume{{Price: Number(1980), Volume: Number(100)}},
				[]types.PriceVolume{{Price: Number(1990), Volume: Number(100)}}),
		}

		candidates := []MarketCandidate{
			{
				Symbol: "ETHUSDT",
				PremiumIndex: &types.PremiumIndex{
					Symbol:          "ETHUSDT",
					LastFundingRate: Number("-0.005"),
				},
				FundingIntervalHours: 8,
			},
		}

		result := s.selectMostProfitableMarket(candidates)
		assert.NotNil(t, result)
		assert.Equal(t, Number("0.5"), result.TargetFuturesPosition.Abs())
		assert.True(t, result.TargetFuturesPosition.Sign() > 0)
	})

	t.Run("zero funding rate candidate is skipped", func(t *testing.T) {
		s := newDefaultTestStrategy()
		s.MarketSelectionConfig.FuturesDirection = types.PositionShort
		s.MarketSelectionConfig.TradeBalanceRatio = Number("1.0")

		markets := types.MarketMap{"BTCUSDT": btcMarket}
		balances := types.BalanceMap{
			"USDT": Balance("USDT", Number(10000)),
		}
		s.spotSession = newTestSession(markets, balances)
		s.futuresSession = newTestSession(markets, balances)

		s.spotOrderBooks = map[string]*types.StreamOrderBook{
			"BTCUSDT": newTestStreamOrderBook("BTCUSDT",
				[]types.PriceVolume{{Price: Number(49900), Volume: Number(10)}},
				[]types.PriceVolume{{Price: Number(50000), Volume: Number(10)}}),
		}
		s.futuresOrderBooks = map[string]*types.StreamOrderBook{
			"BTCUSDT": newTestStreamOrderBook("BTCUSDT",
				[]types.PriceVolume{{Price: Number(50100), Volume: Number(10)}},
				[]types.PriceVolume{{Price: Number(50200), Volume: Number(10)}}),
		}

		candidates := []MarketCandidate{
			{
				Symbol: "BTCUSDT",
				PremiumIndex: &types.PremiumIndex{
					Symbol:          "BTCUSDT",
					LastFundingRate: Number(0),
				},
				FundingIntervalHours: 8,
			},
		}

		result := s.selectMostProfitableMarket(candidates)
		assert.Nil(t, result, "zero funding rate should cause candidate to be skipped")
	})

	t.Run("single candidate returns that candidate", func(t *testing.T) {
		s := newDefaultTestStrategy()
		s.MarketSelectionConfig.FuturesDirection = types.PositionShort
		s.MarketSelectionConfig.TradeBalanceRatio = Number("1.0")

		markets := types.MarketMap{"ETHUSDT": ethMarket}
		balances := types.BalanceMap{
			"USDT": Balance("USDT", Number(10000)),
		}
		s.spotSession = newTestSession(markets, balances)
		s.futuresSession = newTestSession(markets, balances)

		s.spotOrderBooks = map[string]*types.StreamOrderBook{
			"ETHUSDT": newTestStreamOrderBook("ETHUSDT",
				[]types.PriceVolume{{Price: Number(1990), Volume: Number(100)}},
				[]types.PriceVolume{{Price: Number(2000), Volume: Number(100)}}),
		}
		s.futuresOrderBooks = map[string]*types.StreamOrderBook{
			"ETHUSDT": newTestStreamOrderBook("ETHUSDT",
				[]types.PriceVolume{{Price: Number(2010), Volume: Number(100)}},
				[]types.PriceVolume{{Price: Number(2020), Volume: Number(100)}}),
		}

		candidates := []MarketCandidate{
			{
				Symbol: "ETHUSDT",
				PremiumIndex: &types.PremiumIndex{
					Symbol:          "ETHUSDT",
					LastFundingRate: Number("0.005"),
				},
				FundingIntervalHours: 8,
			},
		}

		result := s.selectMostProfitableMarket(candidates)
		assert.NotNil(t, result)
		assert.Equal(t, "ETHUSDT", result.Symbol)
		assert.Equal(t, 8, result.FundingIntervalHours)
	})
}
