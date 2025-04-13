package okex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/envvar"
	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/testutil"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_clientOrderIdRegex(t *testing.T) {
	t.Run("empty client order id", func(t *testing.T) {
		assert.True(t, clientOrderIdRegex.MatchString(""))
	})

	t.Run("mixed of digit and char", func(t *testing.T) {
		assert.True(t, clientOrderIdRegex.MatchString("1s2f3g4h5j"))
	})

	t.Run("mixed of 16 chars and 16 digit", func(t *testing.T) {
		assert.True(t, clientOrderIdRegex.MatchString(strings.Repeat("s", 16)+strings.Repeat("1", 16)))
	})

	t.Run("out of maximum length", func(t *testing.T) {
		assert.False(t, clientOrderIdRegex.MatchString(strings.Repeat("s", 33)))
	})

	t.Run("invalid char: `-`", func(t *testing.T) {
		assert.False(t, clientOrderIdRegex.MatchString(uuid.NewString()))
	})
}

func TestExchange_SubmitMarketOrder(t *testing.T) {
	key, secret, passphrase, _ := testutil.IntegrationTestWithPassphraseConfigured(t, "OKEX")

	ctx := context.Background()
	ex := New(key, secret, passphrase)

	balances, err := ex.QueryAccountBalances(ctx)
	assert.NoError(t, err)

	t.Logf("balances: %+v", balances)

	markets, err := ex.QueryMarkets(ctx)
	assert.NoError(t, err)

	market, ok := markets["BTCUSDT"]
	if assert.True(t, ok) {
		t.Logf("market: %+v", market)

		ex.MarginSettings.IsMargin = true

		createdOrder, err := ex.SubmitOrder(ctx, types.SubmitOrder{
			Symbol:   "BTCUSDT",
			Type:     types.OrderTypeMarket,
			Price:    Number(105200),
			Quantity: fixedpoint.Max(market.MinQuantity, Number(6.0/105200.0)), // 8usdt
			Side:     types.SideTypeBuy,
			Market:   market,
		})
		if assert.NoError(t, err) {
			t.Logf("createdOrder: %+v", createdOrder)
		}
	}
}

