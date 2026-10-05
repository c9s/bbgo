package xfundingv2

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

func bnbMarket() types.Market {
	return types.Market{
		Symbol:          "BNBUSDT",
		BaseCurrency:    "BNB",
		QuoteCurrency:   "USDT",
		TickSize:        Number(0.01),
		StepSize:        Number(0.001),
		PricePrecision:  2,
		VolumePrecision: 3,
	}
}

// newFeeSymbolStrategy builds a strategy whose fee symbol is BNBUSDT, with the given BNB balances per wallet.
func newFeeSymbolStrategy(
	ctrl *gomock.Controller, spotBNB, futuresBNB fixedpoint.Value,
) (*Strategy, *mocks.MockExchange, *mockFuturesServiceForFee) {
	mockExchange := mocks.NewMockExchange(ctrl)
	mockExchange.EXPECT().Name().Return(types.ExchangeBinance).AnyTimes()

	spotAccount := types.NewAccount()
	spotAccount.SetBalance("BNB", types.Balance{Currency: "BNB", Available: spotBNB})
	futuresAccount := types.NewAccount()
	futuresAccount.SetBalance("BNB", types.Balance{Currency: "BNB", Available: futuresBNB})

	spotSession := &bbgo.ExchangeSession{Account: spotAccount, Exchange: mockExchange}
	spotSession.SetMarkets(types.MarketMap{"BNBUSDT": bnbMarket()})
	futuresSession := &bbgo.ExchangeSession{Account: futuresAccount}

	service := &mockFuturesServiceForFee{}
	position := types.NewPositionFromMarket(bnbMarket())
	feeExecutor := bbgo.NewGeneralOrderExecutor(spotSession, "BNBUSDT", "xfundingv2", "test", position)

	s := &Strategy{
		FeeSymbol:                 "BNBUSDT",
		spotSession:               spotSession,
		futuresSession:            futuresSession,
		futuresService:            service,
		spotGeneralOrderExecutors: map[string]*bbgo.GeneralOrderExecutor{"BNBUSDT": feeExecutor},
		spotOrderBooks: map[string]*types.StreamOrderBook{
			"BNBUSDT": newStreamOrderBookWithData("BNBUSDT",
				types.PriceVolumeSlice{{Price: Number(600), Volume: Number(100)}},
				types.PriceVolumeSlice{{Price: Number(601), Volume: Number(100)}},
			),
		},
		ActiveRounds:  map[string]*ArbitrageRound{},
		PendingRounds: map[string]*PendingRound{},
		logger:        logrus.StandardLogger(),
	}
	return s, mockExchange, service
}

// newBNBHedgeRound returns an active short-direction round on BNBUSDT that parks hedgeOnFutures BNB on futures as collateral.
func newBNBHedgeRound(hedgeOnFutures fixedpoint.Value) *ArbitrageRound {
	policy, _ := newDirectionPolicy(types.PositionShort, bnbMarket())
	round := &ArbitrageRound{}
	round.syncState.State = RoundReady
	round.syncState.DirectionPolicy = policy
	round.syncState.TransferInAmount = hedgeOnFutures
	return round
}

