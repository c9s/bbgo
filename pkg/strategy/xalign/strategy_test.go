//go:build !dnum

package xalign

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/xalign/detector"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

// cat ~/.bbgo/cache/max-markets.json  | jq '.[] | select(.symbol == "USDTTWD")'
func getTestMarkets() types.MarketMap {
	return map[string]types.Market{
		"ETHBTC": {
			Exchange:        types.ExchangeMax,
			Symbol:          "ETHBTC",
			LocalSymbol:     "ETHBTC",
			PricePrecision:  6,
			VolumePrecision: 4,
			BaseCurrency:    "ETH",
			QuoteCurrency:   "BTC",
			MinNotional:     Number(0.00030000),
			MinAmount:       Number(0.00030000),
			MinQuantity:     Number(0.00460000),
			StepSize:        Number(0.00010000),
			TickSize:        Number(0.00000100),
		},
		"BTCUSDT": {
			Exchange:        types.ExchangeMax,
			Symbol:          "BTCUSDT",
			LocalSymbol:     "BTCUSDT",
			PricePrecision:  2,
			VolumePrecision: 6,
			BaseCurrency:    "BTC",
			QuoteCurrency:   "USDT",
			MinNotional:     Number(8.00000000),
			MinAmount:       Number(8.00000000),
			MinQuantity:     Number(0.00030000),
			StepSize:        Number(0.00000100),
			TickSize:        Number(0.01000000),
		},
		"BTCTWD": {
			Exchange:        types.ExchangeMax,
			Symbol:          "BTCTWD",
			LocalSymbol:     "BTCTWD",
			PricePrecision:  1,
			VolumePrecision: 8,
			BaseCurrency:    "BTC",
			QuoteCurrency:   "TWD",
			MinNotional:     Number(250.00000000),
			MinAmount:       Number(250.00000000),
			MinQuantity:     Number(0.00030000),
			StepSize:        Number(0.00000001),
			TickSize:        Number(0.01000000),
		},
		"ETHUSDT": {
			Exchange:        types.ExchangeMax,
			Symbol:          "ETHUSDT",
			LocalSymbol:     "ETHUSDT",
			PricePrecision:  2,
			VolumePrecision: 6,
			BaseCurrency:    "ETH",
			QuoteCurrency:   "USDT",
			MinNotional:     Number(8.00000000),
			MinAmount:       Number(8.00000000),
			MinQuantity:     Number(0.00460000),
			StepSize:        Number(0.00001000),
			TickSize:        Number(0.01000000),
		},
		"ETHTWD": {
			Exchange:        types.ExchangeMax,
			Symbol:          "ETHTWD",
			LocalSymbol:     "ETHTWD",
			PricePrecision:  1,
			VolumePrecision: 6,
			BaseCurrency:    "ETH",
			QuoteCurrency:   "TWD",
			MinNotional:     Number(250.00000000),
			MinAmount:       Number(250.00000000),
			MinQuantity:     Number(0.00460000),
			StepSize:        Number(0.00000100),
			TickSize:        Number(0.10000000),
		},
		"USDTTWD": {
			Exchange:        types.ExchangeMax,
			Symbol:          "USDTTWD",
			LocalSymbol:     "USDTTWD",
			PricePrecision:  3,
			VolumePrecision: 2,
			BaseCurrency:    "USDT",
			QuoteCurrency:   "TWD",
			MinNotional:     Number(250.00000000),
			MinAmount:       Number(250.00000000),
			MinQuantity:     Number(8.00000000),
			StepSize:        Number(0.01000000),
			TickSize:        Number(0.00100000),
		},
	}
}

// mockExchange combines Exchange and ExchangeTransferHistoryService for testing
type mockExchange struct {
	types.Exchange
	types.ExchangeTransferHistoryService
}