func TestExchange_QueryTrades(t *testing.T) {
	var (
		assert            = assert.New(t)
		ex                = New("key", "secret", "passphrase")
		expBtcSymbol      = "BTCUSDT"
		expLocalBtcSymbol = "BTC-USDT"
		until             = time.Now()
		since             = until.Add(-threeDaysHistoricalPeriod)

		options = &types.TradeQueryOptions{
			StartTime:   &since,
			EndTime:     &until,
			Limit:       defaultQueryLimit,
			LastTradeID: 0,
		}
		threeDayUrl = "/api/v5/trade/fills"
		historyUrl  = "/api/v5/trade/fills-history"
		expOrder    = []types.Trade{
			{
				ID:            749554213,
				OrderID:       688362711456706560,
				Exchange:      types.ExchangeOKEx,
				Price:         fixedpoint.MustNewFromString("73397.8"),
				Quantity:      fixedpoint.MustNewFromString("0.001"),
				QuoteQuantity: fixedpoint.MustNewFromString("73.3978"),
				Symbol:        expBtcSymbol,
				Side:          types.SideTypeBuy,
				IsBuyer:       true,
				IsMaker:       false,
				Time:          types.Time(types.NewMillisecondTimestampFromInt(1710390459574).Time()),
				Fee:           fixedpoint.MustNewFromString("0.000001"),
				FeeCurrency:   "BTC",
			},
		}
	)
	ex.timeNowFunc = func() time.Time { return until }

	t.Run("3 days", func(t *testing.T) {
		t.Run("succeeds with one record", func(t *testing.T) {
			transport := &httptesting.MockTransport{}
			ex.client.HttpClient.Transport = transport

			// order history
			historyOrderFile, err := os.ReadFile("okexapi/testdata/get_three_days_transaction_history_request.json")
			assert.NoError(err)

			transport.GET(threeDayUrl, func(req *http.Request) (*http.Response, error) {
				query := req.URL.Query()
				assert.Len(query, 5)
				assert.Contains(query, "begin")
				assert.Contains(query, "end")
				assert.Contains(query, "limit")
				assert.Contains(query, "instId")
				assert.Contains(query, "instType")
				assert.Equal(query["begin"], []string{strconv.FormatInt(since.UnixNano()/int64(time.Millisecond), 10)})
				assert.Equal(query["end"], []string{strconv.FormatInt(until.UnixNano()/int64(time.Millisecond), 10)})
				assert.Equal(query["limit"], []string{strconv.FormatInt(defaultQueryLimit, 10)})
				assert.Equal(query["instId"], []string{expLocalBtcSymbol})
				assert.Equal(query["instType"], []string{string(okexapi.InstrumentTypeSpot)})
				return httptesting.BuildResponseString(http.StatusOK, string(historyOrderFile)), nil
			})

			orders, err := ex.QueryTrades(context.Background(), expBtcSymbol, options)
			assert.NoError(err)
			assert.Equal(expOrder, orders)
		})

		t.Run("succeeds with exceeded max records", func(t *testing.T) {
			transport := &httptesting.MockTransport{}
			ex.client.HttpClient.Transport = transport

			tradeId := 749554213
			billId := 688362711465447466
			dataTemplace := `
    {
      "side":"buy",
      "fillSz":"0.001",
      "fillPx":"73397.8",
      "fillPxVol":"",
      "fillFwdPx":"",
      "fee":"-0.000001",
      "fillPnl":"0",
      "ordId":"688362711456706560",
      "feeRate":"-0.001",
      "instType":"SPOT",
      "fillPxUsd":"",
      "instId":"BTC-USDT",
      "clOrdId":"1229606897",
      "posSide":"net",
      "billId":"%d",
      "fillMarkVol":"",
      "tag":"",
      "fillTime":"1710390459571",
      "execType":"T",
      "fillIdxPx":"",
      "tradeId":"%d",
      "fillMarkPx":"",
      "feeCcy":"BTC",
      "ts":"1710390459574"
    }`

			tradesStr := make([]string, 0, defaultQueryLimit+1)
			expTrades := make([]types.Trade, 0, defaultQueryLimit+1)
			for i := 0; i < defaultQueryLimit+1; i++ {
				dataStr := fmt.Sprintf(dataTemplace, billId+i, tradeId+i)
				tradesStr = append(tradesStr, dataStr)

				trade := &okexapi.Trade{}
				err := json.Unmarshal([]byte(dataStr), &trade)
				assert.NoError(err)
				expTrades = append(expTrades, toGlobalTrade(*trade))
			}

			transport.GET(threeDayUrl, func(req *http.Request) (*http.Response, error) {
				query := req.URL.Query()
				assert.Contains(query, "begin")
				assert.Contains(query, "end")
				assert.Contains(query, "limit")
				assert.Contains(query, "instId")
				assert.Contains(query, "instType")
				assert.Equal(query["begin"], []string{strconv.FormatInt(since.UnixNano()/int64(time.Millisecond), 10)})
				assert.Equal(query["end"], []string{strconv.FormatInt(until.UnixNano()/int64(time.Millisecond), 10)})
				assert.Equal(query["limit"], []string{strconv.FormatInt(defaultQueryLimit, 10)})
				assert.Equal(query["instId"], []string{expLocalBtcSymbol})
				assert.Equal(query["instType"], []string{string(okexapi.InstrumentTypeSpot)})

				if _, found := query["after"]; !found {
					resp := &okexapi.APIResponse{
						Code: "0",
						Data: []byte("[" + strings.Join(tradesStr[0:defaultQueryLimit], ",") + "]"),
					}
					respRaw, err := json.Marshal(resp)
					assert.NoError(err)

					return httptesting.BuildResponseString(http.StatusOK, string(respRaw)), nil
				}

				// second time query
				// last order id, so need to -1
				assert.Equal(query["after"], []string{strconv.FormatInt(int64(billId+defaultQueryLimit-1), 10)})

				resp := okexapi.APIResponse{
					Code: "0",
					Data: []byte("[" + strings.Join(tradesStr[defaultQueryLimit:defaultQueryLimit+1], ",") + "]"),
				}
				respRaw, err := json.Marshal(resp)
				assert.NoError(err)
				return httptesting.BuildResponseString(http.StatusOK, string(respRaw)), nil
			})

			trades, err := ex.QueryTrades(context.Background(), expBtcSymbol, options)
			assert.NoError(err)
			assert.Equal(expTrades, trades)
		})
	})
	t.Run("3 days < x < Max days", func(t *testing.T) {
		t.Run("succeeds with one record", func(t *testing.T) {
			transport := &httptesting.MockTransport{}
			ex.client.HttpClient.Transport = transport
			newSince := until.Add(-maxHistoricalDataQueryPeriod)
			options.StartTime = &newSince

			// order history
			historyOrderFile, err := os.ReadFile("okexapi/testdata/get_transaction_history_request.json")
			assert.NoError(err)

			transport.GET(historyUrl, func(req *http.Request) (*http.Response, error) {
				query := req.URL.Query()
				assert.Len(query, 5)
				assert.Contains(query, "begin")
				assert.Contains(query, "end")
				assert.Contains(query, "limit")
				assert.Contains(query, "instId")
				assert.Contains(query, "instType")
				assert.Equal(query["begin"], []string{strconv.FormatInt(newSince.UnixNano()/int64(time.Millisecond), 10)})
				assert.Equal(query["end"], []string{strconv.FormatInt(until.UnixNano()/int64(time.Millisecond), 10)})
				assert.Equal(query["limit"], []string{strconv.FormatInt(defaultQueryLimit, 10)})
				assert.Equal(query["instId"], []string{expLocalBtcSymbol})
				assert.Equal(query["instType"], []string{string(okexapi.InstrumentTypeSpot)})
				return httptesting.BuildResponseString(http.StatusOK, string(historyOrderFile)), nil
			})

			orders, err := ex.QueryTrades(context.Background(), expBtcSymbol, options)
			assert.NoError(err)
			assert.Equal(expOrder, orders)
		})

		t.Run("succeeds with exceeded max records", func(t *testing.T) {
			transport := &httptesting.MockTransport{}
			ex.client.HttpClient.Transport = transport
			newSince := until.Add(-maxHistoricalDataQueryPeriod)
			options.StartTime = &newSince

			tradeId := 749554213
			billId := 688362711465447466
			dataTemplace := `
    {
      "side":"buy",
      "fillSz":"0.001",
      "fillPx":"73397.8",
      "fillPxVol":"",
      "fillFwdPx":"",
      "fee":"-0.000001",
      "fillPnl":"0",
      "ordId":"688362711456706560",
      "feeRate":"-0.001",
      "instType":"SPOT",
      "fillPxUsd":"",
      "instId":"BTC-USDT",
      "clOrdId":"1229606897",
      "posSide":"net",
      "billId":"%d",
      "fillMarkVol":"",
      "tag":"",
      "fillTime":"1710390459571",
      "execType":"T",
      "fillIdxPx":"",
      "tradeId":"%d",
      "fillMarkPx":"",
      "feeCcy":"BTC",
      "ts":"1710390459574"
    }`

			tradesStr := make([]string, 0, defaultQueryLimit+1)
			expTrades := make([]types.Trade, 0, defaultQueryLimit+1)
			for i := 0; i < defaultQueryLimit+1; i++ {
				dataStr := fmt.Sprintf(dataTemplace, billId+i, tradeId+i)
				tradesStr = append(tradesStr, dataStr)

				trade := &okexapi.Trade{}
				err := json.Unmarshal([]byte(dataStr), &trade)
				assert.NoError(err)
				expTrades = append(expTrades, toGlobalTrade(*trade))
			}

			transport.GET(historyUrl, func(req *http.Request) (*http.Response, error) {
				query := req.URL.Query()
				assert.Contains(query, "begin")
				assert.Contains(query, "end")
				assert.Contains(query, "limit")
				assert.Contains(query, "instId")
				assert.Contains(query, "instType")
				assert.Equal(query["begin"], []string{strconv.FormatInt(newSince.UnixNano()/int64(time.Millisecond), 10)})
				assert.Equal(query["end"], []string{strconv.FormatInt(until.UnixNano()/int64(time.Millisecond), 10)})
				assert.Equal(query["limit"], []string{strconv.FormatInt(defaultQueryLimit, 10)})
				assert.Equal(query["instId"], []string{expLocalBtcSymbol})
				assert.Equal(query["instType"], []string{string(okexapi.InstrumentTypeSpot)})

				if _, found := query["after"]; !found {
					resp := &okexapi.APIResponse{
						Code: "0",
						Data: []byte("[" + strings.Join(tradesStr[0:defaultQueryLimit], ",") + "]"),
					}
					respRaw, err := json.Marshal(resp)
					assert.NoError(err)

					return httptesting.BuildResponseString(http.StatusOK, string(respRaw)), nil
				}

				// second time query
				// last order id, so need to -1
				assert.Equal(query["after"], []string{strconv.FormatInt(int64(billId+defaultQueryLimit-1), 10)})

				resp := okexapi.APIResponse{
					Code: "0",
					Data: []byte("[" + strings.Join(tradesStr[defaultQueryLimit:defaultQueryLimit+1], ",") + "]"),
				}
				respRaw, err := json.Marshal(resp)
				assert.NoError(err)
				return httptesting.BuildResponseString(http.StatusOK, string(respRaw)), nil
			})

			trades, err := ex.QueryTrades(context.Background(), expBtcSymbol, options)
			assert.NoError(err)
			assert.Equal(expTrades, trades)
		})
	})

	t.Run("start time exceeded 3 months", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport
		newSince := options.StartTime.Add(-365 * 24 * time.Hour)
		newOpts := *options
		newOpts.StartTime = &newSince

		expSinceTime := until.Add(-maxHistoricalDataQueryPeriod)

		// order history
		historyOrderFile, err := os.ReadFile("okexapi/testdata/get_three_days_transaction_history_request.json")
		assert.NoError(err)

		transport.GET(historyUrl, func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			assert.Contains(query, "begin")
			assert.Contains(query, "end")
			assert.Contains(query, "limit")
			assert.Contains(query, "instId")
			assert.Contains(query, "instType")
			assert.Equal(query["begin"], []string{strconv.FormatInt(expSinceTime.UnixNano()/int64(time.Millisecond), 10)})
			assert.Equal(query["end"], []string{strconv.FormatInt(until.UnixNano()/int64(time.Millisecond), 10)})
			assert.Equal(query["limit"], []string{strconv.FormatInt(defaultQueryLimit, 10)})
			assert.Equal(query["instId"], []string{expLocalBtcSymbol})
			assert.Equal(query["instType"], []string{string(okexapi.InstrumentTypeSpot)})
			return httptesting.BuildResponseString(http.StatusOK, string(historyOrderFile)), nil
		})

		orders, err := ex.QueryTrades(context.Background(), expBtcSymbol, &newOpts)
		assert.NoError(err)
		assert.Equal(expOrder, orders)
	})

	t.Run("start time after end day", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport
		newSince := options.StartTime.Add(365 * 24 * time.Hour)
		newOpts := *options
		newOpts.StartTime = &newSince

		_, err := ex.QueryTrades(context.Background(), expBtcSymbol, &newOpts)
		assert.ErrorContains(err, "before start")
	})

	t.Run("empty symbol", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport
		newSince := options.StartTime.Add(365 * 24 * time.Hour)
		newOpts := *options
		newOpts.StartTime = &newSince

		_, err := ex.QueryTrades(context.Background(), "", &newOpts)
		assert.ErrorContains(err, ErrSymbolRequired.Error())
	})
}

