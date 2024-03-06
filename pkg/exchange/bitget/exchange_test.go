package bitget

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	v2 "github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi/v2"
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

		f, err := os.ReadFile("bitgetapi/v2/testdata/request_error.json")
		assert.NoError(t, err)

		transport.GET("/api/v2/spot/public/symbols", func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusBadRequest, string(f)), nil
		})

		_, err = ex.QueryMarkets(context.Background())
		assert.ErrorContains(t, err, "Invalid IP")
	})
}

func TestExchange_QueryTicker(t *testing.T) {
	var (
		assert = assert.New(t)
		ex     = New("key", "secret", "passphrase")
		url    = "/api/v2/spot/market/tickers"
	)

	t.Run("succeeds", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/get_ticker_request.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusOK, string(f)), nil
		})

		tickers, err := ex.QueryTicker(context.Background(), "BTCUSDT")
		assert.NoError(err)
		expTicker := &types.Ticker{
			Time:   types.NewMillisecondTimestampFromInt(1709626631127).Time(),
			Volume: fixedpoint.MustNewFromString("29439.351448"),
			Last:   fixedpoint.MustNewFromString("66554.03"),
			Open:   fixedpoint.MustNewFromString("64654.54"),
			High:   fixedpoint.MustNewFromString("68686.93"),
			Low:    fixedpoint.MustNewFromString("64583.42"),
			Buy:    fixedpoint.MustNewFromString("66554"),
			Sell:   fixedpoint.MustNewFromString("66554.07"),
		}
		assert.Equal(expTicker, tickers)
	})

	t.Run("unexpected length", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/get_tickers_request.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusOK, string(f)), nil
		})

		_, err = ex.QueryTicker(context.Background(), "BTCUSDT")
		assert.ErrorContains(err, "unexpected length of query")
	})

	t.Run("error", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/request_error.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusBadRequest, string(f)), nil
		})

		_, err = ex.QueryTicker(context.Background(), "BTCUSDT")
		assert.ErrorContains(err, "Invalid IP")
	})
}

func TestExchange_QueryTickers(t *testing.T) {
	var (
		assert       = assert.New(t)
		ex           = New("key", "secret", "passphrase")
		url          = "/api/v2/spot/market/tickers"
		expBtcSymbol = "BTCUSDT"
		expBtcTicker = types.Ticker{
			Time:   types.NewMillisecondTimestampFromInt(1709626631127).Time(),
			Volume: fixedpoint.MustNewFromString("29439.351448"),
			Last:   fixedpoint.MustNewFromString("66554.03"),
			Open:   fixedpoint.MustNewFromString("64654.54"),
			High:   fixedpoint.MustNewFromString("68686.93"),
			Low:    fixedpoint.MustNewFromString("64583.42"),
			Buy:    fixedpoint.MustNewFromString("66554"),
			Sell:   fixedpoint.MustNewFromString("66554.07"),
		}
	)

	t.Run("succeeds", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/get_tickers_request.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusOK, string(f)), nil
		})

		tickers, err := ex.QueryTickers(context.Background())
		assert.NoError(err)
		expTickers := map[string]types.Ticker{
			expBtcSymbol: expBtcTicker,
			"ETHUSDT": {
				Time:   types.NewMillisecondTimestampFromInt(1709626631726).Time(),
				Volume: fixedpoint.MustNewFromString("243220.866"),
				Last:   fixedpoint.MustNewFromString("3686.95"),
				Open:   fixedpoint.MustNewFromString("3506.6"),
				High:   fixedpoint.MustNewFromString("3740"),
				Low:    fixedpoint.MustNewFromString("3461.17"),
				Buy:    fixedpoint.MustNewFromString("3686.94"),
				Sell:   fixedpoint.MustNewFromString("3686.98"),
			},
		}
		assert.Equal(expTickers, tickers)
	})

	t.Run("succeeds for query one markets", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/get_ticker_request.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			assert.Contains(req.URL.Query(), "symbol")
			assert.Equal(req.URL.Query()["symbol"], []string{expBtcSymbol})
			return httptesting.BuildResponseString(http.StatusOK, string(f)), nil
		})

		tickers, err := ex.QueryTickers(context.Background(), expBtcSymbol)
		assert.NoError(err)
		expTickers := map[string]types.Ticker{
			expBtcSymbol: expBtcTicker,
		}
		assert.Equal(expTickers, tickers)
	})

	t.Run("error", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/request_error.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusBadRequest, string(f)), nil
		})

		_, err = ex.QueryTicker(context.Background(), expBtcSymbol)
		assert.ErrorContains(err, "Invalid IP")
	})
}

