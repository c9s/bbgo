package xfundingv2

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

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
			FuturesDirection: types.PositionShort,
		},
		TradeBalanceRatio:   Number("0.5"),
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
		s.TradeBalanceRatio = Number("1.0")

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
		s.TradeBalanceRatio = Number("1.0")

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
		s.TradeBalanceRatio = Number("1.0")
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
		s.TradeBalanceRatio = Number("1.0")
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
		s.TradeBalanceRatio = Number("1.0")

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
		s.TradeBalanceRatio = Number("1.0")

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

func TestRemoveRoundsOnStartup(t *testing.T) {
	// helper to build a strategy with the given active-round symbols populated.
	newStrategyWithRounds := func(t *testing.T, ctrl *gomock.Controller, symbols ...string) *Strategy {
		s := newDefaultTestStrategy()
		s.ActiveRounds = make(map[string]*ArbitrageRound)
		s.SpotPositions = make(map[string]*types.Position)
		s.FuturesPositions = make(map[string]*types.Position)
		nextFundingTime := time.Now().Add(time.Hour)
		for _, symbol := range symbols {
			round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)
			s.ActiveRounds[symbol] = round
			s.SpotPositions[symbol] = &types.Position{}
			s.FuturesPositions[symbol] = &types.Position{}
		}
		return s
	}

	t.Run("removes configured active round once and records it", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		s := newStrategyWithRounds(t, ctrl, "BTCUSDT", "ETHUSDT")
		s.RemoveOnStartupSymbols = []string{"BTCUSDT"}

		s.removeRoundsOnStartup()

		_, btcActive := s.ActiveRounds["BTCUSDT"]
		assert.False(t, btcActive, "BTCUSDT round should be removed")
		_, ethActive := s.ActiveRounds["ETHUSDT"]
		assert.True(t, ethActive, "ETHUSDT round should be untouched")
		_, btcSpot := s.SpotPositions["BTCUSDT"]
		assert.False(t, btcSpot, "BTCUSDT spot position should be cleared")
		_, btcFutures := s.FuturesPositions["BTCUSDT"]
		assert.False(t, btcFutures, "BTCUSDT futures position should be cleared")
		assert.Contains(t, s.AppliedStartupRemovals, "BTCUSDT")
	})

	t.Run("does not delete a re-opened round on subsequent startup", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// first startup removes BTCUSDT and records it in the persisted set.
		s := newStrategyWithRounds(t, ctrl, "BTCUSDT")
		s.RemoveOnStartupSymbols = []string{"BTCUSDT"}
		s.removeRoundsOnStartup()

		// simulate a restart: the persisted set survives, and a new BTCUSDT round was
		// legitimately opened again while running (still configured for removal).
		newRound, _ := newTestArbitrageRound(t, ctrl, 8, 3, time.Now().Add(time.Hour))
		s.ActiveRounds["BTCUSDT"] = newRound
		s.removeRoundsOnStartup()

		_, btcActive := s.ActiveRounds["BTCUSDT"]
		assert.True(t, btcActive, "re-opened BTCUSDT round must be preserved on restart")
	})

	t.Run("re-adding a symbol after config change triggers removal again", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// startup 1: remove BTCUSDT.
		s := newStrategyWithRounds(t, ctrl, "BTCUSDT")
		s.RemoveOnStartupSymbols = []string{"BTCUSDT"}
		s.removeRoundsOnStartup()
		assert.Contains(t, s.AppliedStartupRemovals, "BTCUSDT")

		// startup 2: operator drops BTCUSDT from config -> it is pruned from the set,
		// so no two-step "empty config" restart is required.
		s.RemoveOnStartupSymbols = nil
		s.removeRoundsOnStartup()
		assert.NotContains(t, s.AppliedStartupRemovals, "BTCUSDT")

		// startup 3: operator re-adds BTCUSDT (a new round exists again) -> removed again.
		newRound, _ := newTestArbitrageRound(t, ctrl, 8, 3, time.Now().Add(time.Hour))
		s.ActiveRounds["BTCUSDT"] = newRound
		s.RemoveOnStartupSymbols = []string{"BTCUSDT"}
		s.removeRoundsOnStartup()
		_, btcActive := s.ActiveRounds["BTCUSDT"]
		assert.False(t, btcActive, "BTCUSDT round should be removed again after re-adding to config")
	})

	t.Run("switching to a different symbol removes only the new one", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		s := newStrategyWithRounds(t, ctrl, "BTCUSDT", "ETHUSDT")

		// startup 1: remove BTCUSDT.
		s.RemoveOnStartupSymbols = []string{"BTCUSDT"}
		s.removeRoundsOnStartup()

		// startup 2: switch config directly to ETHUSDT -> BTCUSDT pruned, ETHUSDT removed.
		s.RemoveOnStartupSymbols = []string{"ETHUSDT"}
		s.removeRoundsOnStartup()

		_, ethActive := s.ActiveRounds["ETHUSDT"]
		assert.False(t, ethActive, "ETHUSDT round should be removed after switching config")
		assert.Contains(t, s.AppliedStartupRemovals, "ETHUSDT")
		assert.NotContains(t, s.AppliedStartupRemovals, "BTCUSDT")
	})
}
