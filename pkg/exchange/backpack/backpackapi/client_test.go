package backpackapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
	"github.com/c9s/bbgo/pkg/testutil"
)

// testSymbol is the market used by the market data integration tests. It is a liquid spot
// market with a small tick size, which keeps the recorded responses stable.
const testSymbol = "SOL_USDC"

// testOrderSymbol is the market used by the order life cycle test.
//
// Every Backpack market is quoted in USDC, so selling the base asset of USDT_USDC is the way
// to exercise the order endpoints with a USDT balance.
const testOrderSymbol = "USDT_USDC"

// setupTestClient wires up the http record/replay transport and the api credentials.
//
// The recorder is installed before the credentials are checked so that replaying a recording
// never skips. When the credentials are absent (the replay case) the client is authenticated
// with a throwaway key pair, so that the signing path is still exercised: without it the
// authenticated requests would fail before ever reaching the mock transport.
func setupTestClient(t *testing.T) (client *RestClient, isRecording bool, saveRecord func()) {
	client = NewClient()

	isRecording, saveRecord = httptesting.RunHttpTestWithRecorder(t, client.HttpClient, "testdata/"+t.Name()+".json")

	key, secret, ok := testutil.IntegrationTestConfigured(t, "BACKPACK")
	if !ok {
		key, secret = newTestKeyPair(0x7f)
	}

	if err := client.Auth(key, secret); err != nil {
		saveRecord()
		t.Fatalf("unable to authenticate the test client: %v", err)
	}

	if isRecording && !ok {
		saveRecord()
		t.Skipf("BACKPACK api key is not configured, skipping integration test")
	}

	return client, isRecording, saveRecord
}

func TestClient_publicMarketDataApis(t *testing.T) {
	// You can enable recording for updating the test data
	// httptesting.AlwaysRecord = true

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, _, saveRecord := setupTestClient(t)
	defer saveRecord()

	t.Run("GetStatusRequest", func(t *testing.T) {
		status, err := client.NewGetStatusRequest().Do(ctx)
		if assert.NoError(t, err) && assert.NotNil(t, status) {
			assert.NotEmpty(t, status.Status)
			t.Logf("status: %+v", status)
		}
	})

	t.Run("Ping", func(t *testing.T) {
		assert.NoError(t, client.Ping(ctx))
	})

	t.Run("GetServerTime", func(t *testing.T) {
		serverTime, err := client.GetServerTime(ctx)
		if assert.NoError(t, err) {
			assert.False(t, serverTime.IsZero())
			t.Logf("server time: %s", serverTime)
		}
	})

	t.Run("GetMarketsRequest", func(t *testing.T) {
		markets, err := client.NewGetMarketsRequest().Do(ctx)
		if assert.NoError(t, err) && assert.NotEmpty(t, markets) {
			for _, market := range markets {
				assert.NotEmpty(t, market.Symbol)
				assert.NotEmpty(t, market.MarketType)
			}

			t.Logf("fetched %d markets, first: %+v", len(markets), markets[0])
		}
	})

	t.Run("GetMarketRequest", func(t *testing.T) {
		req := client.NewGetMarketRequest()
		req.Symbol(testSymbol)

		market, err := req.Do(ctx)
		if assert.NoError(t, err) && assert.NotNil(t, market) {
			assert.Equal(t, testSymbol, market.Symbol)
			assert.True(t, market.Filters.Price.TickSize.Sign() > 0)
			assert.True(t, market.Filters.Quantity.StepSize.Sign() > 0)
			t.Logf("market: %+v", market)
		}
	})

	t.Run("GetTickersRequest", func(t *testing.T) {
		tickers, err := client.NewGetTickersRequest().Do(ctx)
		if assert.NoError(t, err) && assert.NotEmpty(t, tickers) {
			t.Logf("fetched %d tickers, first: %+v", len(tickers), tickers[0])
		}
	})

	t.Run("GetTickerRequest", func(t *testing.T) {
		req := client.NewGetTickerRequest()
		req.Symbol(testSymbol)

		ticker, err := req.Do(ctx)
		if assert.NoError(t, err) && assert.NotNil(t, ticker) {
			assert.Equal(t, testSymbol, ticker.Symbol)
			assert.True(t, ticker.LastPrice.Sign() > 0)
			t.Logf("ticker: %+v", ticker)
		}
	})

	t.Run("GetDepthRequest", func(t *testing.T) {
		req := client.NewGetDepthRequest()
		req.Symbol(testSymbol).Limit(DepthLimit20)

		depth, err := req.Do(ctx)
		if assert.NoError(t, err) && assert.NotNil(t, depth) {
			assert.NotEmpty(t, depth.Asks)
			assert.NotEmpty(t, depth.Bids)
			assert.False(t, depth.Timestamp.Time().IsZero())
			t.Logf("depth: bids=%d asks=%d lastUpdateId=%d best bid=%s best ask=%s",
				len(depth.Bids), len(depth.Asks), int64(depth.LastUpdateId),
				depth.Bids[len(depth.Bids)-1], depth.Asks[0])
		}
	})

	t.Run("GetKlinesRequest", func(t *testing.T) {
		req := client.NewGetKlinesRequest()
		req.Symbol(testSymbol).
			Interval(KlineInterval1m).
			StartTime(time.Now().Add(-1 * time.Hour))

		klines, err := req.Do(ctx)
		if assert.NoError(t, err) && assert.NotEmpty(t, klines) {
			assert.False(t, klines[0].Start.Time().IsZero())
			t.Logf("fetched %d klines, first: %+v", len(klines), klines[0])
		}
	})

	t.Run("GetTradesRequest", func(t *testing.T) {
		req := client.NewGetTradesRequest()
		req.Symbol(testSymbol).Limit(10)

		trades, err := req.Do(ctx)
		if assert.NoError(t, err) && assert.NotEmpty(t, trades) {
			assert.True(t, trades[0].Price.Sign() > 0)
			t.Logf("fetched %d trades, first: %+v", len(trades), trades[0])
		}
	})

	t.Run("GetMarkPricesRequest", func(t *testing.T) {
		markPrices, err := client.NewGetMarkPricesRequest().Do(ctx)
		if assert.NoError(t, err) && assert.NotEmpty(t, markPrices) {
			t.Logf("fetched %d mark prices, first: %+v", len(markPrices), markPrices[0])
		}
	})

	t.Run("GetOpenInterestRequest", func(t *testing.T) {
		openInterests, err := client.NewGetOpenInterestRequest().Do(ctx)
		if assert.NoError(t, err) {
			t.Logf("fetched %d open interests", len(openInterests))
		}
	})
}