func TestStrategy_FeeSymbolCandidate_AcquireFeeAsset(t *testing.T) {
	origBacktest := bbgo.IsBackTesting
	bbgo.IsBackTesting = true // skip the 5s wait for the market order
	defer func() { bbgo.IsBackTesting = origBacktest }()

	pendingRound := func() *ArbitrageRound {
		round := &ArbitrageRound{}
		round.SetSpotFeeAssetAmount(Number(0.5))
		round.SetFuturesFeeAssetAmount(Number(0.3))
		return round
	}

	t.Run("hedge collateral on futures is not counted as fee reserve", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// futures holds 5 BNB, all of it hedge; the pending round needs 0.3 BNB of futures reserve.
		s, mockExchange, service := newFeeSymbolStrategy(ctrl, Number(1.0), Number(5.0))
		s.ActiveRounds["BNBUSDT"] = newBNBHedgeRound(Number(5.0))
		mockExchange.EXPECT().SubmitOrder(gomock.Any(), gomock.Any()).Times(0)

		err := s.acquireFeeAssetAndTransfer(context.Background(), []*ArbitrageRound{pendingRound()})
		assert.NoError(t, err)

		// spot has 0.5 spare, so 0.3 moves to futures without buying
		assert.True(t, service.transferCalled)
		assert.Equal(t, "BNB", service.transferredAsset)
		assert.Equal(t, Number(0.3), service.transferredAmount)
		assert.Equal(t, types.TransferIn, service.transferredDirection)
	})

	t.Run("spot shortage never pulls the hedge back from futures", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// spot reserve 0.2 BNB, futures holds only hedge
		s, mockExchange, service := newFeeSymbolStrategy(ctrl, Number(0.2), Number(5.0))
		s.ActiveRounds["BNBUSDT"] = newBNBHedgeRound(Number(5.0))

		round := &ArbitrageRound{}
		round.SetSpotFeeAssetAmount(Number(0.5))
		round.SetFuturesFeeAssetAmount(fixedpoint.Zero)

		mockExchange.EXPECT().
			SubmitOrder(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
				assert.Equal(t, types.SideTypeBuy, order.Side)
				assert.Equal(t, Number(0.3), order.Quantity)
				return &types.Order{}, nil
			})

		err := s.acquireFeeAssetAndTransfer(context.Background(), []*ArbitrageRound{round})
		assert.NoError(t, err)
		assert.False(t, service.transferCalled, "the hedge collateral must stay on futures")
	})

	t.Run("control: without a hedge round the whole futures balance counts as reserve", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		s, mockExchange, service := newFeeSymbolStrategy(ctrl, Number(1.0), Number(5.0))
		mockExchange.EXPECT().SubmitOrder(gomock.Any(), gomock.Any()).Times(0)

		err := s.acquireFeeAssetAndTransfer(context.Background(), []*ArbitrageRound{pendingRound()})
		assert.NoError(t, err)
		assert.False(t, service.transferCalled)
	})

	t.Run("active rounds' fee requirement is counted against the shared balance", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// the active round still needs 0.5 spot BNB for fees; spot only holds 0.6
		s, mockExchange, service := newFeeSymbolStrategy(ctrl, Number(0.6), Number(1.0))
		active := &ArbitrageRound{}
		active.syncState.State = RoundPending
		active.SetSpotFeeAssetAmount(Number(0.5))
		active.SetFuturesFeeAssetAmount(fixedpoint.Zero)
		s.ActiveRounds["ETHUSDT"] = active

		// spot needs 0.5 + 0.5 = 1.0 against 0.6, futures has 1.0 spare: pull 0.4 back without buying
		mockExchange.EXPECT().SubmitOrder(gomock.Any(), gomock.Any()).Times(0)

		newRound := &ArbitrageRound{}
		newRound.SetSpotFeeAssetAmount(Number(0.5))
		newRound.SetFuturesFeeAssetAmount(fixedpoint.Zero)

		err := s.acquireFeeAssetAndTransfer(context.Background(), []*ArbitrageRound{newRound})
		assert.NoError(t, err)
		assert.True(t, service.transferCalled)
		assert.Equal(t, Number(0.4), service.transferredAmount)
		assert.Equal(t, types.TransferOut, service.transferredDirection)
	})
}