func TestExchange_QueryKLines(t *testing.T) {
	var (
		assert       = assert.New(t)
		ex           = New("key", "secret", "passphrase")
		url          = "/api/v2/spot/market/candles"
		expBtcSymbol = "BTCUSDT"
		interval     = types.Interval4h
		expBtcKlines = []types.KLine{
			{
				Exchange:    types.ExchangeBitget,
				Symbol:      expBtcSymbol,
				StartTime:   types.Time(types.NewMillisecondTimestampFromInt(1709352000000).Time()),
				EndTime:     types.Time(types.NewMillisecondTimestampFromInt(1709352000000).Time().Add(interval.Duration() - time.Millisecond)),
				Interval:    interval,
				Open:        fixedpoint.MustNewFromString("62308.42"),
				Close:       fixedpoint.MustNewFromString("62014.17"),
				High:        fixedpoint.MustNewFromString("62308.43"),
				Low:         fixedpoint.MustNewFromString("61760"),
				Volume:      fixedpoint.MustNewFromString("987.377637"),
				QuoteVolume: fixedpoint.MustNewFromString("61283110.57046518"),
				Closed:      false,
			},
			{
				Exchange:    types.ExchangeBitget,
				Symbol:      expBtcSymbol,
				StartTime:   types.Time(types.NewMillisecondTimestampFromInt(1709366400000).Time()),
				EndTime:     types.Time(types.NewMillisecondTimestampFromInt(1709366400000).Time().Add(interval.Duration() - time.Millisecond)),
				Interval:    interval,
				Open:        fixedpoint.MustNewFromString("62014.17"),
				Close:       fixedpoint.MustNewFromString("61825.64"),
				High:        fixedpoint.MustNewFromString("62122.8"),
				Low:         fixedpoint.MustNewFromString("61648.26"),
				Volume:      fixedpoint.MustNewFromString("1271.183413"),
				QuoteVolume: fixedpoint.MustNewFromString("78680550.55539777"),
				Closed:      false,
			},
		}
	)

	t.Run("succeeds without time range", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/get_k_line_request.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			assert.Len(query, 3)
			assert.Contains(query, "symbol")
			assert.Contains(query, "granularity")
			assert.Contains(query, "limit")
			assert.Equal(query["symbol"], []string{expBtcSymbol})
			assert.Equal(query["granularity"], []string{interval.String()})
			assert.Equal(query["limit"], []string{strconv.Itoa(defaultKLineLimit)})
			return httptesting.BuildResponseString(http.StatusOK, string(f)), nil
		})

		klines, err := ex.QueryKLines(context.Background(), expBtcSymbol, interval, types.KLineQueryOptions{})
		assert.NoError(err)
		assert.Equal(expBtcKlines, klines)
	})

	t.Run("succeeds with time range", func(t *testing.T) {
		var (
			transport   = &httptesting.MockTransport{}
			limit       = 50
			startTime   = time.Now()
			endTime     = startTime.Add(8 * time.Hour)
			startTimeMs = strconv.FormatInt(startTime.UnixNano()/int64(time.Millisecond), 10)
			endTimeMs   = strconv.FormatInt(endTime.UnixNano()/int64(time.Millisecond), 10)
		)

		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/get_k_line_request.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			assert.Len(query, 5)
			assert.Contains(query, "symbol")
			assert.Contains(query, "granularity")
			assert.Contains(query, "limit")
			assert.Contains(query, "startTime")
			assert.Contains(query, "endTime")
			assert.Equal(query["symbol"], []string{expBtcSymbol})
			assert.Equal(query["granularity"], []string{interval.String()})
			assert.Equal(query["limit"], []string{strconv.Itoa(limit)})
			assert.Equal(query["startTime"], []string{startTimeMs})
			assert.Equal(query["endTime"], []string{endTimeMs})

			return httptesting.BuildResponseString(http.StatusOK, string(f)), nil
		})

		klines, err := ex.QueryKLines(context.Background(), expBtcSymbol, interval, types.KLineQueryOptions{
			Limit:     limit,
			StartTime: &startTime,
			EndTime:   &endTime,
		})
		assert.NoError(err)
		assert.Equal(expBtcKlines, klines)
	})

	t.Run("error", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/request_error.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusBadRequest, string(f)), nil
		})

		_, err = ex.QueryKLines(context.Background(), expBtcSymbol, interval, types.KLineQueryOptions{})
		assert.ErrorContains(err, "Invalid IP")
	})

	t.Run("reach max duration", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponse(http.StatusBadRequest, nil), nil
		})

		before31Days := time.Now().Add(-31 * 24 * time.Hour)
		_, err := ex.QueryKLines(context.Background(), expBtcSymbol, types.Interval1m, types.KLineQueryOptions{
			EndTime: &before31Days,
		})
		assert.ErrorContains(err, "are greater than max duration")
	})

	t.Run("end time before start time", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponse(http.StatusBadRequest, nil), nil
		})

		startTime := time.Now()
		endTime := startTime.Add(-time.Hour)
		_, err := ex.QueryKLines(context.Background(), expBtcSymbol, types.Interval1m, types.KLineQueryOptions{
			StartTime: &startTime,
			EndTime:   &endTime,
		})
		assert.ErrorContains(err, "before start time")
	})

	t.Run("unexpected duraiton", func(t *testing.T) {
		_, err := ex.QueryKLines(context.Background(), expBtcSymbol, "87h", types.KLineQueryOptions{})
		assert.ErrorContains(err, "not supported")
	})
}