func TestClient_accountApis(t *testing.T) {
	// You can enable recording for updating the test data
	// httptesting.AlwaysRecord = true

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, _, saveRecord := setupTestClient(t)
	defer saveRecord()

	t.Run("GetBalancesRequest", func(t *testing.T) {
		balances, err := client.NewGetBalancesRequest().Do(ctx)
		if assert.NoError(t, err) {
			for currency, balance := range balances {
				assert.NotEmpty(t, currency)
				assert.False(t, balance.Total().Sign() < 0, "a balance should never be negative")
			}

			t.Logf("balances: %+v", balances)
		}
	})

	t.Run("GetCollateralRequest", func(t *testing.T) {
		summary, err := client.NewGetCollateralRequest().Do(ctx)
		if assert.NoError(t, err) && assert.NotNil(t, summary) {
			t.Logf("collateral summary: netEquity=%s assets=%s collateral entries=%d",
				summary.NetEquity.String(), summary.AssetsValue.String(), len(summary.Collateral))
		}
	})

	t.Run("GetPositionsRequest", func(t *testing.T) {
		positions, err := client.NewGetPositionsRequest().Do(ctx)
		if assert.NoError(t, err) {
			t.Logf("fetched %d positions", len(positions))
		}
	})
}

func TestClient_historyApis(t *testing.T) {
	// You can enable recording for updating the test data
	// httptesting.AlwaysRecord = true

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, _, saveRecord := setupTestClient(t)
	defer saveRecord()

	t.Run("GetOrderHistoryRequest", func(t *testing.T) {
		req := client.NewGetOrderHistoryRequest()
		req.Symbol(testOrderSymbol).Limit(10)

		orders, err := req.Do(ctx)
		if assert.NoError(t, err) {
			for _, order := range orders {
				assert.NotEmpty(t, order.Id)
				assert.False(t, order.CreatedAt.Time().IsZero())
			}

			t.Logf("fetched %d historical orders", len(orders))
		}
	})

	t.Run("GetFillHistoryRequest", func(t *testing.T) {
		req := client.NewGetFillHistoryRequest()
		req.Symbol(testOrderSymbol).
			FillType(FillTypeUser).
			Limit(10)

		fills, err := req.Do(ctx)
		if assert.NoError(t, err) {
			for _, fill := range fills {
				assert.NotEmpty(t, fill.OrderId)
				assert.False(t, fill.Timestamp.Time().IsZero())
			}

			t.Logf("fetched %d fills", len(fills))
		}
	})
}

