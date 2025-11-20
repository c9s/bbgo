package hyperliquid

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/c9s/bbgo/pkg/exchange/hyperliquid/hyperapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockTestExchange(t *testing.T, isFutures bool) (*Exchange, *httptesting.MockTransport) {
	t.Helper()

	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	ex := New(fmt.Sprintf("%x", crypto.FromECDSA(privateKey)), crypto.PubkeyToAddress(privateKey.PublicKey).Hex(), "")
	ex.IsFutures = isFutures

	transport := &httptesting.MockTransport{}
	ex.client.HttpClient.Transport = transport

	return ex, transport
}

func TestQueryMarkets(t *testing.T) {
	t.Run("succeeds querying spot markets", func(t *testing.T) {
		ex, transport := newMockTestExchange(t, false)

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
		assert.Equal(t, "PURR@0", purrUsdcMarket.LocalSymbol)
		assert.Equal(t, 8, purrUsdcMarket.PricePrecision)
		assert.Equal(t, 0, purrUsdcMarket.VolumePrecision)
	})

	t.Run("succeeds querying preps markets", func(t *testing.T) {
		ex, transport := newMockTestExchange(t, true)

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
		ex, transport := newMockTestExchange(t, false)

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
		ex, transport := newMockTestExchange(t, true)

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
		// Use InDelta for floating point comparisons to handle platform-specific precision differences
		const delta = 1e-6 // Allow 0.000001 difference for large numbers
		assert.InDelta(t, 121968206.668798998, futuresInfo.TotalMarginBalance.Float64(), delta)
		assert.InDelta(t, 46255052.1990050003, futuresInfo.TotalInitialMargin.Float64(), delta)
		assert.InDelta(t, 12598833.5037019998, futuresInfo.TotalMaintMargin.Float64(), delta)
		assert.InDelta(t, 75713154.4697940052, futuresInfo.AvailableBalance.Float64(), delta)

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
		assert.InDelta(t, 120498051.8718400002, btcPos.Quote.Float64(), delta)
		assert.False(t, btcPos.Isolated, "BTC position should be cross margin")
		assert.NotNil(t, btcPos.PositionRisk)
		assert.Equal(t, "10", btcPos.PositionRisk.Leverage.String())
		assert.Equal(t, "111616.8", btcPos.PositionRisk.EntryPrice.String())
		assert.InDelta(t, 191751.8562150183, btcPos.PositionRisk.LiquidationPrice.Float64(), delta)
		assert.InDelta(t, 11962126.6084020007, btcPos.PositionRisk.UnrealizedPnL.Float64(), delta)
		assert.InDelta(t, 120498051.8718400002, btcPos.PositionRisk.Notional.Float64(), delta)

		// Verify ETH short position
		ethPosKey := types.NewPositionKey("ETHUSDC", types.PositionShort)
		ethPos, exists := futuresInfo.Positions[ethPosKey]
		assert.True(t, exists, "ETH short position should exist")
		assert.Equal(t, "ETHUSDC", ethPos.Symbol)
		assert.Equal(t, types.PositionShort, ethPos.PositionSide)
		assert.Equal(t, "ETH", ethPos.BaseCurrency)
		assert.Equal(t, "51446.7322", ethPos.Base.String())
		assert.InDelta(t, 170664244.7270599902, ethPos.Quote.Float64(), delta)
		assert.False(t, ethPos.Isolated, "ETH position should be cross margin")
		assert.NotNil(t, ethPos.PositionRisk)
		assert.Equal(t, "10", ethPos.PositionRisk.Leverage.String())
		assert.Equal(t, "3527.73", ethPos.PositionRisk.EntryPrice.String())
		assert.InDelta(t, 5374.5993530082, ethPos.PositionRisk.LiquidationPrice.Float64(), delta)

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

func TestQueryAccountBalances(t *testing.T) {
	t.Run("succeeds querying spot account balances", func(t *testing.T) {
		ex, transport := newMockTestExchange(t, false)

		spotMeta, err := os.ReadFile("./testdata/spotClearinghouseState.json")
		require.NoError(t, err)

		transport.POST("/info", func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusOK, string(spotMeta)), nil
		})

		balances, err := ex.QueryAccountBalances(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, balances)
		assert.Greater(t, len(balances), 0)

		// Verify USDC balance
		usdcBalance, exists := balances["USDC"]
		assert.True(t, exists, "USDC balance should exist")
		assert.Equal(t, "USDC", usdcBalance.Currency)
		assert.Equal(t, "1808443.14193129", usdcBalance.NetAsset.String())
		assert.Equal(t, "1808443.14193129", usdcBalance.Available.String())
		assert.Equal(t, 0.0, usdcBalance.Locked.Float64())
		assert.Equal(t, "1808443.14193129", usdcBalance.MaxWithdrawAmount.String())

		// Verify HYPE balance
		hypeBalance, exists := balances["HYPE"]
		assert.True(t, exists, "HYPE balance should exist")
		assert.Equal(t, "HYPE", hypeBalance.Currency)
		assert.Equal(t, "503808.46025243", hypeBalance.NetAsset.String())
		assert.Equal(t, "503778.46025243", hypeBalance.Available.String())
		assert.Equal(t, 30.0, hypeBalance.Locked.Float64())
		assert.Equal(t, "503778.46025243", hypeBalance.MaxWithdrawAmount.String())

		// Verify LATINA balance
		latinaBalance, exists := balances["LATINA"]
		assert.True(t, exists, "LATINA balance should exist")
		assert.Equal(t, "LATINA", latinaBalance.Currency)
		assert.Equal(t, "45658.0657", latinaBalance.NetAsset.String())
		assert.Equal(t, "45658.0657", latinaBalance.Available.String())
		assert.Equal(t, 0.0, latinaBalance.Locked.Float64())

		// Verify UBTC balance (zero balance)
		ubtcBalance, exists := balances["UBTC"]
		assert.True(t, exists, "UBTC balance should exist")
		assert.Equal(t, "UBTC", ubtcBalance.Currency)
		assert.Equal(t, "0", ubtcBalance.NetAsset.String())
		assert.Equal(t, "0", ubtcBalance.Available.String())
		assert.Equal(t, 0.0, ubtcBalance.Locked.Float64())
	})
}

func TestQueryKLines(t *testing.T) {
	t.Run("succeeds querying futures klines without time bounds", func(t *testing.T) {
		ex, transport := newMockTestExchange(t, true)

		candles, err := os.ReadFile("./testdata/candles.json")
		require.NoError(t, err)

		transport.POST("/info", func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)

			var payload map[string]any
			require.NoError(t, json.Unmarshal(body, &payload))

			reqType, ok := payload["type"].(string)
			require.True(t, ok)
			assert.Equal(t, "candleSnapshot", reqType)

			reqPayload, ok := payload["req"].(map[string]any)
			require.True(t, ok)

			assert.Equal(t, "BTC", reqPayload["coin"])
			assert.Equal(t, "1h", reqPayload["interval"])
			_, hasStart := reqPayload["startTime"]
			assert.True(t, hasStart)
			_, hasEnd := reqPayload["endTime"]
			assert.False(t, hasEnd)

			return httptesting.BuildResponseString(http.StatusOK, string(candles)), nil
		})

		kLines, err := ex.QueryKLines(context.Background(), "BTCUSDC", types.Interval1h, types.KLineQueryOptions{})
		require.NoError(t, err)
		require.Len(t, kLines, 3)

		first := kLines[0]
		assert.Equal(t, types.ExchangeHyperliquid, first.Exchange)
		assert.Equal(t, "BTCUSDC", first.Symbol)
		assert.Equal(t, types.Interval1h, first.Interval)
		assert.True(t, first.Closed)
		assert.Equal(t, uint64(14946), first.NumberOfTrades)

		assert.Equal(t, int64(1762776000000), first.StartTime.Time().UnixMilli())
		assert.Equal(t, int64(1762779599999), first.EndTime.Time().UnixMilli())
		assert.Equal(t, fixedpoint.MustNewFromString("106211.0"), first.Open)
		assert.Equal(t, fixedpoint.MustNewFromString("105968.0"), first.Close)
		assert.Equal(t, fixedpoint.MustNewFromString("106248.0"), first.High)
		assert.Equal(t, fixedpoint.MustNewFromString("105886.0"), first.Low)
		assert.Equal(t, fixedpoint.MustNewFromString("736.72125"), first.Volume)
	})
}