func TestExchange_Margin(t *testing.T) {
	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, "OKEX")
	if !ok {
		t.SkipNow()
		return
	}

	ctx := context.Background()
	ex := New(key, secret, passphrase)

	maxBorrowable, err := ex.QueryMarginAssetMaxBorrowable(ctx, "BTC")
	if assert.NoError(t, err) {
		assert.NotZero(t, maxBorrowable.Float64())
		t.Logf("max borrowable: %f", maxBorrowable.Float64())

		err2 := ex.BorrowMarginAsset(ctx, "BTC", maxBorrowable)
		if assert.NoError(t, err2) {
			time.Sleep(time.Second)
			err3 := ex.RepayMarginAsset(ctx, "BTC", maxBorrowable)
			assert.NoError(t, err3)
		}
	}
}

func TestExchange_QueryMarkets(t *testing.T) {
	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, "OKEX")
	if !ok {
		t.SkipNow()
		return
	}

	ex := New(key, secret, passphrase)
	markets, err := ex.QueryMarkets(context.Background())
	assert.NoError(t, err)
	assert.NotEmpty(t, markets)

	ex.IsFutures = true
	markets1, err := ex.QueryMarkets(context.Background())
	assert.NoError(t, err)
	assert.NotEmpty(t, markets1)

	assert.NotEqual(t, len(markets), len(markets1))
}

