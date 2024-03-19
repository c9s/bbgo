//go:build !dnum

package xalign

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