func TestExchange_QueryAccount(t *testing.T) {
	var (
		assert     = assert.New(t)
		ex         = New("key", "secret", "passphrase")
		url        = "/api/v2/spot/account/assets"
		expAccount = &types.Account{
			AccountType:        "spot",
			FuturesInfo:        nil,
			MarginInfo:         nil,
			IsolatedMarginInfo: nil,
			MarginLevel:        fixedpoint.Zero,
			MarginTolerance:    fixedpoint.Zero,
			BorrowEnabled:      false,
			TransferEnabled:    false,
			MarginRatio:        fixedpoint.Zero,
			LiquidationPrice:   fixedpoint.Zero,
			LiquidationRate:    fixedpoint.Zero,
			MakerFeeRate:       fixedpoint.Zero,
			TakerFeeRate:       fixedpoint.Zero,
			TotalAccountValue:  fixedpoint.Zero,
			CanDeposit:         false,
			CanTrade:           false,
			CanWithdraw:        false,
		}
		balances = types.BalanceMap{
			"BTC": {
				Currency:          "BTC",
				Available:         fixedpoint.MustNewFromString("0.00000690"),
				Locked:            fixedpoint.Zero,
				Borrowed:          fixedpoint.Zero,
				Interest:          fixedpoint.Zero,
				NetAsset:          fixedpoint.Zero,
				MaxWithdrawAmount: fixedpoint.Zero,
			},
			"USDT": {
				Currency:          "USDT",
				Available:         fixedpoint.MustNewFromString("0.68360342"),
				Locked:            fixedpoint.MustNewFromString("9.08096000"),
				Borrowed:          fixedpoint.Zero,
				Interest:          fixedpoint.Zero,
				NetAsset:          fixedpoint.Zero,
				MaxWithdrawAmount: fixedpoint.Zero,
			},
		}
	)
	expAccount.UpdateBalances(balances)

	t.Run("succeeds", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/get_account_assets_request.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			assert.Len(query, 1)
			assert.Contains(query, "limit")
			assert.Equal(query["limit"], []string{string(v2.AssetTypeHoldOnly)})
			return httptesting.BuildResponseString(http.StatusOK, string(f)), nil
		})

		acct, err := ex.QueryAccount(context.Background())
		assert.NoError(err)
		assert.Equal(expAccount, acct)
	})

	t.Run("error", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/request_error.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusBadRequest, string(f)), nil
		})

		_, err = ex.QueryAccount(context.Background())
		assert.ErrorContains(err, "Invalid IP")
	})
}

func TestExchange_QueryAccountBalances(t *testing.T) {
	var (
		assert         = assert.New(t)
		ex             = New("key", "secret", "passphrase")
		url            = "/api/v2/spot/account/assets"
		expBalancesMap = types.BalanceMap{
			"BTC": {
				Currency:          "BTC",
				Available:         fixedpoint.MustNewFromString("0.00000690"),
				Locked:            fixedpoint.Zero,
				Borrowed:          fixedpoint.Zero,
				Interest:          fixedpoint.Zero,
				NetAsset:          fixedpoint.Zero,
				MaxWithdrawAmount: fixedpoint.Zero,
			},
			"USDT": {
				Currency:          "USDT",
				Available:         fixedpoint.MustNewFromString("0.68360342"),
				Locked:            fixedpoint.MustNewFromString("9.08096000"),
				Borrowed:          fixedpoint.Zero,
				Interest:          fixedpoint.Zero,
				NetAsset:          fixedpoint.Zero,
				MaxWithdrawAmount: fixedpoint.Zero,
			},
		}
	)

	t.Run("succeeds", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/get_account_assets_request.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			assert.Len(query, 1)
			assert.Contains(query, "limit")
			assert.Equal(query["limit"], []string{string(v2.AssetTypeHoldOnly)})
			return httptesting.BuildResponseString(http.StatusOK, string(f)), nil
		})

		acct, err := ex.QueryAccountBalances(context.Background())
		assert.NoError(err)
		assert.Equal(expBalancesMap, acct)
	})

	t.Run("error", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/request_error.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusBadRequest, string(f)), nil
		})

		_, err = ex.QueryAccountBalances(context.Background())
		assert.ErrorContains(err, "Invalid IP")
	})
}