func TestExchange_QueryFuturesOpenOrders(t *testing.T) {
	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, "OKEX")
	if !ok {
		t.SkipNow()
		return
	}

	ex := New(key, secret, passphrase)
	ex.UseFutures()

	orders, err := ex.QueryOpenOrders(context.Background(), "BTCUSDT")
	assert.NoError(t, err)
	assert.NotEmpty(t, orders)
	assert.True(t, orders[0].IsFutures)
}

func TestExchange_Futures_SubmitOrder(t *testing.T) {
	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, "OKEX")
	if !ok {
		t.SkipNow()
		return
	}

	ex := New(key, secret, passphrase)
	ex.UseFutures()
	dualSidePosition = true

	markets, err := ex.QueryMarkets(context.Background())
	assert.NoError(t, err)

	market := markets["BTCUSDT"]
	orders, err := ex.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:   "BTCUSDT",
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
		Price:    fixedpoint.MustNewFromString("90000"),
		Quantity: market.MinQuantity,
		Market:   market,
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, orders)
}

func TestExchange_QueryKlines(t *testing.T) {
	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, "OKEX")
	if !ok {
		t.SkipNow()
		return
	}

	ex := New(key, secret, passphrase)

	klines, err := ex.QueryKLines(context.Background(), "BTCUSDT", types.Interval1m, types.KLineQueryOptions{})
	assert.NoError(t, err)
	assert.NotEmpty(t, klines)

	ex.UseFutures()
	klines, err = ex.QueryKLines(context.Background(), "BTCUSDT", types.Interval1m, types.KLineQueryOptions{})
	assert.NoError(t, err)
	assert.NotEmpty(t, klines)
}

