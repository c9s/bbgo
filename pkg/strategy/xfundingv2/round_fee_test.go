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

// mockFuturesServiceForFee extends the mock to track transfer calls
type mockFuturesServiceForFee struct {
	mockFuturesService
	transferredAsset     string
	transferredAmount    fixedpoint.Value
	transferredDirection types.TransferDirection
	transferCalled       bool
}

func (m *mockFuturesServiceForFee) TransferFuturesAccountAsset(
	ctx context.Context, asset string, amount fixedpoint.Value, direction types.TransferDirection,
) error {
	m.transferCalled = true
	m.transferredAsset = asset
	m.transferredAmount = amount
	m.transferredDirection = direction
	return m.transferErr
}

func TestStrategy_AquireFeeAssetAndTransfer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Helper to create a strategy with mocked sessions
	setup := func(
		spotBNBBalance, futuresBNBBalance fixedpoint.Value,
	) (*Strategy, *mocks.MockExchange, *mockFuturesServiceForFee) {
		mockExchange := mocks.NewMockExchange(ctrl)
		mockExchange.EXPECT().Name().Return(types.ExchangeBinance).AnyTimes()

		spotAccount := types.NewAccount()
		spotAccount.SetBalance("BNB", types.Balance{
			Currency:  "BNB",
			Available: spotBNBBalance,
		})

		futuresAccount := types.NewAccount()
		futuresAccount.SetBalance("BNB", types.Balance{
			Currency:  "BNB",
			Available: futuresBNBBalance,
		})

		spotSession := &bbgo.ExchangeSession{
			Account:  spotAccount,
			Exchange: mockExchange,
		}
		spotSession.SetMarkets(types.MarketMap{
			"BNBUSDT": {
				Symbol:          "BNBUSDT",
				BaseCurrency:    "BNB",
				QuoteCurrency:   "USDT",
				TickSize:        Number(0.01),
				StepSize:        Number(0.001),
				PricePrecision:  2,
				VolumePrecision: 3,
			},
		})

		futuresSession := &bbgo.ExchangeSession{
			Account: futuresAccount,
		}

		mockService := &mockFuturesServiceForFee{}

		// Create a GeneralOrderExecutor for the fee symbol so acquireFeeAssetAndTransfer can buy fee assets
		position := types.NewPositionFromMarket(types.Market{
			Symbol:          "BNBUSDT",
			BaseCurrency:    "BNB",
			QuoteCurrency:   "USDT",
			TickSize:        Number(0.01),
			StepSize:        Number(0.001),
			PricePrecision:  2,
			VolumePrecision: 3,
		})
		feeExecutor := bbgo.NewGeneralOrderExecutor(spotSession, "BNBUSDT", "xfundingv2", "test", position)

		spotOrderBooks := map[string]*types.StreamOrderBook{
			"BNBUSDT": newStreamOrderBookWithData("BNBUSDT",
				types.PriceVolumeSlice{{Price: Number(600), Volume: Number(100)}},
				types.PriceVolumeSlice{{Price: Number(601), Volume: Number(100)}},
			),
		}

		s := &Strategy{
			FeeSymbol:                 "BNBUSDT",
			spotSession:               spotSession,
			futuresSession:            futuresSession,
			futuresService:            mockService,
			spotGeneralOrderExecutors: map[string]*bbgo.GeneralOrderExecutor{"BNBUSDT": feeExecutor},
			spotOrderBooks:            spotOrderBooks,
			logger:                    logrus.StandardLogger(),
		}

		return s, mockExchange, mockService
	}

	// Helper to create rounds with preset fee amounts
	makeRounds := func(spotFee, futuresFee fixedpoint.Value) []*ArbitrageRound {
		round := &ArbitrageRound{}
		round.SetSpotFeeAssetAmount(spotFee)
		round.SetFuturesFeeAssetAmount(futuresFee)
		return []*ArbitrageRound{round}
	}

	t.Run("both accounts have sufficient fee asset", func(t *testing.T) {
		// Spot needs 0.5 BNB, has 1.0 BNB; Futures needs 0.3 BNB, has 1.0 BNB
		s, mockExchange, mockService := setup(Number(1.0), Number(1.0))
		mockExchange.EXPECT().SubmitOrder(gomock.Any(), gomock.Any()).Times(0)
		rounds := makeRounds(Number(0.5), Number(0.3))

		ctx := context.Background()
		err := s.acquireFeeAssetAndTransfer(ctx, rounds)

		assert.NoError(t, err)
		// No buy needed and no transfer needed
		assert.False(t, mockService.transferCalled)
	})

	t.Run("both accounts are in shortage of fee asset", func(t *testing.T) {
		// Spot needs 1.0 BNB, has 0.2 BNB; Futures needs 0.8 BNB, has 0.1 BNB
		// spotDeficit = 1.0 - 0.2 = 0.8, futuresDeficit = 0.8 - 0.1 = 0.7
		// total deficit = 0.8 + 0.7 = 1.5 > 0 → buy 1.5, transfer max(0, 0.7) = 0.7 to futures
		s, mockExchange, mockService := setup(Number("0.2"), Number("0.1"))
		rounds := makeRounds(Number("1.0"), Number("0.8"))

		mockExchange.EXPECT().
			SubmitOrder(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
				assert.Equal(t, "BNBUSDT", order.Symbol)
				assert.Equal(t, types.SideTypeBuy, order.Side)
				assert.Equal(t, types.OrderTypeMarket, order.Type)
				assert.Equal(t, Number(1.5), order.Quantity)
				return &types.Order{}, nil
			})

		ctx := context.Background()
		err := s.acquireFeeAssetAndTransfer(ctx, rounds)

		assert.NoError(t, err)
		assert.True(t, mockService.transferCalled)
		assert.Equal(t, "BNB", mockService.transferredAsset)
		assert.Equal(t, Number(0.7), mockService.transferredAmount)
		assert.Equal(t, types.TransferIn, mockService.transferredDirection)
	})

	t.Run("spot sufficient, futures not, total sufficient - transfer only", func(t *testing.T) {
		// Spot needs 0.5 BNB, has 2.0 BNB; Futures needs 1.0 BNB, has 0.3 BNB
		// spotDeficit = 0.5 - 2.0 = -1.5, futuresDeficit = 1.0 - 0.3 = 0.7
		// total = -1.5 + 0.7 = -0.8 < 0 → no buy needed
		// futuresDeficit > 0 → transfer 0.7 in (spot → futures)
		s, mockExchange, mockService := setup(Number(2.0), Number(0.3))
		mockExchange.EXPECT().SubmitOrder(gomock.Any(), gomock.Any()).Times(0)
		rounds := makeRounds(Number(0.5), Number(1.0))

		ctx := context.Background()
		err := s.acquireFeeAssetAndTransfer(ctx, rounds)

		assert.NoError(t, err)
		assert.True(t, mockService.transferCalled)
		assert.Equal(t, "BNB", mockService.transferredAsset)
		assert.Equal(t, Number(0.7), mockService.transferredAmount)
		assert.Equal(t, types.TransferIn, mockService.transferredDirection)
	})

	t.Run("spot sufficient, futures not, total insufficient - buy and transfer", func(t *testing.T) {
		// Spot needs 0.5 BNB, has 0.6 BNB; Futures needs 2.0 BNB, has 0.5 BNB
		// spotDeficit = 0.5 - 0.6 = -0.1, futuresDeficit = 2.0 - 0.5 = 1.5
		// total = -0.1 + 1.5 = 1.4 >= 0 → buy 1.4, transfer max(0, 1.5) = 1.5 to futures
		s, mockExchange, mockService := setup(Number(0.6), Number(0.5))
		rounds := makeRounds(Number(0.5), Number(2.0))

		mockExchange.EXPECT().
			SubmitOrder(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
				assert.Equal(t, "BNBUSDT", order.Symbol)
				assert.Equal(t, types.SideTypeBuy, order.Side)
				assert.Equal(t, types.OrderTypeMarket, order.Type)
				assert.Equal(t, Number(1.4), order.Quantity)
				return &types.Order{}, nil
			})

		ctx := context.Background()
		err := s.acquireFeeAssetAndTransfer(ctx, rounds)

		assert.NoError(t, err)
		assert.True(t, mockService.transferCalled)
		assert.Equal(t, "BNB", mockService.transferredAsset)
		assert.Equal(t, Number(1.5), mockService.transferredAmount)
		assert.Equal(t, types.TransferIn, mockService.transferredDirection)
	})

	t.Run("futures sufficient, spot not, total sufficient - transfer only", func(t *testing.T) {
		// Spot needs 1.0 BNB, has 0.4 BNB; Futures needs 0.3 BNB, has 2.0 BNB
		// spotDeficit = 1.0 - 0.4 = 0.6, futuresDeficit = 0.3 - 2.0 = -1.7
		// total = 0.6 + (-1.7) = -1.1 < 0 → no buy needed
		// spotDeficit > 0 → transfer 0.6 out (futures → spot)
		s, mockExchange, mockService := setup(Number(0.4), Number(2.0))
		mockExchange.EXPECT().SubmitOrder(gomock.Any(), gomock.Any()).Times(0)
		rounds := makeRounds(Number(1.0), Number(0.3))

		ctx := context.Background()
		err := s.acquireFeeAssetAndTransfer(ctx, rounds)

		assert.NoError(t, err)
		assert.True(t, mockService.transferCalled)
		assert.Equal(t, "BNB", mockService.transferredAsset)
		assert.Equal(t, Number(0.6), mockService.transferredAmount)
		assert.Equal(t, types.TransferOut, mockService.transferredDirection)
	})

	t.Run("futures sufficient, spot not, total insufficient - buy and transfer", func(t *testing.T) {
		// Spot needs 2.0 BNB, has 0.3 BNB; Futures needs 0.5 BNB, has 0.6 BNB
		// spotDeficit = 2.0 - 0.3 = 1.7, futuresDeficit = 0.5 - 0.6 = -0.1
		// total = 1.7 + (-0.1) = 1.6 >= 0 → buy 1.6, transfer max(0, -0.1) = 0 to futures
		// futuresDeficit <= 0 and spotDeficit > 0
		// so transferAmount = futuresDeficit.Neg() = 0.1 → transfer out
		s, mockExchange, mockService := setup(Number(0.3), Number(0.6))
		rounds := makeRounds(Number(2.0), Number(0.5))

		mockExchange.EXPECT().
			SubmitOrder(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
				assert.Equal(t, "BNBUSDT", order.Symbol)
				assert.Equal(t, types.SideTypeBuy, order.Side)
				assert.Equal(t, types.OrderTypeMarket, order.Type)
				assert.Equal(t, Number(1.6), order.Quantity)
				return &types.Order{}, nil
			})

		ctx := context.Background()
		err := s.acquireFeeAssetAndTransfer(ctx, rounds)

		assert.NoError(t, err)
		// transfer needed:
		assert.True(t, mockService.transferCalled)
		assert.Equal(t, Number(0.1), mockService.transferredAmount)
		assert.Equal(t, types.TransferOut, mockService.transferredDirection)
	})
}

