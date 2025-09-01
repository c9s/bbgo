package hyperliquid

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/c9s/bbgo/pkg/testing/httptesting"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryMarkets(t *testing.T) {
	t.Run("succeeds querying spot markets", func(t *testing.T) {
		privateKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		ex := New(fmt.Sprintf("%x", crypto.FromECDSA(privateKey)), "")

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		spotMeta, err := os.ReadFile("./testdata/spotMeta.json")
		require.NoError(t, err)

		transport.POST("/info", func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusOK, string(spotMeta)), nil
		})

		markets, err := ex.QueryMarkets(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, markets)
		assert.Greater(t, len(markets), 0)

		// Verify specific market data
		purrUsdcMarket, exists := markets["PURRUSDC"]
		assert.True(t, exists, "PURRUSDC market should exist")
		assert.Equal(t, types.ExchangeHyperliquid, purrUsdcMarket.Exchange)
		assert.Equal(t, "PURRUSDC", purrUsdcMarket.Symbol)
		assert.Equal(t, "PURR", purrUsdcMarket.BaseCurrency)
		assert.Equal(t, "USDC", purrUsdcMarket.QuoteCurrency)
		assert.Equal(t, "1000", purrUsdcMarket.LocalSymbol)
		assert.Equal(t, 8, purrUsdcMarket.PricePrecision)
		assert.Equal(t, 0, purrUsdcMarket.VolumePrecision)
	})

	t.Run("succeeds querying preps markets", func(t *testing.T) {
		privateKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		ex := New(fmt.Sprintf("%x", crypto.FromECDSA(privateKey)), "")
		ex.IsFutures = true

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		perpsMeta, err := os.ReadFile("./testdata/perpsMeta.json")
		require.NoError(t, err)

		transport.POST("/info", func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusOK, string(perpsMeta)), nil
		})

		markets, err := ex.QueryMarkets(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, markets)
		assert.Greater(t, len(markets), 0)

		purrUsdcMarket, exists := markets["ETHUSDC"]
		assert.True(t, exists, "ETHUSDC market should exist")
		assert.Equal(t, "ETH", purrUsdcMarket.BaseCurrency)
		assert.Equal(t, "USDC", purrUsdcMarket.QuoteCurrency)
	})

}
