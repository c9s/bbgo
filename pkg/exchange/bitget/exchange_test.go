package bitget

import (
	"context"
	"math"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
	"github.com/c9s/bbgo/pkg/types"
)

func TestExchange_QueryMarkets(t *testing.T) {
	ex := New("key", "secret", "passphrase")

	t.Run("succeeds", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/get_symbols_request.json")
		assert.NoError(t, err)

		transport.GET("/api/v2/spot/public/symbols", func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusOK, string(f)), nil
		})

		mkts, err := ex.QueryMarkets(context.Background())
		assert.NoError(t, err)

		expMkts := types.MarketMap{
			"ETHUSDT": types.Market{
				Exchange:        types.ExchangeBitget,
				Symbol:          "ETHUSDT",
				LocalSymbol:     "ETHUSDT",
				PricePrecision:  2,
				VolumePrecision: 4,
				QuoteCurrency:   "USDT",
				BaseCurrency:    "ETH",
				MinNotional:     fixedpoint.NewFromInt(5),
				MinAmount:       fixedpoint.NewFromInt(5),
				MinQuantity:     fixedpoint.NewFromInt(0),
				MaxQuantity:     fixedpoint.NewFromInt(10000000000),
				StepSize:        fixedpoint.NewFromFloat(1.0 / math.Pow10(4)),
				TickSize:        fixedpoint.NewFromFloat(1.0 / math.Pow10(2)),
				MinPrice:        fixedpoint.Zero,
				MaxPrice:        fixedpoint.Zero,
			},
			"BTCUSDT": types.Market{
				Exchange:        types.ExchangeBitget,
				Symbol:          "BTCUSDT",
				LocalSymbol:     "BTCUSDT",
				PricePrecision:  2,
				VolumePrecision: 6,
				QuoteCurrency:   "USDT",
				BaseCurrency:    "BTC",
				MinNotional:     fixedpoint.NewFromInt(5),
				MinAmount:       fixedpoint.NewFromInt(5),
				MinQuantity:     fixedpoint.NewFromInt(0),
				MaxQuantity:     fixedpoint.NewFromInt(10000000000),
				StepSize:        fixedpoint.NewFromFloat(1.0 / math.Pow10(6)),
				TickSize:        fixedpoint.NewFromFloat(1.0 / math.Pow10(2)),
				MinPrice:        fixedpoint.Zero,
				MaxPrice:        fixedpoint.Zero,
			},
		}
		assert.Equal(t, expMkts, mkts)
	})

	t.Run("error", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/get_symbols_request_error.json")
		assert.NoError(t, err)

		transport.GET("/api/v2/spot/public/symbols", func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusBadRequest, string(f)), nil
		})

		_, err = ex.QueryMarkets(context.Background())
		assert.ErrorContains(t, err, "Invalid IP")
	})
}