func TestStrategy(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx := context.Background()
	s := &Strategy{
		ExpectedBalances: map[string]fixedpoint.Value{
			"TWD": Number(10_000),
		},
		PreferredQuoteCurrencies: &QuoteCurrencyPreference{
			Buy:  []string{"TWD", "USDT"},
			Sell: []string{"USDT"},
		},
		PreferredSessions: []string{"max"},
		UseTakerOrder:     true,
	}

	testMarkets := getTestMarkets()

	t.Run("buy TWD", func(t *testing.T) {
		mockEx := mocks.NewMockExchange(mockCtrl)
		mockEx.EXPECT().QueryTicker(ctx, "USDTTWD").Return(&types.Ticker{
			Buy:  Number(32.0),
			Sell: Number(33.0),
		}, nil)

		account := types.NewAccount()
		account.AddBalance("TWD", Number(20_000))
		account.AddBalance("USDT", Number(80_000))
		mockEx.EXPECT().QueryAccount(ctx).Return(account, nil)

		session := &bbgo.ExchangeSession{
			Exchange: mockEx,
			Account:  account,
		}
		session.SetMarkets(testMarkets)
		sessions := map[string]*bbgo.ExchangeSession{}
		sessions["max"] = session

		_, submitOrder := s.selectSessionForCurrency(ctx, sessions, "TWD", Number(70_000))
		assert.NotNil(t, submitOrder)
		assert.Equal(t, types.SideTypeSell, submitOrder.Side)
		assert.Equal(t, Number(32).String(), submitOrder.Price.String())
		assert.Equal(t, "2187.5", submitOrder.Quantity.String(), "70_000 / 32 best bid = 2187.5")
	})

	t.Run("sell TWD", func(t *testing.T) {
		mockEx := mocks.NewMockExchange(mockCtrl)
		mockEx.EXPECT().QueryTicker(ctx, "USDTTWD").Return(&types.Ticker{
			Buy:  Number(32.0),
			Sell: Number(33.0),
		}, nil)

		account := types.NewAccount()
		account.AddBalance("TWD", Number(20_000))
		account.AddBalance("USDT", Number(80_000))
		mockEx.EXPECT().QueryAccount(ctx).Return(account, nil)

		session := &bbgo.ExchangeSession{
			Exchange: mockEx,
			Account:  account,
		}
		session.SetMarkets(testMarkets)
		sessions := map[string]*bbgo.ExchangeSession{}
		sessions["max"] = session

		_, submitOrder := s.selectSessionForCurrency(ctx, sessions, "TWD", Number(-10_000))
		assert.NotNil(t, submitOrder)
		assert.Equal(t, types.SideTypeBuy, submitOrder.Side)
		assert.Equal(t, Number(33).String(), submitOrder.Price.String())
		assert.Equal(t, "303.03", submitOrder.Quantity.String(), "10_000 / 33 best ask = 303.0303030303")
	})

	t.Run("buy BTC with USDT instead of TWD", func(t *testing.T) {
		mockEx := mocks.NewMockExchange(mockCtrl)

		mockEx.EXPECT().QueryTicker(ctx, "BTCTWD").Return(&types.Ticker{
			Sell: Number(36000.0 * 32),
			Buy:  Number(35000.0 * 31),
		}, nil)

		mockEx.EXPECT().QueryTicker(ctx, "BTCUSDT").Return(&types.Ticker{
			Sell: Number(36000.0),
			Buy:  Number(35000.0),
		}, nil)

		account := types.NewAccount()
		account.AddBalance("BTC", Number(0.955))
		account.AddBalance("TWD", Number(60_000))
		account.AddBalance("USDT", Number(80_000))
		mockEx.EXPECT().QueryAccount(ctx).Return(account, nil)

		// 36000.0 * 32 * 0.045

		session := &bbgo.ExchangeSession{
			Exchange: mockEx,
			Account:  account,
		}

		session.SetMarkets(testMarkets)
		sessions := map[string]*bbgo.ExchangeSession{}
		sessions["max"] = session

		_, submitOrder := s.selectSessionForCurrency(ctx, sessions, "BTC", Number(0.045))
		assert.NotNil(t, submitOrder)
		assert.Equal(t, types.SideTypeBuy, submitOrder.Side)
		assert.Equal(t, "36000", submitOrder.Price.String())
		assert.Equal(t, "0.045", submitOrder.Quantity.String())
	})
}

