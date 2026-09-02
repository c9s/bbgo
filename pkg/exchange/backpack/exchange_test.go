package backpack

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/backpack/backpackapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
)

// testSymbol is the market used by the market data tests: a liquid spot market with a small
// tick size, which keeps the recorded responses stable.
const testSymbol = "SOLUSDC"

// testOrderSymbol is the market used by the order life cycle test.
//
// Every Backpack market is quoted in USDC, so selling the base asset of USDT_USDC is the way to
// exercise the order endpoints with a USDT balance.
const testOrderSymbol = "USDTUSDC"

// newTestKeyPair returns a deterministic ed25519 key pair encoded the way the Backpack console
// issues credentials.
func newTestKeyPair() (key, secret string) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = 0x7f
	}

	privateKey := ed25519.NewKeyFromSeed(seed)
	return base64.StdEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey)),
		base64.StdEncoding.EncodeToString(seed)
}

// setupTestExchange wires up the http record/replay transport and the api credentials.
//
// The recorder is installed before the credentials are checked so that replaying a recording
// never skips. Without credentials the exchange is built with a throwaway key pair so that the
// signing path is still exercised, otherwise the authenticated requests would fail before ever
// reaching the mock transport.
func setupTestExchange(t *testing.T) (ex *Exchange, isRecording bool, saveRecord func()) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "BACKPACK")
	if !ok {
		key, secret = newTestKeyPair()
	}

	ex, err := New(key, secret)
	if err != nil {
		t.Fatalf("unable to create the test exchange: %v", err)
	}

	isRecording, saveRecord = httptesting.RunHttpTestWithRecorder(
		t, ex.client.HttpClient, "testdata/"+t.Name()+".json")

	if isRecording && !ok {
		saveRecord()
		t.Skipf("BACKPACK api key is not configured, skipping integration test")
	}

	return ex, isRecording, saveRecord
}

func TestExchange_basics(t *testing.T) {
	ex, err := New("", "")
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, types.ExchangeBackpack, ex.Name())
	assert.Equal(t, PlatformToken, ex.PlatformFeeCurrency())

	// the websocket stream is not implemented yet, but NewStream still has to return a usable
	// value because bbgo.NewExchangeSession binds a connectivity watcher to it
	stream := ex.NewStream()
	if assert.NotNil(t, stream) {
		assert.ErrorIs(t, stream.Connect(context.Background()), ErrStreamNotImplemented)
	}
}

func TestExchange_getLocalSymbol(t *testing.T) {
	clearSymbolMaps(t)
	defer clearSymbolMaps(t)

	ex, err := New("", "")
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "SOL_USDC", ex.getLocalSymbol("SOLUSDC"))

	// the futures settings switch the lookup to the perpetual market
	ex.UseFutures()
	assert.Equal(t, "SOL_USDC_PERP", ex.getLocalSymbol("SOLUSDC"))
}

