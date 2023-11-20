package bybit

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/exchange/bybit/mocks"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestFeeRatePoller_getAllFeeRates(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	unknownErr := errors.New("unknown err")

	t.Run("succeeds", func(t *testing.T) {
		mockMarketProvider := mocks.NewMockStreamDataProvider(mockCtrl)
		s := &feeRatePoller{
			client: mockMarketProvider,
		}

		ctx := context.Background()
		feeRates := bybitapi.FeeRates{
			List: []bybitapi.FeeRate{
				{
					Symbol:       "BTCUSDT",
					TakerFeeRate: fixedpoint.NewFromFloat(0.001),
					MakerFeeRate: fixedpoint.NewFromFloat(0.001),
				},
				{
					Symbol:       "ETHUSDT",
					TakerFeeRate: fixedpoint.NewFromFloat(0.001),
					MakerFeeRate: fixedpoint.NewFromFloat(0.001),
				},
				{
					Symbol:       "OPTIONCOIN",
					TakerFeeRate: fixedpoint.NewFromFloat(0.001),
					MakerFeeRate: fixedpoint.NewFromFloat(0.001),
				},
			},
		}

		mkts := types.MarketMap{
			"BTCUSDT": types.Market{
				Symbol:        "BTCUSDT",
				QuoteCurrency: "USDT",
				BaseCurrency:  "BTC",
			},
			"ETHUSDT": types.Market{
				Symbol:        "ETHUSDT",
				QuoteCurrency: "USDT",
				BaseCurrency:  "ETH",
			},
		}

		mockMarketProvider.EXPECT().GetAllFeeRates(ctx).Return(feeRates, nil).Times(1)
		mockMarketProvider.EXPECT().QueryMarkets(ctx).Return(mkts, nil).Times(1)

		expFeeRates := map[string]symbolFeeDetail{
			"BTCUSDT": {
				FeeRate:   feeRates.List[0],
				BaseCoin:  "BTC",
				QuoteCoin: "USDT",
			},
			"ETHUSDT": {
				FeeRate:   feeRates.List[1],
				BaseCoin:  "ETH",
				QuoteCoin: "USDT",
			},
		}
		symbolFeeDetails, err := s.getAllFeeRates(ctx)
		assert.NoError(t, err)
		assert.Equal(t, expFeeRates, symbolFeeDetails)
	})

	t.Run("failed to query markets", func(t *testing.T) {
		mockMarketProvider := mocks.NewMockStreamDataProvider(mockCtrl)
		s := &feeRatePoller{
			client: mockMarketProvider,
		}

		ctx := context.Background()
		feeRates := bybitapi.FeeRates{
			List: []bybitapi.FeeRate{
				{
					Symbol:       "BTCUSDT",
					TakerFeeRate: fixedpoint.NewFromFloat(0.001),
					MakerFeeRate: fixedpoint.NewFromFloat(0.001),
				},
				{
					Symbol:       "ETHUSDT",
					TakerFeeRate: fixedpoint.NewFromFloat(0.001),
					MakerFeeRate: fixedpoint.NewFromFloat(0.001),
				},
				{
					Symbol:       "OPTIONCOIN",
					TakerFeeRate: fixedpoint.NewFromFloat(0.001),
					MakerFeeRate: fixedpoint.NewFromFloat(0.001),
				},
			},
		}

		mockMarketProvider.EXPECT().GetAllFeeRates(ctx).Return(feeRates, nil).Times(1)
		mockMarketProvider.EXPECT().QueryMarkets(ctx).Return(nil, unknownErr).Times(1)

		symbolFeeDetails, err := s.getAllFeeRates(ctx)
		assert.Equal(t, fmt.Errorf("failed to get markets: %w", unknownErr), err)
		assert.Equal(t, map[string]symbolFeeDetail(nil), symbolFeeDetails)
	})

	t.Run("failed to get fee rates", func(t *testing.T) {
		mockMarketProvider := mocks.NewMockStreamDataProvider(mockCtrl)
		s := &feeRatePoller{
			client: mockMarketProvider,
		}

		ctx := context.Background()

		mockMarketProvider.EXPECT().GetAllFeeRates(ctx).Return(bybitapi.FeeRates{}, unknownErr).Times(1)

		symbolFeeDetails, err := s.getAllFeeRates(ctx)
		assert.Equal(t, fmt.Errorf("failed to call get fee rates: %w", unknownErr), err)
		assert.Equal(t, map[string]symbolFeeDetail(nil), symbolFeeDetails)
	})
}

func Test_feeRatePoller_Get(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockMarketProvider := mocks.NewMockStreamDataProvider(mockCtrl)
	t.Run("found", func(t *testing.T) {
		symbol := "BTCUSDT"
		expFeeDetail := symbolFeeDetail{
			FeeRate: bybitapi.FeeRate{
				Symbol:       symbol,
				TakerFeeRate: fixedpoint.NewFromFloat(0.1),
				MakerFeeRate: fixedpoint.NewFromFloat(0.2),
			},
			BaseCoin:  "BTC",
			QuoteCoin: "USDT",
		}

		s := &feeRatePoller{
			client: mockMarketProvider,
			symbolFeeDetail: map[string]symbolFeeDetail{
				symbol: expFeeDetail,
			},
		}

		res, found := s.Get(symbol)
		assert.True(t, found)
		assert.Equal(t, expFeeDetail, res)
	})
	t.Run("not found", func(t *testing.T) {
		symbol := "BTCUSDT"
		s := &feeRatePoller{
			client:          mockMarketProvider,
			symbolFeeDetail: map[string]symbolFeeDetail{},
		}

		_, found := s.Get(symbol)
		assert.False(t, found)
	})
}
