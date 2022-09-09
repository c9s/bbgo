package ftx

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func integrationTestConfigured() (key, secret string, ok bool) {
	var hasKey, hasSecret bool
	key, hasKey = os.LookupEnv("FTX_API_KEY")
	secret, hasSecret = os.LookupEnv("FTX_API_SECRET")
	ok = hasKey && hasSecret && os.Getenv("TEST_FTX") == "1"
	return key, secret, ok
}

func TestExchange_IOCOrder(t *testing.T) {
	key, secret, ok := integrationTestConfigured()
	if !ok {
		t.SkipNow()
		return
	}

	ex := NewExchange(key, secret, "")
	createdOrder, err := ex.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:   "LTCUSDT",
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeLimitMaker,
		Quantity: fixedpoint.NewFromFloat(1.0),
		Price:    fixedpoint.NewFromFloat(50.0),
		Market: types.Market{
			Symbol:          "LTCUSDT",
			LocalSymbol:     "LTC/USDT",
			PricePrecision:  3,
			VolumePrecision: 2,
			QuoteCurrency:   "USDT",
			BaseCurrency:    "LTC",
			MinQuantity:     fixedpoint.NewFromFloat(0.01),
			StepSize:        fixedpoint.NewFromFloat(0.01),
			TickSize:        fixedpoint.NewFromFloat(0.01),
		},
		TimeInForce: "IOC",
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, createdOrder)
	t.Logf("created orders: %+v", createdOrder)
}

func TestExchange_QueryAccountBalances(t *testing.T) {
	successResp := `
{
  "result": [
    {
      "availableWithoutBorrow": 19.47458865,
      "coin": "USD",
      "free": 19.48085209,
      "spotBorrow": 0.0,
      "total": 1094.66405065,
      "usdValue": 1094.664050651561
    }
  ],
  "success": true
}
`
	failureResp := `{"result":[],"success":false}`
	i := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if i == 0 {
			fmt.Fprintln(w, successResp)
			i++
			return
		}
		fmt.Fprintln(w, failureResp)
	}))
	defer ts.Close()

	ex := NewExchange("test-key", "test-secret", "")
	serverURL, err := url.Parse(ts.URL)
	assert.NoError(t, err)
	ex.client.BaseURL = serverURL

	resp, err := ex.QueryAccountBalances(context.Background())
	assert.NoError(t, err)

	assert.Len(t, resp, 1)
	b, ok := resp["USD"]
	assert.True(t, ok)
	expectedAvailable := fixedpoint.Must(fixedpoint.NewFromString("19.48085209"))
	assert.Equal(t, expectedAvailable, b.Available)
	assert.Equal(t, fixedpoint.Must(fixedpoint.NewFromString("1094.66405065")).Sub(expectedAvailable), b.Locked)
}

func TestExchange_QueryOpenOrders(t *testing.T) {
	successResp := `
{
  "success": true,
  "result": [
    {
      "createdAt": "2019-03-05T09:56:55.728933+00:00",
      "filledSize": 10,
      "future": "XRP-PERP",
      "id": 9596912,
      "market": "XRP-PERP",
      "price": 0.306525,
      "avgFillPrice": 0.306526,
      "remainingSize": 31421,
      "side": "sell",
      "size": 31431,
      "status": "open",
      "type": "limit",
      "reduceOnly": false,
      "ioc": false,
      "postOnly": false,
      "clientId": null
    }
  ]
}
`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, successResp)
	}))
	defer ts.Close()

	ex := NewExchange("test-key", "test-secret", "")

	serverURL, err := url.Parse(ts.URL)
	assert.NoError(t, err)
	ex.client.BaseURL = serverURL

	resp, err := ex.QueryOpenOrders(context.Background(), "XRP-PREP")
	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, "XRP-PERP", resp[0].Symbol)
}