func TestExchange_SubmitOrder(t *testing.T) {
	var (
		assert          = assert.New(t)
		ex              = New("key", "secret", "passphrase")
		placeOrderUrl   = "/api/v2/spot/trade/place-order"
		openOrderUrl    = "/api/v2/spot/trade/unfilled-orders"
		tickerUrl       = "/api/v2/spot/market/tickers"
		historyOrderUrl = "/api/v2/spot/trade/history-orders"
		clientOrderId   = "684a79df-f931-474f-a9a5-f1deab1cd770"
		expBtcSymbol    = "BTCUSDT"
		expOrder        = &types.Order{
			SubmitOrder: types.SubmitOrder{
				ClientOrderID: clientOrderId,
				Symbol:        expBtcSymbol,
				Side:          types.SideTypeBuy,
				Type:          types.OrderTypeLimit,
				Quantity:      fixedpoint.MustNewFromString("0.00009"),
				Price:         fixedpoint.MustNewFromString("66000"),
				TimeInForce:   types.TimeInForceGTC,
			},
			Exchange:         types.ExchangeBitget,
			OrderID:          1148903850645331968,
			UUID:             "1148903850645331968",
			Status:           types.OrderStatusNew,
			ExecutedQuantity: fixedpoint.Zero,
			IsWorking:        true,
			CreationTime:     types.Time(types.NewMillisecondTimestampFromInt(1709645944272).Time()),
			UpdateTime:       types.Time(types.NewMillisecondTimestampFromInt(1709645944272).Time()),
		}
		reqLimitOrder = types.SubmitOrder{
			ClientOrderID: clientOrderId,
			Symbol:        expBtcSymbol,
			Side:          types.SideTypeBuy,
			Type:          types.OrderTypeLimit,
			Quantity:      fixedpoint.MustNewFromString("0.00009"),
			Price:         fixedpoint.MustNewFromString("66000"),
			Market: types.Market{
				Symbol:          expBtcSymbol,
				LocalSymbol:     expBtcSymbol,
				PricePrecision:  fixedpoint.MustNewFromString("2").Int(),
				VolumePrecision: fixedpoint.MustNewFromString("6").Int(),
				StepSize:        fixedpoint.NewFromFloat(1.0 / math.Pow10(6)),
				TickSize:        fixedpoint.NewFromFloat(1.0 / math.Pow10(2)),
			},
			TimeInForce: types.TimeInForceGTC,
		}
	)

	type NewOrder struct {
		ClientOid string `json:"clientOid"`
		Force     string `json:"force"`
		OrderType string `json:"orderType"`
		Price     string `json:"price"`
		Side      string `json:"side"`
		Size      string `json:"size"`
		Symbol    string `json:"symbol"`
	}

	t.Run("Limit order", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		placeOrderFile, err := os.ReadFile("bitgetapi/v2/testdata/place_order_request.json")
		assert.NoError(err)

		transport.POST(placeOrderUrl, func(req *http.Request) (*http.Response, error) {
			raw, err := io.ReadAll(req.Body)
			assert.NoError(err)

			reqq := &NewOrder{}
			err = json.Unmarshal(raw, &reqq)
			assert.NoError(err)
			assert.Equal(&NewOrder{
				ClientOid: expOrder.ClientOrderID,
				Force:     string(v2.OrderForceGTC),
				OrderType: string(v2.OrderTypeLimit),
				Price:     "66000.00",
				Side:      string(v2.SideTypeBuy),
				Size:      "0.000090",
				Symbol:    expBtcSymbol,
			}, reqq)

			return httptesting.BuildResponseString(http.StatusOK, string(placeOrderFile)), nil
		})

		unfilledFile, err := os.ReadFile("bitgetapi/v2/testdata/get_unfilled_orders_request_limit_order.json")
		assert.NoError(err)

		transport.GET(openOrderUrl, func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			assert.Len(query, 1)
			assert.Contains(query, "orderId")
			assert.Equal(query["orderId"], []string{strconv.FormatUint(expOrder.OrderID, 10)})
			return httptesting.BuildResponseString(http.StatusOK, string(unfilledFile)), nil
		})

		acct, err := ex.SubmitOrder(context.Background(), reqLimitOrder)
		assert.NoError(err)
		assert.Equal(expOrder, acct)
	})

	t.Run("Limit Maker order", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		placeOrderFile, err := os.ReadFile("bitgetapi/v2/testdata/place_order_request.json")
		assert.NoError(err)

		transport.POST(placeOrderUrl, func(req *http.Request) (*http.Response, error) {
			raw, err := io.ReadAll(req.Body)
			assert.NoError(err)

			reqq := &NewOrder{}
			err = json.Unmarshal(raw, &reqq)
			assert.NoError(err)
			assert.Equal(&NewOrder{
				ClientOid: expOrder.ClientOrderID,
				Force:     string(v2.OrderForcePostOnly),
				OrderType: string(v2.OrderTypeLimit),
				Price:     "66000.00",
				Side:      string(v2.SideTypeBuy),
				Size:      "0.000090",
				Symbol:    expBtcSymbol,
			}, reqq)

			return httptesting.BuildResponseString(http.StatusOK, string(placeOrderFile)), nil
		})

		unfilledFile, err := os.ReadFile("bitgetapi/v2/testdata/get_unfilled_orders_request_limit_order.json")
		assert.NoError(err)

		transport.GET(openOrderUrl, func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			assert.Len(query, 1)
			assert.Contains(query, "orderId")
			assert.Equal(query["orderId"], []string{strconv.FormatUint(expOrder.OrderID, 10)})
			return httptesting.BuildResponseString(http.StatusOK, string(unfilledFile)), nil
		})

		reqLimitOrder2 := reqLimitOrder
		reqLimitOrder2.Type = types.OrderTypeLimitMaker
		acct, err := ex.SubmitOrder(context.Background(), reqLimitOrder2)
		assert.NoError(err)
		assert.Equal(expOrder, acct)
	})

	t.Run("Market order", func(t *testing.T) {
		t.Run("Buy", func(t *testing.T) {
			transport := &httptesting.MockTransport{}
			ex.client.HttpClient.Transport = transport

			// get ticker to calculate btc amount
			tickerFile, err := os.ReadFile("bitgetapi/v2/testdata/get_ticker_request.json")
			assert.NoError(err)

			transport.GET(tickerUrl, func(req *http.Request) (*http.Response, error) {
				assert.Contains(req.URL.Query(), "symbol")
				assert.Equal(req.URL.Query()["symbol"], []string{expBtcSymbol})
				return httptesting.BuildResponseString(http.StatusOK, string(tickerFile)), nil
			})

			// place order
			placeOrderFile, err := os.ReadFile("bitgetapi/v2/testdata/place_order_request.json")
			assert.NoError(err)

			transport.POST(placeOrderUrl, func(req *http.Request) (*http.Response, error) {
				raw, err := io.ReadAll(req.Body)
				assert.NoError(err)

				reqq := &NewOrder{}
				err = json.Unmarshal(raw, &reqq)
				assert.NoError(err)
				assert.Equal(&NewOrder{
					ClientOid: expOrder.ClientOrderID,
					Force:     string(v2.OrderForceGTC),
					OrderType: string(v2.OrderTypeMarket),
					Price:     "",
					Side:      string(v2.SideTypeBuy),
					Size:      reqLimitOrder.Market.FormatQuantity(fixedpoint.MustNewFromString("66554").Mul(fixedpoint.MustNewFromString("0.00009"))), // ticker: 66554, size: 0.00009
					Symbol:    expBtcSymbol,
				}, reqq)

				return httptesting.BuildResponseString(http.StatusOK, string(placeOrderFile)), nil
			})

			// unfilled order
			unfilledFile, err := os.ReadFile("bitgetapi/v2/testdata/get_unfilled_orders_request_market_buy_order.json")
			assert.NoError(err)

			transport.GET(openOrderUrl, func(req *http.Request) (*http.Response, error) {
				query := req.URL.Query()
				assert.Len(query, 1)
				assert.Contains(query, "orderId")
				assert.Equal(query["orderId"], []string{strconv.FormatUint(expOrder.OrderID, 10)})
				return httptesting.BuildResponseString(http.StatusOK, string(unfilledFile)), nil
			})

			reqMarketOrder := reqLimitOrder
			reqMarketOrder.Side = types.SideTypeBuy
			reqMarketOrder.Type = types.OrderTypeMarket
			acct, err := ex.SubmitOrder(context.Background(), reqMarketOrder)
			assert.NoError(err)
			expOrder2 := *expOrder
			expOrder2.Side = types.SideTypeBuy
			expOrder2.Type = types.OrderTypeMarket
			expOrder2.Quantity = fixedpoint.Zero
			expOrder2.Price = fixedpoint.Zero
			assert.Equal(&expOrder2, acct)
		})

		t.Run("Sell", func(t *testing.T) {
			transport := &httptesting.MockTransport{}
			ex.client.HttpClient.Transport = transport

			// get ticker to calculate btc amount
			tickerFile, err := os.ReadFile("bitgetapi/v2/testdata/get_ticker_request.json")
			assert.NoError(err)

			transport.GET(tickerUrl, func(req *http.Request) (*http.Response, error) {
				assert.Contains(req.URL.Query(), "symbol")
				assert.Equal(req.URL.Query()["symbol"], []string{expBtcSymbol})
				return httptesting.BuildResponseString(http.StatusOK, string(tickerFile)), nil
			})

			// place order
			placeOrderFile, err := os.ReadFile("bitgetapi/v2/testdata/place_order_request.json")
			assert.NoError(err)

			transport.POST(placeOrderUrl, func(req *http.Request) (*http.Response, error) {
				raw, err := io.ReadAll(req.Body)
				assert.NoError(err)

				reqq := &NewOrder{}
				err = json.Unmarshal(raw, &reqq)
				assert.NoError(err)
				assert.Equal(&NewOrder{
					ClientOid: expOrder.ClientOrderID,
					Force:     string(v2.OrderForceGTC),
					OrderType: string(v2.OrderTypeMarket),
					Price:     "",
					Side:      string(v2.SideTypeSell),
					Size:      "0.000090", // size: 0.00009
					Symbol:    expBtcSymbol,
				}, reqq)

				return httptesting.BuildResponseString(http.StatusOK, string(placeOrderFile)), nil
			})

			// unfilled order
			unfilledFile, err := os.ReadFile("bitgetapi/v2/testdata/get_unfilled_orders_request_market_sell_order.json")
			assert.NoError(err)

			transport.GET(openOrderUrl, func(req *http.Request) (*http.Response, error) {
				query := req.URL.Query()
				assert.Len(query, 1)
				assert.Contains(query, "orderId")
				assert.Equal(query["orderId"], []string{strconv.FormatUint(expOrder.OrderID, 10)})
				return httptesting.BuildResponseString(http.StatusOK, string(unfilledFile)), nil
			})

			reqMarketOrder := reqLimitOrder
			reqMarketOrder.Side = types.SideTypeSell
			reqMarketOrder.Type = types.OrderTypeMarket
			acct, err := ex.SubmitOrder(context.Background(), reqMarketOrder)
			assert.NoError(err)
			expOrder2 := *expOrder
			expOrder2.Side = types.SideTypeSell
			expOrder2.Type = types.OrderTypeMarket
			expOrder2.Price = fixedpoint.Zero
			assert.Equal(&expOrder2, acct)
		})

		t.Run("failed to get ticker on buy", func(t *testing.T) {
			transport := &httptesting.MockTransport{}
			ex.client.HttpClient.Transport = transport

			// get ticker to calculate btc amount
			requestErrFile, err := os.ReadFile("bitgetapi/v2/testdata/request_error.json")
			assert.NoError(err)

			transport.GET(tickerUrl, func(req *http.Request) (*http.Response, error) {
				assert.Contains(req.URL.Query(), "symbol")
				assert.Equal(req.URL.Query()["symbol"], []string{expBtcSymbol})
				return httptesting.BuildResponseString(http.StatusBadRequest, string(requestErrFile)), nil
			})

			reqMarketOrder := reqLimitOrder
			reqMarketOrder.Side = types.SideTypeBuy
			reqMarketOrder.Type = types.OrderTypeMarket
			_, err = ex.SubmitOrder(context.Background(), reqMarketOrder)
			assert.ErrorContains(err, "Invalid IP")
		})

		t.Run("get order from history due to unfilled order not found", func(t *testing.T) {
			transport := &httptesting.MockTransport{}
			ex.client.HttpClient.Transport = transport

			// get ticker to calculate btc amount
			tickerFile, err := os.ReadFile("bitgetapi/v2/testdata/get_ticker_request.json")
			assert.NoError(err)

			transport.GET(tickerUrl, func(req *http.Request) (*http.Response, error) {
				assert.Contains(req.URL.Query(), "symbol")
				assert.Equal(req.URL.Query()["symbol"], []string{expBtcSymbol})
				return httptesting.BuildResponseString(http.StatusOK, string(tickerFile)), nil
			})

			// place order
			placeOrderFile, err := os.ReadFile("bitgetapi/v2/testdata/place_order_request.json")
			assert.NoError(err)

			transport.POST(placeOrderUrl, func(req *http.Request) (*http.Response, error) {
				raw, err := io.ReadAll(req.Body)
				assert.NoError(err)

				reqq := &NewOrder{}
				err = json.Unmarshal(raw, &reqq)
				assert.NoError(err)
				assert.Equal(&NewOrder{
					ClientOid: expOrder.ClientOrderID,
					Force:     string(v2.OrderForceGTC),
					OrderType: string(v2.OrderTypeMarket),
					Price:     "",
					Side:      string(v2.SideTypeBuy),
					Size:      reqLimitOrder.Market.FormatQuantity(fixedpoint.MustNewFromString("66554").Mul(fixedpoint.MustNewFromString("0.00009"))), // ticker: 66554, size: 0.00009
					Symbol:    expBtcSymbol,
				}, reqq)

				return httptesting.BuildResponseString(http.StatusOK, string(placeOrderFile)), nil
			})

			// unfilled order
			transport.GET(openOrderUrl, func(req *http.Request) (*http.Response, error) {
				query := req.URL.Query()
				assert.Len(query, 1)
				assert.Contains(query, "orderId")
				assert.Equal(query["orderId"], []string{strconv.FormatUint(expOrder.OrderID, 10)})

				apiResp := v2.APIResponse{Code: "00000"}
				raw, err := json.Marshal(apiResp)
				assert.NoError(err)
				return httptesting.BuildResponseString(http.StatusOK, string(raw)), nil
			})

			// order history
			historyOrderFile, err := os.ReadFile("bitgetapi/v2/testdata/get_history_orders_request_market_buy.json")
			assert.NoError(err)

			transport.GET(historyOrderUrl, func(req *http.Request) (*http.Response, error) {
				query := req.URL.Query()
				assert.Len(query, 1)
				assert.Contains(query, "orderId")
				assert.Equal(query["orderId"], []string{strconv.FormatUint(expOrder.OrderID, 10)})
				return httptesting.BuildResponseString(http.StatusOK, string(historyOrderFile)), nil
			})

			reqMarketOrder := reqLimitOrder
			reqMarketOrder.Side = types.SideTypeBuy
			reqMarketOrder.Type = types.OrderTypeMarket
			acct, err := ex.SubmitOrder(context.Background(), reqMarketOrder)
			assert.NoError(err)
			expOrder2 := *expOrder
			expOrder2.Side = types.SideTypeBuy
			expOrder2.Type = types.OrderTypeMarket
			expOrder2.Status = types.OrderStatusFilled
			expOrder2.ExecutedQuantity = fixedpoint.MustNewFromString("0.000089")
			expOrder2.Quantity = fixedpoint.MustNewFromString("0.000089")
			expOrder2.Price = fixedpoint.MustNewFromString("67360.87")
			expOrder2.IsWorking = false
			assert.Equal(&expOrder2, acct)
		})
	})

	t.Run("error on query open orders", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		placeOrderFile, err := os.ReadFile("bitgetapi/v2/testdata/place_order_request.json")
		assert.NoError(err)

		transport.POST(placeOrderUrl, func(req *http.Request) (*http.Response, error) {
			raw, err := io.ReadAll(req.Body)
			assert.NoError(err)

			reqq := &NewOrder{}
			err = json.Unmarshal(raw, &reqq)
			assert.NoError(err)
			assert.Equal(&NewOrder{
				ClientOid: expOrder.ClientOrderID,
				Force:     string(v2.OrderForceGTC),
				OrderType: string(v2.OrderTypeLimit),
				Price:     "66000.00",
				Side:      string(v2.SideTypeBuy),
				Size:      "0.000090",
				Symbol:    expBtcSymbol,
			}, reqq)

			return httptesting.BuildResponseString(http.StatusOK, string(placeOrderFile)), nil
		})

		unfilledFile, err := os.ReadFile("bitgetapi/v2/testdata/request_error.json")
		assert.NoError(err)

		transport.GET(openOrderUrl, func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			assert.Len(query, 1)
			assert.Contains(query, "orderId")
			assert.Equal(query["orderId"], []string{strconv.FormatUint(expOrder.OrderID, 10)})
			return httptesting.BuildResponseString(http.StatusBadRequest, string(unfilledFile)), nil
		})

		_, err = ex.SubmitOrder(context.Background(), reqLimitOrder)
		assert.ErrorContains(err, "failed to query open order")
	})

	t.Run("unexpected client order id", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		placeOrderFile, err := os.ReadFile("bitgetapi/v2/testdata/place_order_request.json")
		assert.NoError(err)

		transport.POST(placeOrderUrl, func(req *http.Request) (*http.Response, error) {
			raw, err := io.ReadAll(req.Body)
			assert.NoError(err)

			reqq := &NewOrder{}
			err = json.Unmarshal(raw, &reqq)
			assert.NoError(err)
			assert.Equal(&NewOrder{
				ClientOid: expOrder.ClientOrderID,
				Force:     string(v2.OrderForceGTC),
				OrderType: string(v2.OrderTypeLimit),
				Price:     "66000.00",
				Side:      string(v2.SideTypeBuy),
				Size:      "0.000090",
				Symbol:    expBtcSymbol,
			}, reqq)

			apiResp := &v2.APIResponse{}
			err = json.Unmarshal(placeOrderFile, &apiResp)
			assert.NoError(err)
			placeOrderResp := &v2.PlaceOrderResponse{}
			err = json.Unmarshal(apiResp.Data, &placeOrderResp)
			assert.NoError(err)
			// remove the client order id to test
			placeOrderResp.ClientOrderId = ""

			return httptesting.BuildResponseString(http.StatusOK, string(placeOrderFile)), nil
		})

		_, err = ex.SubmitOrder(context.Background(), reqLimitOrder)
		assert.ErrorContains(err, "failed to query open order")
	})

	t.Run("failed to place order", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		placeOrderFile, err := os.ReadFile("bitgetapi/v2/testdata/place_order_request.json")
		assert.NoError(err)

		transport.POST(placeOrderUrl, func(req *http.Request) (*http.Response, error) {
			raw, err := io.ReadAll(req.Body)
			assert.NoError(err)

			reqq := &NewOrder{}
			err = json.Unmarshal(raw, &reqq)
			assert.NoError(err)
			assert.Equal(&NewOrder{
				ClientOid: expOrder.ClientOrderID,
				Force:     string(v2.OrderForceGTC),
				OrderType: string(v2.OrderTypeLimit),
				Price:     "66000.00",
				Side:      string(v2.SideTypeBuy),
				Size:      "0.000090",
				Symbol:    expBtcSymbol,
			}, reqq)

			return httptesting.BuildResponseString(http.StatusBadRequest, string(placeOrderFile)), nil
		})

		_, err = ex.SubmitOrder(context.Background(), reqLimitOrder)
		assert.ErrorContains(err, "failed to place order")
	})

	t.Run("unexpected client order id", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		reqOrder2 := reqLimitOrder
		reqOrder2.ClientOrderID = strings.Repeat("s", maxOrderIdLen+1)
		_, err := ex.SubmitOrder(context.Background(), reqOrder2)
		assert.ErrorContains(err, "unexpected length of client order id")
	})

	t.Run("time-in-force unsupported", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		reqOrder2 := reqLimitOrder
		reqOrder2.TimeInForce = types.TimeInForceIOC
		_, err := ex.SubmitOrder(context.Background(), reqOrder2)
		assert.ErrorContains(err, "not supported")

		reqOrder2.TimeInForce = types.TimeInForceFOK
		_, err = ex.SubmitOrder(context.Background(), reqOrder2)
		assert.ErrorContains(err, "not supported")
	})

	t.Run("unexpected side", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		reqOrder2 := reqLimitOrder
		reqOrder2.Side = "GG"
		_, err := ex.SubmitOrder(context.Background(), reqOrder2)
		assert.ErrorContains(err, "not supported")
	})

	t.Run("unexpected side", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		reqOrder2 := reqLimitOrder
		reqOrder2.Type = "GG"
		_, err := ex.SubmitOrder(context.Background(), reqOrder2)
		assert.ErrorContains(err, "not supported")
	})
}