func Test_init(t *testing.T) {
	t.Run("env var not set", func(t *testing.T) {
		os.Unsetenv("OKEX_ENABLE_FUTURES_HEDGE_MODE")

		dualSidePosition = false
		if val, ok := envvar.Bool("OKEX_ENABLE_FUTURES_HEDGE_MODE"); ok {
			dualSidePosition = val
		}

		assert.False(t, dualSidePosition)
	})

	t.Run("env var set true", func(t *testing.T) {
		os.Setenv("OKEX_ENABLE_FUTURES_HEDGE_MODE", "true")

		dualSidePosition = false
		if val, ok := envvar.Bool("OKEX_ENABLE_FUTURES_HEDGE_MODE"); ok {
			dualSidePosition = val
		}

		assert.True(t, dualSidePosition)
	})

	t.Run("env var set false", func(t *testing.T) {
		os.Setenv("OKEX_ENABLE_FUTURES_HEDGE_MODE", "false")

		dualSidePosition = true
		if val, ok := envvar.Bool("OKEX_ENABLE_FUTURES_HEDGE_MODE"); ok {
			dualSidePosition = val
		}

		assert.False(t, dualSidePosition)
	})
}

func TestSetDualSidePosition(t *testing.T) {
	trueValue := true
	tests := []struct {
		name               string
		order              types.SubmitOrder
		expectedPosSide    okexapi.PosSide
		expectedReduceOnly *bool
	}{
		{
			name: "Buy order without reduce only",
			order: types.SubmitOrder{
				Side:       types.SideTypeBuy,
				ReduceOnly: false,
				Type:       types.OrderTypeLimit,
			},
			expectedPosSide:    okexapi.PosSideLong,
			expectedReduceOnly: nil,
		},
		{
			name: "Buy order with reduce only",
			order: types.SubmitOrder{
				Side:       types.SideTypeBuy,
				ReduceOnly: true,
				Type:       types.OrderTypeLimit,
			},
			expectedPosSide:    okexapi.PosSideShort,
			expectedReduceOnly: &trueValue,
		},
		{
			name: "Sell order without reduce only",
			order: types.SubmitOrder{
				Side:       types.SideTypeSell,
				ReduceOnly: false,
				Type:       types.OrderTypeLimit,
			},
			expectedPosSide:    okexapi.PosSideShort,
			expectedReduceOnly: nil,
		},
		{
			name: "Sell order with reduce only",
			order: types.SubmitOrder{
				Side:       types.SideTypeSell,
				ReduceOnly: true,
				Type:       types.OrderTypeLimit,
			},
			expectedPosSide:    okexapi.PosSideLong,
			expectedReduceOnly: &trueValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new PlaceOrderRequest
			req := &okexapi.PlaceOrderRequest{}
			req.TradeMode(okexapi.TradeModeCross)
			req.Side(toLocalSideType(tt.order.Side))

			ordType, err := toLocalOrderType(tt.order.Type)
			assert.NoError(t, err)
			req.OrderType(ordType)

			// Call the function being tested
			setDualSidePosition(req, tt.order)

			// Get the parameters to verify the results
			params, err := req.GetParameters()
			assert.NoError(t, err)

			// Assert the results
			assert.Equal(t, tt.expectedPosSide, params["posSide"])
			if tt.expectedReduceOnly == nil {
				assert.NotContains(t, params, "reduceOnly")
			} else {
				assert.Equal(t, *tt.expectedReduceOnly, params["reduceOnly"])
			}
		})
	}
}

func TestExchange_QueryFuturesAccount(t *testing.T) {
	key, secret, passphrase, ok := testutil.IntegrationTestWithPassphraseConfigured(t, "OKEX")
	if !ok {
		t.SkipNow()
		return
	}

	// Create new exchange instance and enable futures mode
	ex := New(key, secret, passphrase)
	ex.UseFutures()

	ctx := context.Background()
	account, err := ex.QueryAccount(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, account)

	// Verify account type is futures
	assert.Equal(t, types.AccountTypeFutures, account.AccountType)

	// Verify basic account information
	assert.NotEmpty(t, account.Balances)

	// Verify futures specific information
	assert.NotNil(t, account.FuturesInfo)
	assert.NotEmpty(t, account.FuturesInfo.Positions)
}
