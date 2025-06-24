//go:build !dnum

package tradingdesk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

func TestStrategy_calculatePositionSize(t *testing.T) {
	ctx := context.Background()

	// Setup market
	market := types.Market{
		Symbol:          "BTCUSDT",
		BaseCurrency:    "BTC",
		QuoteCurrency:   "USDT",
		PricePrecision:  2,
		VolumePrecision: 6,
		MinNotional:     Number(8.0),
		MinQuantity:     Number(0.001),
		StepSize:        Number(0.00001),
		TickSize:        Number(0.01),
	}

	t.Run("Buy Order - Normal Case", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		// Setup account with balances
		account := types.NewAccount()
		account.AddBalance("USDT", Number(10000)) // 10,000 USDT
		account.AddBalance("BTC", Number(0.5))    // 0.5 BTC

		// Create mock exchange
		mockExchange := mocks.NewMockExchange(mockCtrl)
		ticker := &types.Ticker{
			Buy:  Number(50100), // Ask price
			Sell: Number(49900), // Bid price
			Last: Number(50000),
		}
		mockExchange.EXPECT().QueryTicker(ctx, "BTCUSDT").Return(ticker, nil).Times(1)

		// Create session
		mockSession := &bbgo.ExchangeSession{
			Exchange: mockExchange,
			Account:  account,
		}
		mockSession.SetMarkets(types.MarketMap{"BTCUSDT": market})

		strategy := &Strategy{
			Session:      mockSession,
			MaxLossLimit: Number(100), // 100 USDT max loss
		}

		param := OpenPositionParam{
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeBuy,
			Quantity:      Number(1.0),   // Want to buy 1 BTC
			StopLossPrice: Number(49100), // Stop at 49,100
		}

		quantity, err := strategy.calculatePositionSize(ctx, param)

		assert.NoError(t, err)
		// Risk per unit = 50100 - 49100 = 1000 USDT
		// Max quantity by risk = 100 / 1000 = 0.1 BTC
		// Max quantity by balance = 10000 / 50100 = 0.1996 BTC
		// Final quantity = min(1.0, 0.1, 0.1996) = 0.1 BTC
		assert.Equal(t, "0.1", quantity.String())
	})

	t.Run("Sell Order - Normal Case", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		// Setup account with balances
		account := types.NewAccount()
		account.AddBalance("USDT", Number(10000))
		account.AddBalance("BTC", Number(0.5))

		mockExchange := mocks.NewMockExchange(mockCtrl)
		ticker := &types.Ticker{
			Buy:  Number(50100),
			Sell: Number(49900), // Bid price for sell
			Last: Number(50000),
		}
		mockExchange.EXPECT().QueryTicker(ctx, "BTCUSDT").Return(ticker, nil).Times(1)

		mockSession := &bbgo.ExchangeSession{
			Exchange: mockExchange,
			Account:  account,
		}
		mockSession.SetMarkets(types.MarketMap{"BTCUSDT": market})

		strategy := &Strategy{
			Session:      mockSession,
			MaxLossLimit: Number(100),
		}

		param := OpenPositionParam{
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeSell,
			Quantity:      Number(1.0),   // Want to sell 1 BTC
			StopLossPrice: Number(50900), // Stop at 50,900
		}

		quantity, err := strategy.calculatePositionSize(ctx, param)

		assert.NoError(t, err)
		// Risk per unit = 50900 - 49900 = 1000 USDT
		// Max quantity by risk = 100 / 1000 = 0.1 BTC
		// Max quantity by balance = 0.5 BTC (available BTC)
		// Final quantity = min(1.0, 0.1, 0.5) = 0.1 BTC
		assert.Equal(t, "0.1", quantity.String())
	})

	t.Run("No Stop Loss - Return Original Quantity", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		account := types.NewAccount()
		account.AddBalance("USDT", Number(10000))
		account.AddBalance("BTC", Number(0.5))

		mockExchange := mocks.NewMockExchange(mockCtrl)
		mockSession := &bbgo.ExchangeSession{
			Exchange: mockExchange,
			Account:  account,
		}
		mockSession.SetMarkets(types.MarketMap{"BTCUSDT": market})

		strategy := &Strategy{
			Session:      mockSession,
			MaxLossLimit: Number(100),
		}

		param := OpenPositionParam{
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeBuy,
			Quantity:      Number(0.05),
			StopLossPrice: fixedpoint.Zero, // No stop loss
		}

		quantity, err := strategy.calculatePositionSize(ctx, param)

		assert.NoError(t, err)
		assert.Equal(t, "0.05", quantity.String())
	})

	t.Run("Invalid Stop Loss Price - Buy Order", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		account := types.NewAccount()
		account.AddBalance("USDT", Number(10000))
		account.AddBalance("BTC", Number(0.5))

		mockExchange := mocks.NewMockExchange(mockCtrl)
		ticker := &types.Ticker{
			Buy:  Number(50100),
			Sell: Number(49900),
			Last: Number(50000),
		}
		mockExchange.EXPECT().QueryTicker(ctx, "BTCUSDT").Return(ticker, nil).Times(1)

		mockSession := &bbgo.ExchangeSession{
			Exchange: mockExchange,
			Account:  account,
		}
		mockSession.SetMarkets(types.MarketMap{"BTCUSDT": market})

		strategy := &Strategy{
			Session:      mockSession,
			MaxLossLimit: Number(100),
		}

		param := OpenPositionParam{
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeBuy,
			Quantity:      Number(1.0),
			StopLossPrice: Number(51000), // Stop loss above current price
		}

		_, err := strategy.calculatePositionSize(ctx, param)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid stop loss price")
	})

	t.Run("Invalid Stop Loss Price - Sell Order", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		account := types.NewAccount()
		account.AddBalance("USDT", Number(10000))
		account.AddBalance("BTC", Number(0.5))

		mockExchange := mocks.NewMockExchange(mockCtrl)
		ticker := &types.Ticker{
			Buy:  Number(50100),
			Sell: Number(49900),
			Last: Number(50000),
		}
		mockExchange.EXPECT().QueryTicker(ctx, "BTCUSDT").Return(ticker, nil).Times(1)

		mockSession := &bbgo.ExchangeSession{
			Exchange: mockExchange,
			Account:  account,
		}
		mockSession.SetMarkets(types.MarketMap{"BTCUSDT": market})

		strategy := &Strategy{
			Session:      mockSession,
			MaxLossLimit: Number(100),
		}

		param := OpenPositionParam{
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeSell,
			Quantity:      Number(1.0),
			StopLossPrice: Number(49000), // Stop loss below current price
		}

		_, err := strategy.calculatePositionSize(ctx, param)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid stop loss price")
	})

	t.Run("Insufficient Balance - Buy Order", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		// Create strategy with low balance
		lowBalanceAccount := types.NewAccount()
		lowBalanceAccount.AddBalance("USDT", Number(10)) // Only 10 USDT

		mockExchange := mocks.NewMockExchange(mockCtrl)
		lowBalanceSession := &bbgo.ExchangeSession{
			Exchange: mockExchange,
			Account:  lowBalanceAccount,
		}
		lowBalanceSession.SetMarkets(types.MarketMap{"BTCUSDT": market})

		lowBalanceStrategy := &Strategy{
			Session:      lowBalanceSession,
			MaxLossLimit: Number(100),
		}

		ticker := &types.Ticker{
			Buy:  Number(50100),
			Sell: Number(49900),
			Last: Number(50000),
		}
		mockExchange.EXPECT().QueryTicker(ctx, "BTCUSDT").Return(ticker, nil).Times(1)

		param := OpenPositionParam{
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeBuy,
			Quantity:      Number(1.0),
			StopLossPrice: Number(49100),
		}

		quantity, err := lowBalanceStrategy.calculatePositionSize(ctx, param)

		assert.NoError(t, err)
		// Max quantity by balance = 10 / 50100 ≈ 0.0001996 BTC
		// This should be the limiting factor
		expected := Number(10).Div(Number(50100))
		assert.Equal(t, expected.String(), quantity.String())
	})

	t.Run("Zero MaxLossLimit - Use Original Quantity", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		account := types.NewAccount()
		account.AddBalance("USDT", Number(10000))
		account.AddBalance("BTC", Number(0.5))

		mockExchange := mocks.NewMockExchange(mockCtrl)
		mockSession := &bbgo.ExchangeSession{
			Exchange: mockExchange,
			Account:  account,
		}
		mockSession.SetMarkets(types.MarketMap{"BTCUSDT": market})

		strategyNoLimit := &Strategy{
			Session:      mockSession,
			MaxLossLimit: fixedpoint.Zero, // No loss limit
		}

		ticker := &types.Ticker{
			Buy:  Number(50100),
			Sell: Number(49900),
			Last: Number(50000),
		}
		mockExchange.EXPECT().QueryTicker(ctx, "BTCUSDT").Return(ticker, nil).Times(1)

		param := OpenPositionParam{
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeBuy,
			Quantity:      Number(0.15), // Want 0.15 BTC
			StopLossPrice: Number(49100),
		}

		quantity, err := strategyNoLimit.calculatePositionSize(ctx, param)

		assert.NoError(t, err)
		// Max quantity by balance = 10000 / 50100 ≈ 0.1996 BTC
		// Final quantity = min(0.15, 0.15, 0.1996) = 0.15 BTC
		assert.Equal(t, "0.15", quantity.String())
	})
}