func TestExchange_QueryClosedOrders(t *testing.T) {
	t.Run("no closed orders", func(t *testing.T) {
		successResp := `{"success": true, "result": []}`

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, successResp)
		}))
		defer ts.Close()

		ex := NewExchange("test-key", "test-secret", "")
		serverURL, err := url.Parse(ts.URL)
		assert.NoError(t, err)
		ex.client.BaseURL = serverURL

		resp, err := ex.QueryClosedOrders(context.Background(), "BTC-PERP", time.Now(), time.Now(), 100)
		assert.NoError(t, err)

		assert.Len(t, resp, 0)
	})
	t.Run("one closed order", func(t *testing.T) {
		successResp := `
{
  "success": true,
  "result": [
    {
      "avgFillPrice": 10135.25,
      "clientId": null,
      "createdAt": "2019-06-27T15:24:03.101197+00:00",
      "filledSize": 0.001,
      "future": "BTC-PERP",
      "id": 257132591,
      "ioc": false,
      "market": "BTC-PERP",
      "postOnly": false,
      "price": 10135.25,
      "reduceOnly": false,
      "remainingSize": 0.0,
      "side": "buy",
      "size": 0.001,
      "status": "closed",
      "type": "limit"
    }
  ],
  "hasMoreData": false
}
`
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, successResp)
		}))
		defer ts.Close()

		ex := NewExchange("test-key", "test-secret", "")
		serverURL, err := url.Parse(ts.URL)
		assert.NoError(t, err)
		ex.client.BaseURL = serverURL

		resp, err := ex.QueryClosedOrders(context.Background(), "BTC-PERP", time.Now(), time.Now(), 100)
		assert.NoError(t, err)
		assert.Len(t, resp, 1)
		assert.Equal(t, "BTC-PERP", resp[0].Symbol)
	})

	t.Run("sort the order", func(t *testing.T) {
		successResp := `
{
  "success": true,
  "result": [
    {
			"status": "closed",
      "createdAt": "2020-09-01T15:24:03.101197+00:00",
      "id": 789
    },
    {
			"status": "closed",
      "createdAt": "2019-03-27T15:24:03.101197+00:00",
      "id": 123
    },
    {
			"status": "closed",
      "createdAt": "2019-06-27T15:24:03.101197+00:00",
      "id": 456
    },
    {
			"status": "new",
      "createdAt": "2019-06-27T15:24:03.101197+00:00",
      "id": 999
    }
  ],
  "hasMoreData": false
}
`
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, successResp)
		}))
		defer ts.Close()

		ex := NewExchange("test-key", "test-secret", "")
		serverURL, err := url.Parse(ts.URL)
		assert.NoError(t, err)
		ex.client.BaseURL = serverURL

		resp, err := ex.QueryClosedOrders(context.Background(), "BTC-PERP", time.Now(), time.Now(), 100)
		assert.NoError(t, err)
		assert.Len(t, resp, 3)

		expectedOrderID := []uint64{123, 456, 789}
		for i, o := range resp {
			assert.Equal(t, expectedOrderID[i], o.OrderID)
		}
	})
}

func TestExchange_QueryAccount(t *testing.T) {
	balanceResp := `
{
  "result": [
    {
      "availableWithoutBorrow": 19.47458865,
      "coin": "USD",
      "free": 19.48085209,
      "spotBorrow": 0.0,
      "total": 1094.66405065,
      "usdValue": 1094.664050651561
    }
  ],
  "success": true
}
`

	accountInfoResp := `
{
  "success": true,
  "result": {
    "backstopProvider": true,
    "collateral": 3568181.02691129,
    "freeCollateral": 1786071.456884368,
    "initialMarginRequirement": 0.12222384240257728,
    "leverage": 10,
    "liquidating": false,
    "maintenanceMarginRequirement": 0.07177992558058484,
    "makerFee": 0.0002,
    "marginFraction": 0.5588433331419503,
    "openMarginFraction": 0.2447194090423075,
    "takerFee": 0.0005,
    "totalAccountValue": 3568180.98341129,
    "totalPositionSize": 6384939.6992,
    "username": "user@domain.com",
    "positions": [
      {
        "cost": -31.7906,
        "entryPrice": 138.22,
        "future": "ETH-PERP",
        "initialMarginRequirement": 0.1,
        "longOrderSize": 1744.55,
        "maintenanceMarginRequirement": 0.04,
        "netSize": -0.23,
        "openSize": 1744.32,
        "realizedPnl": 3.39441714,
        "shortOrderSize": 1732.09,
        "side": "sell",
        "size": 0.23,
        "unrealizedPnl": 0
      }
    ]
  }
}
`
	returnBalance := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if returnBalance {
			fmt.Fprintln(w, balanceResp)
			return
		}
		returnBalance = true
		fmt.Fprintln(w, accountInfoResp)
	}))
	defer ts.Close()

	ex := NewExchange("test-key", "test-secret", "")
	serverURL, err := url.Parse(ts.URL)
	assert.NoError(t, err)
	ex.client.BaseURL = serverURL

	resp, err := ex.QueryAccount(context.Background())
	assert.NoError(t, err)

	b, ok := resp.Balance("USD")
	assert.True(t, ok)
	expected := types.Balance{
		Currency:  "USD",
		Available: fixedpoint.MustNewFromString("19.48085209"),
		Locked:    fixedpoint.MustNewFromString("1094.66405065"),
	}
	expected.Locked = expected.Locked.Sub(expected.Available)
	assert.Equal(t, expected, b)
}