func TestStrategy_FeeReserveHelpers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s, _, _ := newFeeSymbolStrategy(ctrl, Number(1), Number(1))
	pending := &ArbitrageRound{}
	pending.SetSpotFeeAssetAmount(Number(0.5))
	pending.SetFuturesFeeAssetAmount(Number(0.3))
	s.PendingRounds["ETHUSDT"] = &PendingRound{Round: pending}

	t.Run("isFeeCurrency", func(t *testing.T) {
		assert.True(t, s.isFeeCurrency("BNB"))
		assert.False(t, s.isFeeCurrency("ETH"))

		noFee := &Strategy{spotSession: s.spotSession}
		assert.False(t, noFee.isFeeCurrency("BNB"))
	})

	t.Run("feeReserveOnSpot only applies to the fee currency", func(t *testing.T) {
		assert.Equal(t, Number(0.5), s.feeReserveOnSpot("BNB"))
		assert.Equal(t, fixedpoint.Zero, s.feeReserveOnSpot("ETH"))
	})

	t.Run("reservedSpotBase follows the snapshot", func(t *testing.T) {
		assert.Equal(t, fixedpoint.Zero, s.reservedSpotBase("BNB"), "no snapshot taken yet")

		s.refreshFeeReserveSnapshot()
		assert.Equal(t, Number(0.5), s.reservedSpotBase("BNB"))
		assert.Equal(t, fixedpoint.Zero, s.reservedSpotBase("ETH"))
	})

	t.Run("reservedSpotBase is safe while a round lock is held", func(t *testing.T) {
		s.refreshFeeReserveSnapshot()
		pending.mu.Lock()
		defer pending.mu.Unlock()
		assert.Equal(t, Number(0.5), s.reservedSpotBase("BNB"))
	})
}

func TestArbitrageRound_HeldFeeAssetAmounts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nextFundingTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)

	t.Run("short direction holds the collateral on futures", func(t *testing.T) {
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)
		round.syncState.TransferInAmount = Number(5)
		round.syncState.TransferOutAmount = Number(1.5)

		spot, futures := round.HeldFeeAssetAmounts("BTC")
		assert.Equal(t, fixedpoint.Zero, spot)
		assert.Equal(t, Number(3.5), futures)
	})

	t.Run("floors at zero when more was transferred out than in", func(t *testing.T) {
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)
		round.syncState.TransferInAmount = Number(1)
		round.syncState.TransferOutAmount = Number(2)

		_, futures := round.HeldFeeAssetAmounts("BTC")
		assert.Equal(t, fixedpoint.Zero, futures)
	})

	t.Run("other fee currency holds nothing", func(t *testing.T) {
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)
		round.syncState.TransferInAmount = Number(5)

		spot, futures := round.HeldFeeAssetAmounts("BNB")
		assert.Equal(t, fixedpoint.Zero, spot)
		assert.Equal(t, fixedpoint.Zero, futures)

		spot, futures = round.HeldFeeAssetAmounts("")
		assert.Equal(t, fixedpoint.Zero, spot)
		assert.Equal(t, fixedpoint.Zero, futures)
	})

	t.Run("long direction holds the unsold base on spot while opening", func(t *testing.T) {
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)
		policy, _ := newDirectionPolicy(types.PositionLong, round.spotWorker.Market())
		round.syncState.DirectionPolicy = policy
		round.syncState.State = RoundOpening
		round.spotWorker.SetTargetPosition(Number(-10))

		spot, futures := round.HeldFeeAssetAmounts("BTC")
		assert.Equal(t, Number(10), spot)
		assert.Equal(t, fixedpoint.Zero, futures)

		round.syncState.State = RoundReady
		spot, _ = round.HeldFeeAssetAmounts("BTC")
		assert.Equal(t, fixedpoint.Zero, spot)
	})
}

func TestArbitrageRound_FeeCurrency(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nextFundingTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)

	assert.Equal(t, "", round.feeCurrency())

	t.Run("falls back to the fee symbol minus the quote currency", func(t *testing.T) {
		round.syncState.FeeSymbol = "BNBUSDT"
		assert.Equal(t, "BNB", round.feeCurrency())
	})

	t.Run("prefers the persisted fee currency", func(t *testing.T) {
		round.SetAvgFeeCost("BNBUSDT", "BNB", Number(600))
		assert.Equal(t, "BNB", round.feeCurrency())
		assert.True(t, round.reserveCoversFee("BNB"))
		assert.False(t, round.reserveCoversFee("BTC"))
	})

	t.Run("pnl fee average costs are keyed by the fee currency", func(t *testing.T) {
		round.SetAvgFeeCost("BNBUSDT", "BNB", Number(600))
		pnl := round.RealizedPnL()
		assert.Equal(t, Number(600), pnl.SpotPosition.FeeAverageCosts["BNB"])
		assert.Equal(t, Number(600), pnl.FuturesPosition.FeeAverageCosts["BNB"])
		_, wrongKey := pnl.SpotPosition.FeeAverageCosts["BTC"]
		assert.False(t, wrongKey, "must not be keyed by the traded base currency")
	})
}