// TestClient_orderApis places a post-only limit order that can not fill, queries it and
// cancels it again.
//
// The order sells the base asset of testOrderSymbol, which is what the funded test account
// holds. It rests above the market but inside the market's mean mark price band, and being
// post-only it is also exempt from the taker speed bump, so the recording stays reproducible.
func TestClient_orderApis(t *testing.T) {
	// You can enable recording for updating the test data
	// httptesting.AlwaysRecord = true

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, isRecording, saveRecord := setupTestClient(t)
	defer saveRecord()

	market, err := client.NewGetMarketRequest().Symbol(testOrderSymbol).Do(ctx)
	if !assert.NoError(t, err) || !assert.NotNil(t, market) {
		return
	}

	ticker, err := client.NewGetTickerRequest().Symbol(testOrderSymbol).Do(ctx)
	if !assert.NoError(t, err) || !assert.NotNil(t, ticker) {
		return
	}

	price := restingAskPrice(market, ticker.LastPrice)
	quantity := fixedpoint.NewFromInt(20).Round(quantityScale(market), fixedpoint.Down)

	t.Logf("submitting a post-only %s ask: price=%s quantity=%s (last=%s)",
		testOrderSymbol, price, quantity, ticker.LastPrice)

	var placedOrder *Order

	t.Run("PlaceOrderRequest", func(t *testing.T) {
		req := client.NewPlaceOrderRequest()
		req.Symbol(testOrderSymbol).
			Side(SideAsk).
			OrderType(OrderTypeLimit).
			TimeInForce(TimeInForceGTC).
			PostOnly(true).
			Price(price.String()).
			Quantity(quantity.String())

		order, err := req.Do(ctx)

		// an unfunded test account still proves that the request was well formed and correctly
		// signed: a bad signature would come back as INVALID_SIGNATURE instead.
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Code == ApiErrorCodeInsufficientFunds {
			t.Skipf("the test account has insufficient funds to place an order (%s); "+
				"fund it and re-record this test to cover the order life cycle", apiErr.Message)
			return
		}

		if assert.NoError(t, err) && assert.NotNil(t, order) {
			assert.NotEmpty(t, order.Id)
			assert.Equal(t, testOrderSymbol, order.Symbol)
			assert.Equal(t, SideAsk, order.Side)
			assert.Equal(t, OrderTypeLimit, order.OrderType)
			assert.True(t, order.Status.IsWorking(), "a post-only order away from the market should rest")

			placedOrder = order
			t.Logf("placed order: %+v", order)
		}
	})

	if placedOrder == nil {
		t.Skip("order placement failed, skipping the dependent subtests")
	}

	t.Run("GetOrderRequest", func(t *testing.T) {
		req := client.NewGetOrderRequest()
		req.Symbol(testOrderSymbol).OrderId(placedOrder.Id)

		order, err := req.Do(ctx)
		if assert.NoError(t, err) && assert.NotNil(t, order) {
			assert.Equal(t, placedOrder.Id, order.Id)
			t.Logf("queried order: %+v", order)
		}
	})

	t.Run("GetOpenOrdersRequest", func(t *testing.T) {
		req := client.NewGetOpenOrdersRequest()
		req.Symbol(testOrderSymbol)

		orders, err := req.Do(ctx)
		if assert.NoError(t, err) {
			var found bool
			for _, order := range orders {
				if order.Id == placedOrder.Id {
					found = true
					break
				}
			}

			assert.True(t, found, "the placed order should be listed in the open orders")
			t.Logf("fetched %d open orders", len(orders))
		}
	})

	t.Run("CancelOrderRequest", func(t *testing.T) {
		req := client.NewCancelOrderRequest()
		req.Symbol(testOrderSymbol).OrderId(placedOrder.Id)

		order, err := req.Do(ctx)
		if assert.NoError(t, err) && assert.NotNil(t, order) {
			assert.Equal(t, placedOrder.Id, order.Id)
			t.Logf("cancelled order: %+v", order)
		}
	})

	t.Run("CancelAllOrdersRequest", func(t *testing.T) {
		if isRecording {
			// give the matching engine a moment to settle the cancellation above
			time.Sleep(500 * time.Millisecond)
		}

		req := client.NewCancelAllOrdersRequest()
		req.Symbol(testOrderSymbol)

		orders, err := req.Do(ctx)
		if assert.NoError(t, err) {
			t.Logf("cancelled %d remaining orders", len(orders))
		}
	})
}

// restingAskPrice returns a sell price that is above the market but still inside the market's
// mean mark price band, so that the order rests instead of being rejected or filled.
func restingAskPrice(market *Market, lastPrice fixedpoint.Value) fixedpoint.Value {
	multiplier := fixedpoint.NewFromFloat(1.008)

	if band := market.Filters.Price.MeanMarkPriceBand; band != nil && band.MaxMultiplier.Sign() > 0 {
		// stay just inside the upper bound of the band
		capped := band.MaxMultiplier.Mul(fixedpoint.NewFromFloat(0.998))
		if capped.Compare(multiplier) < 0 {
			multiplier = capped
		}
	}

	return lastPrice.Mul(multiplier).Round(priceScale(market), fixedpoint.Down)
}

func priceScale(market *Market) int {
	return market.Filters.Price.TickSize.NumFractionalDigits()
}

func quantityScale(market *Market) int {
	return market.Filters.Quantity.StepSize.NumFractionalDigits()
}