func TestMockExchange_SubmitOrder(t *testing.T) {

	t.Run("places limit order and returns resting response", func(t *testing.T) {
		ex, transport := newMockTestExchange(t, true)
		ctx := context.Background()

		quantity := fixedpoint.MustNewFromString("0.5")
		price := fixedpoint.MustNewFromString("100000")

		transport.POST("/exchange", func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)

			var payload struct {
				Action struct {
					Type     string           `json:"type"`
					Orders   []hyperapi.Order `json:"orders"`
					Grouping string           `json:"grouping"`
				} `json:"action"`
			}

			require.NoError(t, json.Unmarshal(body, &payload))
			require.Len(t, payload.Action.Orders, 1)

			submitted := payload.Action.Orders[0]
			assert.Equal(t, 0, submitted.Asset)
			assert.True(t, submitted.IsBuy)
			assert.Equal(t, quantity.String(), submitted.Size)
			assert.Equal(t, price.String(), submitted.Price)
			assert.False(t, submitted.ReduceOnly)
			assert.Equal(t, hyperapi.TimeInForceGTC, submitted.OrderType.Limit.Tif)

			return httptesting.BuildResponseJson(http.StatusOK, map[string]any{
				"status": "success",
				"response": map[string]any{
					"type": "order",
					"data": map[string]any{
						"statuses": []any{
							map[string]any{
								"resting": map[string]any{"oid": 123456},
							},
						},
					},
				},
			}), nil
		})

		order := types.SubmitOrder{
			Symbol:      "BTCUSDC",
			Market:      types.Market{Symbol: "BTCUSDC"},
			Side:        types.SideTypeBuy,
			Type:        types.OrderTypeLimit,
			Quantity:    quantity,
			Price:       price,
			TimeInForce: types.TimeInForceGTC,
		}

		createdOrder, err := ex.SubmitOrder(ctx, order)
		require.NoError(t, err)
		require.NotNil(t, createdOrder)

		assert.Equal(t, uint64(123456), createdOrder.OrderID)
		assert.Equal(t, types.OrderStatusNew, createdOrder.Status)
		assert.True(t, createdOrder.IsWorking)
		assert.True(t, createdOrder.ExecutedQuantity.IsZero())
	})

	t.Run("returns filled status when order is fully executed", func(t *testing.T) {
		ex, transport := newMockTestExchange(t, true)
		ctx := context.Background()

		quantity := fixedpoint.MustNewFromString("1.25")

		transport.POST("/exchange", func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)

			var payload struct {
				Action struct {
					Type   string           `json:"type"`
					Orders []hyperapi.Order `json:"orders"`
				} `json:"action"`
			}

			require.NoError(t, json.Unmarshal(body, &payload))
			require.Len(t, payload.Action.Orders, 1)

			submitted := payload.Action.Orders[0]
			assert.True(t, submitted.OrderType.Trigger.IsMarket)
			assert.Equal(t, "0", submitted.OrderType.Trigger.TriggerPx)

			return httptesting.BuildResponseJson(http.StatusOK, map[string]any{
				"status": "success",
				"response": map[string]any{
					"type": "order",
					"data": map[string]any{
						"statuses": []any{
							map[string]any{
								"filled": map[string]any{
									"oid":     888,
									"totalSz": quantity.String(),
									"avgPx":   "105000",
								},
							},
						},
					},
				},
			}), nil
		})

		order := types.SubmitOrder{
			Symbol:   "BTCUSDC",
			Market:   types.Market{Symbol: "BTCUSDC"},
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeMarket,
			Quantity: quantity,
			Price:    fixedpoint.Zero,
		}

		createdOrder, err := ex.SubmitOrder(ctx, order)
		require.NoError(t, err)
		require.NotNil(t, createdOrder)

		assert.Equal(t, uint64(888), createdOrder.OrderID)
		assert.Equal(t, types.OrderStatusFilled, createdOrder.Status)
		assert.Equal(t, quantity.String(), createdOrder.ExecutedQuantity.String())
	})

	t.Run("rejects invalid client order id", func(t *testing.T) {
		ex, _ := newMockTestExchange(t, true)
		ctx := context.Background()

		order := types.SubmitOrder{
			Symbol:        "BTCUSDC",
			Market:        types.Market{Symbol: "BTCUSDC"},
			Side:          types.SideTypeBuy,
			Type:          types.OrderTypeLimit,
			Quantity:      fixedpoint.MustNewFromString("0.1"),
			Price:         fixedpoint.MustNewFromString("100000"),
			ClientOrderID: "invalidID",
		}

		createdOrder, err := ex.SubmitOrder(ctx, order)
		require.Error(t, err)
		assert.Nil(t, createdOrder)
		assert.Contains(t, err.Error(), "client order id")
	})
}

