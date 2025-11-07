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
		assert.Equal(t, "@0", purrUsdcMarket.LocalSymbol)
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

func TestQueryAccount(t *testing.T) {
	t.Run("succeeds querying spot account", func(t *testing.T) {
		privateKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		ex := New(fmt.Sprintf("%x", crypto.FromECDSA(privateKey)), "")

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		spotMeta, err := os.ReadFile("./testdata/spotClearinghouseState.json")
		require.NoError(t, err)

		transport.POST("/info", func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusOK, string(spotMeta)), nil
		})

		account, err := ex.QueryAccount(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, account)

		// Verify account balances
		balances := account.Balances()
		assert.NotNil(t, balances)
		assert.Greater(t, len(balances), 0)

		// Verify specific balance data from test data
		usdcBalance, exists := balances["USDC"]
		assert.True(t, exists, "USDC balance should exist")
		assert.Equal(t, "USDC", usdcBalance.Currency)
		assert.Equal(t, "1808443.14193129", usdcBalance.NetAsset.String())
		assert.Equal(t, "1808443.14193129", usdcBalance.Available.String())
		assert.Equal(t, 0.0, usdcBalance.Locked.Float64())

		hypeBalance, exists := balances["HYPE"]
		assert.True(t, exists, "HYPE balance should exist")
		assert.Equal(t, "HYPE", hypeBalance.Currency)
		assert.Equal(t, "503808.46025243", hypeBalance.NetAsset.String())
		assert.Equal(t, "503778.46025243", hypeBalance.Available.String())
		assert.Equal(t, 30.0, hypeBalance.Locked.Float64())

		// Verify account type is spot (default)
		assert.Equal(t, types.AccountTypeSpot, account.AccountType)
	})

	t.Run("succeeds querying futures account", func(t *testing.T) {
		privateKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		ex := New(fmt.Sprintf("%x", crypto.FromECDSA(privateKey)), "")
		ex.IsFutures = true

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		clearinghouseState, err := os.ReadFile("./testdata/clearinghouseState.json")
		require.NoError(t, err)

		transport.POST("/info", func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusOK, string(clearinghouseState)), nil
		})

		account, err := ex.QueryAccount(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, account)

		// Verify account type is futures
		assert.Equal(t, types.AccountTypeFutures, account.AccountType)

		// Verify futures info is initialized
		assert.NotNil(t, account.FuturesInfo)
		assert.NotNil(t, account.FuturesInfo.Assets)
		assert.NotNil(t, account.FuturesInfo.Positions)

		futuresInfo := account.FuturesInfo

		// Verify futures account summary fields
		assert.Equal(t, 121968206.668798998, futuresInfo.TotalMarginBalance.Float64())
		assert.Equal(t, 46255052.1990050003, futuresInfo.TotalInitialMargin.Float64())
		assert.Equal(t, 12598833.5037019998, futuresInfo.TotalMaintMargin.Float64())
		assert.Equal(t, 75713154.4697940052, futuresInfo.AvailableBalance.Float64())

		// Verify positions count
		assert.Equal(t, 11, len(futuresInfo.Positions))

		// Verify BTC short position
		btcPosKey := types.NewPositionKey("BTCUSDC", types.PositionShort)
		btcPos, exists := futuresInfo.Positions[btcPosKey]
		assert.True(t, exists, "BTC short position should exist")
		assert.Equal(t, "BTCUSDC", btcPos.Symbol)
		assert.Equal(t, types.PositionShort, btcPos.PositionSide)
		assert.Equal(t, "BTC", btcPos.BaseCurrency)
		assert.Equal(t, "USDC", btcPos.QuoteCurrency)
		assert.Equal(t, "1186.74032", btcPos.Base.String())
		assert.Equal(t, 120498051.8718400002, btcPos.Quote.Float64())
		assert.False(t, btcPos.Isolated, "BTC position should be cross margin")
		assert.NotNil(t, btcPos.PositionRisk)
		assert.Equal(t, "10", btcPos.PositionRisk.Leverage.String())
		assert.Equal(t, "111616.8", btcPos.PositionRisk.EntryPrice.String())
		assert.Equal(t, 191751.85621501, btcPos.PositionRisk.LiquidationPrice.Float64())
		assert.Equal(t, 11962126.6084020007, btcPos.PositionRisk.UnrealizedPnL.Float64())
		assert.Equal(t, 120498051.8718400002, btcPos.PositionRisk.Notional.Float64())

		// Verify ETH short position
		ethPosKey := types.NewPositionKey("ETHUSDC", types.PositionShort)
		ethPos, exists := futuresInfo.Positions[ethPosKey]
		assert.True(t, exists, "ETH short position should exist")
		assert.Equal(t, "ETHUSDC", ethPos.Symbol)
		assert.Equal(t, types.PositionShort, ethPos.PositionSide)
		assert.Equal(t, "ETH", ethPos.BaseCurrency)
		assert.Equal(t, "51446.7322", ethPos.Base.String())
		assert.Equal(t, 170664244.7270599902, ethPos.Quote.Float64())
		assert.False(t, ethPos.Isolated, "ETH position should be cross margin")
		assert.NotNil(t, ethPos.PositionRisk)
		assert.Equal(t, "10", ethPos.PositionRisk.Leverage.String())
		assert.Equal(t, "3527.73", ethPos.PositionRisk.EntryPrice.String())
		assert.Equal(t, 5374.599353, ethPos.PositionRisk.LiquidationPrice.Float64())

		// Verify ASTER long position (the only long position in test data)
		asterPosKey := types.NewPositionKey("ASTERUSDC", types.PositionLong)
		asterPos, exists := futuresInfo.Positions[asterPosKey]
		assert.True(t, exists, "ASTER long position should exist")
		assert.Equal(t, "ASTERUSDC", asterPos.Symbol)
		assert.Equal(t, types.PositionLong, asterPos.PositionSide)
		assert.Equal(t, "ASTER", asterPos.BaseCurrency)
		assert.Equal(t, "3058645", asterPos.Base.String())
		assert.Equal(t, "3148263.2985", asterPos.Quote.String())
		assert.False(t, asterPos.Isolated, "ASTER position should be cross margin")
		assert.NotNil(t, asterPos.PositionRisk)
		assert.Equal(t, "3", asterPos.PositionRisk.Leverage.String())
		assert.Equal(t, "0.968318", asterPos.PositionRisk.EntryPrice.String())
	})

}