func TestExchange_QueryOpenOrders(t *testing.T) {
	var (
		assert       = assert.New(t)
		ex           = New("key", "secret", "passphrase")
		expBtcSymbol = "BTCUSDT"
		url          = "/api/v2/spot/trade/unfilled-orders"
	)

	t.Run("succeeds", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/get_unfilled_orders_request_limit_order.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			assert.Len(query, 2)
			assert.Contains(query, "symbol")
			assert.Equal(query["symbol"], []string{expBtcSymbol})
			assert.Equal(query["limit"], []string{strconv.FormatInt(queryLimit, 10)})
			return httptesting.BuildResponseString(http.StatusOK, string(f)), nil
		})

		orders, err := ex.QueryOpenOrders(context.Background(), expBtcSymbol)
		assert.NoError(err)
		expOrder := []types.Order{
			{
				SubmitOrder: types.SubmitOrder{
					ClientOrderID: "684a79df-f931-474f-a9a5-f1deab1cd770",
					Symbol:        expBtcSymbol,
					Side:          types.SideTypeBuy,
					Type:          types.OrderTypeLimit,
					Quantity:      fixedpoint.MustNewFromString("0.00009"),
					Price:         fixedpoint.MustNewFromString("66000"),
					TimeInForce:   types.TimeInForceGTC,
				},
				Exchange:         types.ExchangeBitget,
				OrderID:          1148903850645331968,
				UUID:             "1148903850645331968",
				Status:           types.OrderStatusNew,
				ExecutedQuantity: fixedpoint.Zero,
				IsWorking:        true,
				CreationTime:     types.Time(types.NewMillisecondTimestampFromInt(1709645944272).Time()),
				UpdateTime:       types.Time(types.NewMillisecondTimestampFromInt(1709645944272).Time()),
			},
		}
		assert.Equal(expOrder, orders)
	})

	t.Run("succeeds on pagination with mock limit + 1", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		dataTemplate := `{
      "userId":"8672173294",
      "symbol":"BTCUSDT",
      "orderId":"%d",
      "clientOid":"684a79df-f931-474f-a9a5-f1deab1cd770",
      "priceAvg":"0",
      "size":"0.00009",
      "orderType":"market",
      "side":"sell",
      "status":"live",
      "basePrice":"0",
      "baseVolume":"0",
      "quoteVolume":"0",
      "enterPointSource":"API",
      "orderSource":"market",
      "cTime":"1709645944272",
      "uTime":"1709645944272"
    }`

		openOrdersStr := make([]string, 0, queryLimit+1)
		expOrders := make([]types.Order, 0, queryLimit+1)
		for i := 0; i < queryLimit+1; i++ {
			dataStr := fmt.Sprintf(dataTemplate, i)
			openOrdersStr = append(openOrdersStr, dataStr)

			unfilledOdr := &v2.UnfilledOrder{}
			err := json.Unmarshal([]byte(dataStr), &unfilledOdr)
			assert.NoError(err)
			gOdr, err := unfilledOrderToGlobalOrder(*unfilledOdr)
			assert.NoError(err)
			expOrders = append(expOrders, *gOdr)
		}

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			assert.Contains(query, "symbol")
			assert.Equal(query["symbol"], []string{expBtcSymbol})
			assert.Equal(query["limit"], []string{strconv.FormatInt(queryLimit, 10)})
			if len(query) == 2 {
				// first time query
				resp := &v2.APIResponse{
					Code: "00000",
					Data: []byte("[" + strings.Join(openOrdersStr[0:queryLimit], ",") + "]"),
				}
				respRaw, err := json.Marshal(resp)
				assert.NoError(err)

				return httptesting.BuildResponseString(http.StatusOK, string(respRaw)), nil
			}
			// second time query
			// last order id, so need to -1
			assert.Equal(query["idLessThan"], []string{strconv.FormatInt(queryLimit-1, 10)})

			resp := &v2.APIResponse{
				Code: "00000",
				Data: []byte("[" + strings.Join(openOrdersStr[queryLimit:queryLimit+1], ",") + "]"),
			}
			respRaw, err := json.Marshal(resp)
			assert.NoError(err)
			return httptesting.BuildResponseString(http.StatusOK, string(respRaw)), nil
		})

		orders, err := ex.QueryOpenOrders(context.Background(), expBtcSymbol)
		assert.NoError(err)
		assert.Equal(expOrders, orders)
	})

	t.Run("succeeds on pagination with mock limit + 1", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		dataTemplate := `{
      "userId":"8672173294",
      "symbol":"BTCUSDT",
      "orderId":"%d",
      "clientOid":"684a79df-f931-474f-a9a5-f1deab1cd770",
      "priceAvg":"0",
      "size":"0.00009",
      "orderType":"market",
      "side":"sell",
      "status":"live",
      "basePrice":"0",
      "baseVolume":"0",
      "quoteVolume":"0",
      "enterPointSource":"API",
      "orderSource":"market",
      "cTime":"1709645944272",
      "uTime":"1709645944272"
    }`

		openOrdersStr := make([]string, 0, queryLimit+1)
		for i := 0; i < queryLimit+1; i++ {
			dataStr := fmt.Sprintf(dataTemplate, i)
			openOrdersStr = append(openOrdersStr, dataStr)
		}

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			assert.Contains(query, "symbol")
			assert.Equal(query["symbol"], []string{expBtcSymbol})
			assert.Equal(query["limit"], []string{strconv.FormatInt(queryLimit, 10)})
			// first time query
			resp := &v2.APIResponse{
				Code: "00000",
				Data: []byte("[" + strings.Join(openOrdersStr, ",") + "]"),
			}
			respRaw, err := json.Marshal(resp)
			assert.NoError(err)

			return httptesting.BuildResponseString(http.StatusOK, string(respRaw)), nil
		})

		_, err := ex.QueryOpenOrders(context.Background(), expBtcSymbol)
		assert.ErrorContains(err, "unexpected open orders length")
	})

	t.Run("error", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/request_error.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusBadRequest, string(f)), nil
		})

		_, err = ex.QueryOpenOrders(context.Background(), "BTCUSDT")
		assert.ErrorContains(err, "Invalid IP")
	})
}