func TestExchange_QueryMarkets(t *testing.T) {
	respJSON := `{
"success": true,
"result": [
  {
    "name": "BTC/USD",
    "enabled": true,
    "postOnly": false,
    "priceIncrement": 1.0,
    "sizeIncrement": 0.0001,
    "minProvideSize": 0.001,
    "last": 59039.0,
    "bid": 59038.0,
    "ask": 59040.0,
    "price": 59039.0,
    "type": "spot",
    "baseCurrency": "BTC",
    "quoteCurrency": "USD",
    "underlying": null,
    "restricted": false,
    "highLeverageFeeExempt": true,
    "change1h": 0.0015777151969599294,
    "change24h": 0.05475756601279165,
    "changeBod": -0.0035107262814994852,
    "quoteVolume24h": 316493675.5463,
    "volumeUsd24h": 316493675.5463
  }
]
}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, respJSON)
	}))
	defer ts.Close()

	ex := NewExchange("test-key", "test-secret", "")
	serverURL, err := url.Parse(ts.URL)
	assert.NoError(t, err)
	ex.client.BaseURL = serverURL
	ex.restEndpoint = serverURL

	resp, err := ex.QueryMarkets(context.Background())
	assert.NoError(t, err)

	assert.Len(t, resp, 1)
	assert.Equal(t, types.Market{
		Symbol:          "BTCUSD",
		LocalSymbol:     "BTC/USD",
		PricePrecision:  0,
		VolumePrecision: 4,
		QuoteCurrency:   "USD",
		BaseCurrency:    "BTC",
		MinQuantity:     fixedpoint.NewFromFloat(0.001),
		StepSize:        fixedpoint.NewFromFloat(0.0001),
		TickSize:        fixedpoint.NewFromInt(1),
	}, resp["BTCUSD"])
}

func TestExchange_QueryDepositHistory(t *testing.T) {
	respJSON := `
{
  "success": true,
  "result": [
    {
      "coin": "TUSD",
      "confirmations": 64,
      "confirmedTime": "2019-03-05T09:56:55.728933+00:00",
      "fee": 0,
      "id": 1,
      "sentTime": "2019-03-05T09:56:55.735929+00:00",
      "size": 99.0,
      "status": "confirmed",
      "time": "2019-03-05T09:56:55.728933+00:00",
      "txid": "0x8078356ae4b06a036d64747546c274af19581f1c78c510b60505798a7ffcaf1",
			"address": {"address": "test-addr", "tag": "test-tag"}
    }
  ]
}
`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, respJSON)
	}))
	defer ts.Close()

	ex := NewExchange("test-key", "test-secret", "")
	serverURL, err := url.Parse(ts.URL)
	assert.NoError(t, err)
	ex.client.BaseURL = serverURL
	ex.restEndpoint = serverURL

	ctx := context.Background()
	layout := "2006-01-02T15:04:05.999999Z07:00"
	actualConfirmedTime, err := time.Parse(layout, "2019-03-05T09:56:55.728933+00:00")
	assert.NoError(t, err)
	dh, err := ex.QueryDepositHistory(ctx, "TUSD", actualConfirmedTime.Add(-1*time.Hour), actualConfirmedTime.Add(1*time.Hour))
	assert.NoError(t, err)
	assert.Len(t, dh, 1)
	assert.Equal(t, types.Deposit{
		Exchange:      types.ExchangeFTX,
		Time:          types.Time(actualConfirmedTime),
		Amount:        fixedpoint.NewFromInt(99),
		Asset:         "TUSD",
		TransactionID: "0x8078356ae4b06a036d64747546c274af19581f1c78c510b60505798a7ffcaf1",
		Status:        types.DepositSuccess,
		Address:       "test-addr",
		AddressTag:    "test-tag",
	}, dh[0])

	// not in the time range
	dh, err = ex.QueryDepositHistory(ctx, "TUSD", actualConfirmedTime.Add(1*time.Hour), actualConfirmedTime.Add(2*time.Hour))
	assert.NoError(t, err)
	assert.Len(t, dh, 0)

	// exclude by asset
	dh, err = ex.QueryDepositHistory(ctx, "BTC", actualConfirmedTime.Add(-1*time.Hour), actualConfirmedTime.Add(1*time.Hour))
	assert.NoError(t, err)
	assert.Len(t, dh, 0)
}

func TestExchange_QueryTrades(t *testing.T) {
	t.Run("empty response", func(t *testing.T) {
		respJSON := `
{
  "success": true,
  "result": []
}
`
		var f fillsResponse
		assert.NoError(t, json.Unmarshal([]byte(respJSON), &f))
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, respJSON)
		}))
		defer ts.Close()

		ex := NewExchange("test-key", "test-secret", "")
		serverURL, err := url.Parse(ts.URL)
		assert.NoError(t, err)
		ex.client.BaseURL = serverURL

		ctx := context.Background()
		actualConfirmedTime, err := parseDatetime("2021-02-23T09:29:08.534000+00:00")
		assert.NoError(t, err)

		since := actualConfirmedTime.Add(-1 * time.Hour)
		until := actualConfirmedTime.Add(1 * time.Hour)

		// ignore unavailable market
		trades, err := ex.QueryTrades(ctx, "TSLA/USD", &types.TradeQueryOptions{
			StartTime:   &since,
			EndTime:     &until,
			Limit:       0,
			LastTradeID: 0,
		})
		assert.NoError(t, err)
		assert.Len(t, trades, 0)
	})

	t.Run("duplicated response", func(t *testing.T) {
		respJSON := `
{
  "success": true,
  "result": [{
		"id": 123,
		"market": "TSLA/USD",
		"future": null,
		"baseCurrency": "TSLA",
		"quoteCurrency": "USD",
		"type": "order",
		"side": "sell",
		"price": 672.5,
		"size": 1.0,
		"orderId": 456,
		"time": "2021-02-23T09:29:08.534000+00:00",
		"tradeId": 789,
		"feeRate": -5e-6,
		"fee": -0.0033625,
		"feeCurrency": "USD",
		"liquidity": "maker"
}, {
		"id": 123,
		"market": "TSLA/USD",
		"future": null,
		"baseCurrency": "TSLA",
		"quoteCurrency": "USD",
		"type": "order",
		"side": "sell",
		"price": 672.5,
		"size": 1.0,
		"orderId": 456,
		"time": "2021-02-23T09:29:08.534000+00:00",
		"tradeId": 789,
		"feeRate": -5e-6,
		"fee": -0.0033625,
		"feeCurrency": "USD",
		"liquidity": "maker"
}]
}
`
		var f fillsResponse
		assert.NoError(t, json.Unmarshal([]byte(respJSON), &f))
		i := 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if i == 0 {
				fmt.Fprintln(w, respJSON)
				return
			}
			fmt.Fprintln(w, `{"success":true, "result":[]}`)
		}))
		defer ts.Close()

		ex := NewExchange("test-key", "test-secret", "")
		serverURL, err := url.Parse(ts.URL)
		assert.NoError(t, err)
		ex.client.BaseURL = serverURL

		ctx := context.Background()
		actualConfirmedTime, err := parseDatetime("2021-02-23T09:29:08.534000+00:00")
		assert.NoError(t, err)

		since := actualConfirmedTime.Add(-1 * time.Hour)
		until := actualConfirmedTime.Add(1 * time.Hour)

		// ignore unavailable market
		trades, err := ex.QueryTrades(ctx, "TSLA/USD", &types.TradeQueryOptions{
			StartTime:   &since,
			EndTime:     &until,
			Limit:       0,
			LastTradeID: 0,
		})
		assert.NoError(t, err)
		assert.Len(t, trades, 1)
		assert.Equal(t, types.Trade{
			ID:            789,
			OrderID:       456,
			Exchange:      types.ExchangeFTX,
			Price:         fixedpoint.NewFromFloat(672.5),
			Quantity:      fixedpoint.One,
			QuoteQuantity: fixedpoint.NewFromFloat(672.5 * 1.0),
			Symbol:        "TSLAUSD",
			Side:          types.SideTypeSell,
			IsBuyer:       false,
			IsMaker:       true,
			Time:          types.Time(actualConfirmedTime),
			Fee:           fixedpoint.NewFromFloat(-0.0033625),
			FeeCurrency:   "USD",
			IsMargin:      false,
			IsIsolated:    false,
			StrategyID:    sql.NullString{},
			PnL:           sql.NullFloat64{},
		}, trades[0])
	})
}

func Test_isIntervalSupportedInKLine(t *testing.T) {
	supportedIntervals := []types.Interval{
		types.Interval1m,
		types.Interval5m,
		types.Interval15m,
		types.Interval1h,
		types.Interval1d,
	}
	for _, i := range supportedIntervals {
		assert.True(t, isIntervalSupportedInKLine(i))
	}
	assert.False(t, isIntervalSupportedInKLine(types.Interval30m))
	assert.False(t, isIntervalSupportedInKLine(types.Interval2h))
	assert.True(t, isIntervalSupportedInKLine(types.Interval3d))
}