func TestExchange_SubmitOrder(t *testing.T) {
	if b, _ := strconv.ParseBool(os.Getenv("CI")); b {
		t.Skip("skip test for CI")
	}

	privateKey, mainAccount, vaultAccount, ok := testutil.IntegrationTestWithPrivateKeyConfigured(t, "HYPERLIQUID")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()
	ex := New(privateKey, mainAccount, vaultAccount)
	hyperapi.TestNet = true
	ex.IsFutures = true

	account, err := ex.QueryAccount(ctx)
	require.NoError(t, err)
	require.NotNil(t, account)

	t.Logf("account: %+v", account)

	markets, err := ex.QueryMarkets(ctx)
	assert.NoError(t, err)

	market, ok := markets["BTCUSDC"]
	if assert.True(t, ok) {
		t.Logf("market: %+v", market)
		createdOrder, err := ex.SubmitOrder(ctx, types.SubmitOrder{
			Symbol:   "BTCUSDC",
			Type:     types.OrderTypeLimit,
			Price:    Number(84925),
			Quantity: fixedpoint.MustNewFromString("0.001"),
			Side:     types.SideTypeBuy,
			Market:   market,
		})
		if assert.NoError(t, err) {
			t.Error(err.Error())
			t.Logf("createdOrder: %+v", createdOrder)
		}
	}
}