func Test_align(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx := context.Background()
	testMarkets := getTestMarkets()

	// Helper lambda to create mock exchange with transfer history
	newMockExchange := func(withdraws []types.Withdraw, deposits []types.Deposit) (*mocks.MockExchange, *mockExchange, *types.Account) {
		baseMockEx := mocks.NewMockExchange(mockCtrl)
		mockTransferHistory := mocks.NewMockExchangeTransferHistoryService(mockCtrl)

		// Setup account with balances that need refilling
		account := types.NewAccount()
		account.AddBalance("USDT", Number(5000)) // Need 10000, so need to refill 5000
		account.AddBalance("TWD", Number(20000))

		// Mock QueryAccount to return the account
		baseMockEx.EXPECT().QueryAccount(ctx).Return(account, nil).AnyTimes()

		// Mock CancelOrders for GracefulCancel in orderBooks
		baseMockEx.EXPECT().CancelOrders(ctx).Return(nil).AnyTimes()

		// Mock ticker query for USDTTWD market (for refilling USDT)
		baseMockEx.EXPECT().QueryTicker(ctx, "USDTTWD").Return(&types.Ticker{
			Buy:  Number(32.0),
			Sell: Number(33.0),
		}, nil).AnyTimes()

		// Mock the transfer history services
		mockTransferHistory.EXPECT().QueryWithdrawHistory(
			ctx,
			"",
			gomock.Any(), // since time
			gomock.Any(), // until time
		).Return(withdraws, nil).AnyTimes()

		mockTransferHistory.EXPECT().QueryDepositHistory(
			ctx,
			"",
			gomock.Any(), // since time
			gomock.Any(), // until time
		).Return(deposits, nil).AnyTimes()

		// Create composite exchange with transfer history
		mockEx := &mockExchange{
			Exchange:                       baseMockEx,
			ExchangeTransferHistoryService: mockTransferHistory,
		}

		return baseMockEx, mockEx, account
	}

	// Helper lambda to setup session
	newTestSession := func(mockEx *mockExchange, account *types.Account) map[string]*bbgo.ExchangeSession {
		session := &bbgo.ExchangeSession{
			ExchangeSessionConfig: bbgo.ExchangeSessionConfig{
				Name: "max",
			},
			Exchange: mockEx,
			Account:  account,
		}
		session.SetMarkets(testMarkets)

		return map[string]*bbgo.ExchangeSession{
			"max": session,
		}
	}

	// Helper lambda to create and initialize strategy
	newStrategyForTest := func(dryRun bool) *Strategy {
		s := &Strategy{
			ExpectedBalances: map[string]fixedpoint.Value{
				"USDT": Number(10000),
			},
			PreferredQuoteCurrencies: &QuoteCurrencyPreference{
				Buy:  []string{"TWD", "USDT"},
				Sell: []string{"TWD", "USDT"},
			},
			PreferredSessions:        []string{"max"},
			UseTakerOrder:            true,
			SkipTransferCheck:        false,
			ActiveTransferTimeWindow: types.Duration(48 * time.Hour),
			DryRun:                   dryRun,
		}

		// Initialize the strategy
		err := s.Initialize()
		assert.NoError(t, err)

		// Initialize orderBooks
		s.orderBooks = make(map[string]*bbgo.ActiveOrderBook)
		s.orderBooks["max"] = bbgo.NewActiveOrderBook("")

		// Initialize price resolver
		s.priceResolver = s.initializePriceResolver(testMarkets)

		return s
	}

	t.Run("with active transfers", func(t *testing.T) {
		// This is a pending withdraw that should trigger the active transfer flag
		activeWithdraw := types.Withdraw{
			Asset:  "USDT",
			Amount: Number(1000),
			Status: types.WithdrawStatusProcessing,
		}

		_, mockEx, account := newMockExchange([]types.Withdraw{activeWithdraw}, []types.Deposit{})
		sessions := newTestSession(mockEx, account)
		s := newStrategyForTest(false)

		activeTransferExists := s.align(ctx, sessions)

		// active transfer detected
		assert.True(t, activeTransferExists, "align should return true when there are active transfers")
	})

	t.Run("no active transfers", func(t *testing.T) {
		baseMockEx, mockEx, account := newMockExchange([]types.Withdraw{}, []types.Deposit{})

		// Mock order submission for refilling balances
		baseMockEx.EXPECT().SubmitOrder(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
			// Verify the order is for buying USDT
			assert.Equal(t, "USDTTWD", order.Symbol)
			assert.Equal(t, types.SideTypeBuy, order.Side)

			// Return a created order
			createdOrder := &types.Order{
				SubmitOrder:      order,
				OrderID:          1,
				Status:           types.OrderStatusNew,
				ExecutedQuantity: fixedpoint.Zero,
			}
			return createdOrder, nil
		}).Times(1)

		sessions := newTestSession(mockEx, account)
		s := newStrategyForTest(false)

		activeTransferExists := s.align(ctx, sessions)

		// no active transfer detected
		assert.False(t, activeTransferExists, "align should return false when there are no active transfers")
	})
}

func Test_updateDurations(t *testing.T) {
	s := &Strategy{
		ActiveTransferInterval: types.Duration(3 * time.Minute),
		Duration:               types.Duration(time.Minute * 30),
		ticker:                 time.NewTicker(1 * time.Second),
		deviationDetectors:     make(map[string]*detector.DeviationDetector[types.Balance]),
	}
	err := s.Defaults()
	assert.NoError(t, err)

	s.deviationDetectors["test"] = detector.NewDeviationDetector(
		types.NewBalance("USDT", fixedpoint.NewFromFloat(0.5)),
		0.01,
		5*time.Minute,
		s.netBalanceValue,
	)
	tickerDuration, detectDuration := s.nextDectectParams(true)
	s.updateDurations(tickerDuration, detectDuration)
	assert.Equal(t, 2*time.Hour, s.deviationDetectors["test"].GetDuration())

	tickerDuration, detectDuration = s.nextDectectParams(false)
	s.updateDurations(tickerDuration, detectDuration)
	assert.Equal(t, 30*time.Minute, s.deviationDetectors["test"].GetDuration())
}