func TestExchange_marketDataApis(t *testing.T) {
	// You can enable recording for updating the test data
	// httptesting.AlwaysRecord = true

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clearSymbolMaps(t)
	defer clearSymbolMaps(t)

	ex, _, saveRecord := setupTestExchange(t)
	defer saveRecord()

	t.Run("QueryMarkets", func(t *testing.T) {
		markets, err := ex.QueryMarkets(ctx)
		if assert.NoError(t, err) && assert.NotEmpty(t, markets) {
			market, ok := markets[testSymbol]
			if assert.True(t, ok, "%s should be listed", testSymbol) {
				assert.Equal(t, "SOL_USDC", market.LocalSymbol)
				assert.Equal(t, "SOL", market.BaseCurrency)
				assert.Equal(t, "USDC", market.QuoteCurrency)
				assert.True(t, market.TickSize.Sign() > 0)
				assert.True(t, market.StepSize.Sign() > 0)
			}

			// only spot markets are returned for now
			for symbol := range markets {
				assert.NotContains(t, symbol, "PERP")
			}

			t.Logf("fetched %d markets", len(markets))
		}
	})

	t.Run("QueryTicker", func(t *testing.T) {
		ticker, err := ex.QueryTicker(ctx, testSymbol)
		if assert.NoError(t, err) && assert.NotNil(t, ticker) {
			assert.True(t, ticker.Last.Sign() > 0)
			// Buy/Sell are filled from the order book, the ticker endpoint has no bid/ask
			assert.True(t, ticker.Buy.Sign() > 0, "the best bid should be filled from the depth")
			assert.True(t, ticker.Sell.Sign() > 0, "the best ask should be filled from the depth")
			assert.True(t, ticker.Buy.Compare(ticker.Sell) < 0, "the bid should be below the ask")
			t.Logf("ticker: %+v", ticker)
		}
	})

	t.Run("QueryTickers", func(t *testing.T) {
		tickers, err := ex.QueryTickers(ctx)
		if assert.NoError(t, err) && assert.NotEmpty(t, tickers) {
			_, ok := tickers[testSymbol]
			assert.True(t, ok, "%s should be present", testSymbol)
			t.Logf("fetched %d tickers", len(tickers))
		}
	})

	t.Run("QueryTickers with a symbol filter", func(t *testing.T) {
		tickers, err := ex.QueryTickers(ctx, testSymbol)
		if assert.NoError(t, err) {
			assert.Len(t, tickers, 1)
			_, ok := tickers[testSymbol]
			assert.True(t, ok)
		}
	})

	t.Run("QueryKLines", func(t *testing.T) {
		kLines, err := ex.QueryKLines(ctx, testSymbol, types.Interval1m, types.KLineQueryOptions{
			Limit: 10,
		})

		if assert.NoError(t, err) && assert.NotEmpty(t, kLines) {
			assert.LessOrEqual(t, len(kLines), 10, "the limit is applied client side")

			k := kLines[0]
			assert.Equal(t, testSymbol, k.Symbol)
			assert.Equal(t, types.Interval1m, k.Interval)
			assert.Equal(t, types.ExchangeBackpack, k.Exchange)
			assert.Equal(t,
				k.StartTime.Time().Add(time.Minute-time.Millisecond),
				k.EndTime.Time())

			t.Logf("fetched %d klines, last: %+v", len(kLines), kLines[len(kLines)-1])
		}
	})

	t.Run("QueryKLines with an unsupported interval", func(t *testing.T) {
		// bbgo has 2w, Backpack does not
		_, err := ex.QueryKLines(ctx, testSymbol, types.Interval2w, types.KLineQueryOptions{})
		assert.Error(t, err)
	})
}

func TestExchange_accountApis(t *testing.T) {
	// You can enable recording for updating the test data
	// httptesting.AlwaysRecord = true

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ex, _, saveRecord := setupTestExchange(t)
	defer saveRecord()

	t.Run("QueryAccountBalances", func(t *testing.T) {
		balances, err := ex.QueryAccountBalances(ctx)
		if assert.NoError(t, err) {
			for currency, balance := range balances {
				assert.NotEmpty(t, currency)
				assert.False(t, balance.Available.Sign() < 0)
			}

			t.Logf("balances: %+v", balances)
		}
	})

	t.Run("QueryAccount", func(t *testing.T) {
		account, err := ex.QueryAccount(ctx)
		if assert.NoError(t, err) && assert.NotNil(t, account) {
			t.Logf("account balances: %+v", account.Balances())
		}
	})
}

func TestExchange_historyApis(t *testing.T) {
	// You can enable recording for updating the test data
	// httptesting.AlwaysRecord = true

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ex, _, saveRecord := setupTestExchange(t)
	defer saveRecord()

	t.Run("QueryClosedOrders", func(t *testing.T) {
		orders, err := ex.QueryClosedOrders(ctx, testOrderSymbol, time.Time{}, time.Time{}, 0)
		if assert.NoError(t, err) {
			for _, order := range orders {
				assert.Equal(t, testOrderSymbol, order.Symbol)
				assert.False(t, order.IsWorking, "a closed order should not be working")
				assert.NotZero(t, order.OrderID)
				assert.NotEmpty(t, order.UUID)
				assert.False(t, order.CreationTime.Time().IsZero())
			}

			t.Logf("fetched %d closed orders", len(orders))
		}
	})

	t.Run("QueryTrades", func(t *testing.T) {
		trades, err := ex.QueryTrades(ctx, testOrderSymbol, &types.TradeQueryOptions{Limit: 10})
		if assert.NoError(t, err) {
			for _, trade := range trades {
				assert.Equal(t, testOrderSymbol, trade.Symbol)
				assert.NotZero(t, trade.OrderID)
			}

			t.Logf("fetched %d trades", len(trades))
		}
	})
}