func TestArbitrageRound_SpotTradeTransferWithFeeReserve(t *testing.T) {
	cases := []struct {
		name        string
		feeCurrency string // fee reserve currency recorded on the round
		tradeFeeCcy string
		expected    fixedpoint.Value
	}{
		{"fee paid in the collateral asset and no reserve", "", "BTC", Number(0.999)},
		{"fee paid in the collateral asset but another reserve exists", "BNB", "BTC", Number(0.999)},
		{"collateral asset is the fee reserve currency", "BTC", "BTC", Number(1)},
		{"fee paid in the reserve currency, collateral differs", "BNB", "BNB", Number(1)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			nextFundingTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
			round, service := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)
			round.syncState.State = RoundOpening
			if c.feeCurrency != "" {
				round.SetAvgFeeCost(c.feeCurrency+"USDT", c.feeCurrency, Number(1))
			}

			account := types.NewAccount()
			account.UpdateBalances(types.BalanceMap{"BTC": Balance("BTC", Number(100))})

			trade := arbSpotTrade(1, 1, types.SideTypeBuy, Number(40000), Number(1))
			trade.Fee = Number(0.001)
			trade.FeeCurrency = c.tradeFeeCcy

			round.handleSpotTradeForOpen(trade, account, time.Now())

			transfers := service.Transfers()
			if assert.Len(t, transfers, 1) {
				assert.Equal(t, "BTC", transfers[0].Asset)
				assert.Equal(t, c.expected, transfers[0].Amount)
			}
		})
	}
}

func TestTWAPWorker_SpotSellKeepsFeeReserve(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := TWAPWorkerConfig{
		Duration:  types.Duration(10 * time.Minute),
		NumSlices: 5,
	}
	market := Market("BTCUSDT")
	startTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

	newWorker := func() *TWAPWorker {
		worker, _, _, _ := newTestTWAPWorker(t, ctrl, config)
		worker.ResetTime(startTime, config.Duration)
		worker.getAccount().SetBalance("BTC", types.Balance{Currency: "BTC", Available: Number(0.5)})
		return worker
	}

	t.Run("sell is capped at the balance minus the reserve", func(t *testing.T) {
		worker := newWorker()
		worker.SetReservedBaseFn(func(asset string) fixedpoint.Value {
			if asset == "BTC" {
				return Number(0.4)
			}
			return fixedpoint.Zero
		})
		// 2.0 / 5 slices = 0.4, but only 0.5 - 0.4 = 0.1 is free to sell
		assert.Equal(t, Number(0.1), worker.calculateSliceQuantity(startTime, Number(-2.0), false, market, fixedpoint.Zero))
	})

	t.Run("sell is capped at the balance without a reserve", func(t *testing.T) {
		worker := newWorker()
		assert.Equal(t, Number(0.4), worker.calculateSliceQuantity(startTime, Number(-2.0), false, market, fixedpoint.Zero))

		worker.getAccount().SetBalance("BTC", types.Balance{Currency: "BTC", Available: Number(0.25)})
		assert.Equal(t, Number(0.25), worker.calculateSliceQuantity(startTime, Number(-2.0), false, market, fixedpoint.Zero))
	})

	t.Run("a reserve larger than the balance leaves nothing to sell", func(t *testing.T) {
		worker := newWorker()
		worker.SetReservedBaseFn(func(string) fixedpoint.Value { return Number(2) })
		assert.Equal(t, fixedpoint.Zero, worker.calculateSliceQuantity(startTime, Number(-2.0), false, market, fixedpoint.Zero))
	})

	t.Run("buy is not affected by the reserve", func(t *testing.T) {
		worker := newWorker()
		worker.SetReservedBaseFn(func(string) fixedpoint.Value { return Number(0.4) })
		assert.Equal(t, Number(0.4), worker.calculateSliceQuantity(startTime, Number(2.0), false, market, fixedpoint.Zero))
	})
}