func newStreamOrderBookWithData(symbol string, bids, asks types.PriceVolumeSlice) *types.StreamOrderBook {
	book := types.NewStreamBook(symbol, types.ExchangeBinance)
	book.Load(types.SliceOrderBook{
		Symbol: symbol,
		Bids:   bids,
		Asks:   asks,
	})
	return book
}

func TestStrategy_CalculateRoundFeeAsset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("returns nil when fee order book is not found", func(t *testing.T) {
		s := &Strategy{
			FeeSymbol:      "BNBUSDT",
			spotOrderBooks: map[string]*types.StreamOrderBook{},
		}

		nextFundingTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)

		err := s.calculateRoundFeeAsset(round)
		assert.NoError(t, err)
		assert.Equal(t, fixedpoint.Zero, round.SpotFeeAssetAmount())
		assert.Equal(t, fixedpoint.Zero, round.FuturesFeeAssetAmount())
	})

	t.Run("returns error when order book data is not ready (positive target position)", func(t *testing.T) {
		// Order books exist but have no data on the relevant side
		spotOrderBooks := map[string]*types.StreamOrderBook{
			"BNBUSDT": newStreamOrderBookWithData("BNBUSDT", nil,
				types.PriceVolumeSlice{
					{Price: Number(600), Volume: Number(100)},
				}),
			"BTCUSDT": newStreamOrderBookWithData("BTCUSDT", nil, nil), // empty ask side
		}
		futuresOrderBooks := map[string]*types.StreamOrderBook{
			"BTCUSDT": newStreamOrderBookWithData("BTCUSDT", nil, nil), // empty bid side
		}

		s := &Strategy{
			FeeSymbol:         "BNBUSDT",
			spotSession:       &bbgo.ExchangeSession{},
			futuresSession:    &bbgo.ExchangeSession{},
			spotOrderBooks:    spotOrderBooks,
			futuresOrderBooks: futuresOrderBooks,
		}

		nextFundingTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)
		// spotWorker target = 1.0 (positive), so it enters the positive branch

		err := s.calculateRoundFeeAsset(round)
		assert.EqualError(t, err, "order book data is not ready yet")
	})

	t.Run("returns error when order book data is not ready (negative target position)", func(t *testing.T) {
		spotOrderBooks := map[string]*types.StreamOrderBook{
			"BNBUSDT": newStreamOrderBookWithData("BNBUSDT", nil,
				types.PriceVolumeSlice{
					{Price: Number(600), Volume: Number(100)},
				}),
			"BTCUSDT": newStreamOrderBookWithData("BTCUSDT", nil, nil), // empty bid side
		}
		futuresOrderBooks := map[string]*types.StreamOrderBook{
			"BTCUSDT": newStreamOrderBookWithData("BTCUSDT", nil, nil), // empty ask side
		}

		s := &Strategy{
			FeeSymbol:         "BNBUSDT",
			spotSession:       &bbgo.ExchangeSession{},
			futuresSession:    &bbgo.ExchangeSession{},
			spotOrderBooks:    spotOrderBooks,
			futuresOrderBooks: futuresOrderBooks,
		}

		nextFundingTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		config := TWAPWorkerConfig{
			Duration:  types.Duration(10 * time.Minute),
			NumSlices: 5,
		}
		spotWorker, _, _, _ := newTestTWAPWorker(t, ctrl, config)
		spotWorker.SetTargetPosition(Number(-1.0)) // negative target position

		futuresWorker, _, _, _ := newTestTWAPWorker(t, ctrl, config)
		futuresWorker.SetTargetPosition(Number(1.0))

		mockService := &mockFuturesService{}
		fundingRate := &types.PremiumIndex{
			LastFundingRate: Number(0.001),
			NextFundingTime: nextFundingTime,
		}
		round := NewArbitrageRound(
			fundingRate,
			types.ExchangeBinance, types.ExchangeBinance,
			3, 8, Number(3),
			spotWorker, futuresWorker, mockService,
			types.PositionLong, time.Minute)

		err := s.calculateRoundFeeAsset(round)
		assert.EqualError(t, err, "order book data is not ready yet")
	})

	t.Run("calculates fee amounts correctly (positive target position)", func(t *testing.T) {
		// Target position = 1.0 BTC (long spot, short futures)
		// Spot ask price = 50000, Futures bid price = 50000
		// Spot fee rate = 0.001, Futures fee rate = 0.001
		// Spot fee in quote = 50000 * 1.0 * 0.001 = 50
		// Futures fee in quote = 50000 * 1.0 * 0.001 = 50
		// Total fee in quote = 100
		// Fee asset (BNB) ask price for 100 USDT depth = 500
		// Spot fee amount = 50 / 500 = 0.1
		// Futures fee amount = 50 / 500 = 0.1
		spotOrderBooks := map[string]*types.StreamOrderBook{
			"BNBUSDT": newStreamOrderBookWithData("BNBUSDT", nil,
				types.PriceVolumeSlice{
					{Price: Number(500), Volume: Number(100)},
				}),
			"BTCUSDT": newStreamOrderBookWithData("BTCUSDT", nil,
				types.PriceVolumeSlice{
					{Price: Number(50000), Volume: Number(10)},
				}),
		}
		futuresOrderBooks := map[string]*types.StreamOrderBook{
			"BTCUSDT": newStreamOrderBookWithData("BTCUSDT",
				types.PriceVolumeSlice{
					{Price: Number(50000), Volume: Number(10)},
				}, nil),
		}

		spotSession := &bbgo.ExchangeSession{}
		spotSession.TakerFeeRate = Number(0.001)
		futuresSession := &bbgo.ExchangeSession{}
		futuresSession.TakerFeeRate = Number(0.001)

		s := &Strategy{
			FeeSymbol:         "BNBUSDT",
			spotSession:       spotSession,
			futuresSession:    futuresSession,
			spotOrderBooks:    spotOrderBooks,
			futuresOrderBooks: futuresOrderBooks,
		}

		nextFundingTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)

		err := s.calculateRoundFeeAsset(round)
		assert.NoError(t, err)

		// Both legs have the same price and fee rate, so fee amounts are equal
		assert.Equal(t, Number(0.2), round.SpotFeeAssetAmount())
		assert.Equal(t, Number(0.2), round.FuturesFeeAssetAmount())
	})

	t.Run("calculates fee amounts correctly (negative target position)", func(t *testing.T) {
		// Target position = -1.0 BTC (short spot, long futures)
		// Spot bid price = 50000, Futures ask price = 50000
		// Spot fee rate = 0.001, Futures fee rate = 0.001
		// Spot fee in quote = 50000 * 1.0 * 0.001 = 50
		// Futures fee in quote = 50000 * 1.0 * 0.001 = 50
		// Total fee in quote = 100
		// Fee asset (BNB) ask price = 500
		// Spot fee amount = 50 / 500 = 0.1
		// Futures fee amount = 50 / 500 = 0.1
		spotOrderBooks := map[string]*types.StreamOrderBook{
			"BNBUSDT": newStreamOrderBookWithData("BNBUSDT", nil,
				types.PriceVolumeSlice{
					{Price: Number(500), Volume: Number(100)},
				}),
			"BTCUSDT": newStreamOrderBookWithData("BTCUSDT",
				types.PriceVolumeSlice{
					{Price: Number(50000), Volume: Number(10)},
				}, nil),
		}
		futuresOrderBooks := map[string]*types.StreamOrderBook{
			"BTCUSDT": newStreamOrderBookWithData("BTCUSDT", nil,
				types.PriceVolumeSlice{
					{Price: Number(50000), Volume: Number(10)},
				}),
		}

		spotSession := &bbgo.ExchangeSession{}
		spotSession.TakerFeeRate = Number(0.001)
		futuresSession := &bbgo.ExchangeSession{}
		futuresSession.TakerFeeRate = Number(0.001)

		s := &Strategy{
			FeeSymbol:         "BNBUSDT",
			spotSession:       spotSession,
			futuresSession:    futuresSession,
			spotOrderBooks:    spotOrderBooks,
			futuresOrderBooks: futuresOrderBooks,
		}

		nextFundingTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
		config := TWAPWorkerConfig{
			Duration:  types.Duration(10 * time.Minute),
			NumSlices: 5,
		}
		spotWorker, _, _, _ := newTestTWAPWorker(t, ctrl, config)
		spotWorker.SetTargetPosition(Number(-1.0))

		futuresWorker, _, _, _ := newTestTWAPWorker(t, ctrl, config)
		futuresWorker.SetTargetPosition(Number(1.0))

		mockService := &mockFuturesService{}
		fundingRate := &types.PremiumIndex{
			LastFundingRate: Number(0.001),
			NextFundingTime: nextFundingTime,
		}
		round := NewArbitrageRound(
			fundingRate,
			types.ExchangeBinance, types.ExchangeBinance,
			3, 8, Number(3), spotWorker, futuresWorker, mockService,
			types.PositionShort, time.Minute)

		err := s.calculateRoundFeeAsset(round)
		assert.NoError(t, err)

		assert.Equal(t, Number(0.2), round.SpotFeeAssetAmount())
		assert.Equal(t, Number(0.2), round.FuturesFeeAssetAmount())
	})
}