// TestExchange_orderApis exercises the full order life cycle through the types.Exchange API.
//
// The order rests above the market but inside the market's mean mark price band and is
// post-only, so it cannot fill and the recording stays reproducible.
func TestExchange_orderApis(t *testing.T) {
	// You can enable recording for updating the test data
	// httptesting.AlwaysRecord = true

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clearSymbolMaps(t)
	defer clearSymbolMaps(t)

	ex, _, saveRecord := setupTestExchange(t)
	defer saveRecord()

	markets, err := ex.QueryMarkets(ctx)
	if !assert.NoError(t, err) {
		return
	}

	market, ok := markets[testOrderSymbol]
	if !assert.True(t, ok, "%s should be listed", testOrderSymbol) {
		return
	}

	ticker, err := ex.QueryTicker(ctx, testOrderSymbol)
	if !assert.NoError(t, err) || !assert.NotNil(t, ticker) {
		return
	}

	// stay within 1% of the mark price, which is the band USDT_USDC enforces
	price := ticker.Last.Mul(fixedpoint.NewFromFloat(1.008)).Round(market.PricePrecision, fixedpoint.Down)
	quantity := fixedpoint.NewFromInt(20)

	t.Logf("submitting a post-only %s sell: price=%s quantity=%s", testOrderSymbol, price, quantity)

	var placedOrder *types.Order

	t.Run("SubmitOrder", func(t *testing.T) {
		createdOrder, err := ex.SubmitOrder(ctx, types.SubmitOrder{
			Symbol:      testOrderSymbol,
			Side:        types.SideTypeSell,
			Type:        types.OrderTypeLimitMaker,
			Quantity:    quantity,
			Price:       price,
			TimeInForce: types.TimeInForceGTC,
			Market:      market,
		})

		// an unfunded account still proves the request was well formed and correctly signed
		var apiErr *backpackapi.APIError
		if errors.As(err, &apiErr) && apiErr.Code == backpackapi.ApiErrorCodeInsufficientFunds {
			t.Skipf("the test account has insufficient funds (%s); fund it and re-record", apiErr.Message)
			return
		}

		if assert.NoError(t, err) && assert.NotNil(t, createdOrder) {
			assert.Equal(t, testOrderSymbol, createdOrder.Symbol)
			assert.Equal(t, types.SideTypeSell, createdOrder.Side)
			assert.Equal(t, types.OrderTypeLimitMaker, createdOrder.Type)
			assert.Equal(t, types.OrderStatusNew, createdOrder.Status)
			assert.True(t, createdOrder.IsWorking)
			assert.NotZero(t, createdOrder.OrderID)
			assert.NotEmpty(t, createdOrder.UUID)

			placedOrder = createdOrder
			t.Logf("placed order: %+v", createdOrder)
		}
	})

	if placedOrder == nil {
		t.Skip("order placement failed, skipping the dependent subtests")
	}

	t.Run("QueryOrder", func(t *testing.T) {
		order, err := ex.QueryOrder(ctx, types.OrderQuery{
			Symbol:    testOrderSymbol,
			OrderUUID: placedOrder.UUID,
		})

		if assert.NoError(t, err) && assert.NotNil(t, order) {
			assert.Equal(t, placedOrder.UUID, order.UUID)
			assert.Equal(t, placedOrder.OrderID, order.OrderID)
		}
	})

	t.Run("QueryOpenOrders", func(t *testing.T) {
		orders, err := ex.QueryOpenOrders(ctx, testOrderSymbol)
		if assert.NoError(t, err) {
			var found bool
			for _, order := range orders {
				if order.UUID == placedOrder.UUID {
					found = true
					break
				}
			}

			assert.True(t, found, "the placed order should be listed in the open orders")
		}
	})

	t.Run("QueryOrderTrades", func(t *testing.T) {
		// the order cannot have filled, so this only checks that the call succeeds
		trades, err := ex.QueryOrderTrades(ctx, types.OrderQuery{
			Symbol:    testOrderSymbol,
			OrderUUID: placedOrder.UUID,
		})

		if assert.NoError(t, err) {
			t.Logf("fetched %d trades for the order", len(trades))
		}
	})

	t.Run("CancelOrders", func(t *testing.T) {
		assert.NoError(t, ex.CancelOrders(ctx, *placedOrder))
	})

	t.Run("CancelOrders without an identifier", func(t *testing.T) {
		// the numeric OrderID alone is not enough: it may be a hash of the UUID
		err := ex.CancelOrders(ctx, types.Order{
			SubmitOrder: types.SubmitOrder{Symbol: testOrderSymbol},
			OrderID:     12345,
		})

		assert.ErrorContains(t, err, "UUID")
	})
}
