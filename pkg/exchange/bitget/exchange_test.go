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
		assert        = assert.New(t)
		ex            = New("key", "secret", "passphrase")
		placeOrderUrl = "/api/v2/spot/trade/place-order"
		tickerUrl     = "/api/v2/spot/market/tickers"
		clientOrderId = "684a79df-f931-474f-a9a5-f1deab1cd770"
		expBtcSymbol  = "BTCUSDT"
		mkt           = types.Market{
			Symbol:          expBtcSymbol,
			LocalSymbol:     expBtcSymbol,
			PricePrecision:  fixedpoint.MustNewFromString("2").Int(),
			VolumePrecision: fixedpoint.MustNewFromString("6").Int(),
			StepSize:        fixedpoint.NewFromFloat(1.0 / math.Pow10(6)),
			TickSize:        fixedpoint.NewFromFloat(1.0 / math.Pow10(2)),
		}
		expOrder = &types.Order{
			SubmitOrder: types.SubmitOrder{
				ClientOrderID: clientOrderId,
				Symbol:        expBtcSymbol,
				Side:          types.SideTypeBuy,
				Type:          types.OrderTypeLimit,
				Quantity:      fixedpoint.MustNewFromString("0.00009"),
				Price:         fixedpoint.MustNewFromString("66000"),
				TimeInForce:   types.TimeInForceGTC,
				Market:        mkt,
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
			Market:        mkt,
			TimeInForce:   types.TimeInForceGTC,
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

		acct, err := ex.SubmitOrder(context.Background(), reqLimitOrder)
		assert.NoError(err)
		expOrder.CreationTime = acct.CreationTime
		expOrder.UpdateTime = acct.UpdateTime
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

		reqLimitOrder2 := reqLimitOrder
		reqLimitOrder2.Type = types.OrderTypeLimitMaker
		acct, err := ex.SubmitOrder(context.Background(), reqLimitOrder2)
		assert.NoError(err)

		expOrder2 := *expOrder
		expOrder2.Status = types.OrderStatusNew
		expOrder2.IsWorking = true
		expOrder2.Type = types.OrderTypeLimitMaker
		expOrder2.SubmitOrder = reqLimitOrder2
		acct.CreationTime = expOrder2.CreationTime
		acct.UpdateTime = expOrder2.UpdateTime
		assert.Equal(&expOrder2, acct)
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

			reqMarketOrder := reqLimitOrder
			reqMarketOrder.Side = types.SideTypeBuy
			reqMarketOrder.Type = types.OrderTypeMarket
			acct, err := ex.SubmitOrder(context.Background(), reqMarketOrder)
			assert.NoError(err)
			expOrder2 := *expOrder
			expOrder2.Side = types.SideTypeBuy
			expOrder2.Type = types.OrderTypeMarket
			expOrder2.CreationTime = acct.CreationTime
			expOrder2.UpdateTime = acct.UpdateTime
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

			reqMarketOrder := reqLimitOrder
			reqMarketOrder.Side = types.SideTypeSell
			reqMarketOrder.Type = types.OrderTypeMarket
			acct, err := ex.SubmitOrder(context.Background(), reqMarketOrder)
			assert.NoError(err)
			expOrder2 := *expOrder
			expOrder2.Side = types.SideTypeSell
			expOrder2.Type = types.OrderTypeMarket
			expOrder2.CreationTime = acct.CreationTime
			expOrder2.UpdateTime = acct.UpdateTime
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
			placeOrderResp.ClientOrderId = "unexpected client order id"

			raw, err = json.Marshal(placeOrderResp)
			assert.NoError(err)
			apiResp.Data = raw
			raw, err = json.Marshal(apiResp)
			assert.NoError(err)

			return httptesting.BuildResponseString(http.StatusOK, string(raw)), nil
		})

		_, err = ex.SubmitOrder(context.Background(), reqLimitOrder)
		assert.ErrorContains(err, "unexpected order id")
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

func TestExchange_QueryClosedOrders(t *testing.T) {
	var (
		assert       = assert.New(t)
		ex           = New("key", "secret", "passphrase")
		expBtcSymbol = "BTCUSDT"
		since        = types.NewMillisecondTimestampFromInt(1709645944272).Time()
		until        = since.Add(time.Hour)
		lastOrderId  = uint64(0)
		url          = "/api/v2/spot/trade/history-orders"
	)

	t.Run("succeeds on market buy", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		// order history
		historyOrderFile, err := os.ReadFile("bitgetapi/v2/testdata/get_history_orders_request.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			assert.Len(query, 4)
			assert.Contains(query, "startTime")
			assert.Contains(query, "endTime")
			assert.Contains(query, "limit")
			assert.Contains(query, "symbol")
			assert.Equal(query["startTime"], []string{strconv.FormatInt(since.UnixNano()/int64(time.Millisecond), 10)})
			assert.Equal(query["endTime"], []string{strconv.FormatInt(until.UnixNano()/int64(time.Millisecond), 10)})
			assert.Equal(query["limit"], []string{strconv.FormatInt(queryLimit, 10)})
			assert.Equal(query["symbol"], []string{expBtcSymbol})
			return httptesting.BuildResponseString(http.StatusOK, string(historyOrderFile)), nil
		})

		orders, err := ex.QueryClosedOrders(context.Background(), expBtcSymbol, since, until, lastOrderId)
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
				Status:           types.OrderStatusCanceled,
				ExecutedQuantity: fixedpoint.MustNewFromString("0"),
				IsWorking:        false,
				CreationTime:     types.Time(types.NewMillisecondTimestampFromInt(1709645944272).Time()),
				UpdateTime:       types.Time(types.NewMillisecondTimestampFromInt(1709648518713).Time()),
			},
			{
				SubmitOrder: types.SubmitOrder{
					ClientOrderID: "bf3ba805-66bc-4ef6-bf34-d63d79dc2e4c",
					Symbol:        expBtcSymbol,
					Side:          types.SideTypeBuy,
					Type:          types.OrderTypeMarket,
					Quantity:      fixedpoint.MustNewFromString("0.000089"),
					Price:         fixedpoint.MustNewFromString("67360.87"),
					TimeInForce:   types.TimeInForceGTC,
				},
				Exchange:         types.ExchangeBitget,
				OrderID:          1148914989018062853,
				UUID:             "1148914989018062853",
				Status:           types.OrderStatusFilled,
				ExecutedQuantity: fixedpoint.MustNewFromString("0.000089"),
				IsWorking:        false,
				CreationTime:     types.Time(types.NewMillisecondTimestampFromInt(1709648599867).Time()),
				UpdateTime:       types.Time(types.NewMillisecondTimestampFromInt(1709648600016).Time()),
			},
		}
		assert.Equal(expOrder, orders)
	})

	t.Run("adjust time range since unexpected since and until", func(t *testing.T) {
		timeNow := time.Now().Truncate(time.Second)
		ex.timeNowFn = func() time.Time {
			return timeNow
		}
		defer func() { ex.timeNowFn = func() time.Time { return time.Now() } }()
		newSince := timeNow.Add(-maxHistoricalDataQueryPeriod * 2)
		expNewSince := timeNow.Add(-maxHistoricalDataQueryPeriod)
		newUntil := timeNow.Add(-maxHistoricalDataQueryPeriod)
		expNewUntil := timeNow

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		// order history
		historyOrderFile, err := os.ReadFile("bitgetapi/v2/testdata/get_history_orders_request.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			assert.Len(query, 4)
			assert.Contains(query, "startTime")
			assert.Contains(query, "endTime")
			assert.Contains(query, "limit")
			assert.Contains(query, "symbol")
			assert.Equal(query["startTime"], []string{strconv.FormatInt(expNewSince.UnixNano()/int64(time.Millisecond), 10)})
			assert.Equal(query["endTime"], []string{strconv.FormatInt(expNewUntil.UnixNano()/int64(time.Millisecond), 10)})
			assert.Equal(query["limit"], []string{strconv.FormatInt(queryLimit, 10)})
			assert.Equal(query["symbol"], []string{expBtcSymbol})
			return httptesting.BuildResponseString(http.StatusOK, string(historyOrderFile)), nil
		})

		orders, err := ex.QueryClosedOrders(context.Background(), expBtcSymbol, newSince, newUntil, lastOrderId)
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
				Status:           types.OrderStatusCanceled,
				ExecutedQuantity: fixedpoint.MustNewFromString("0"),
				IsWorking:        false,
				CreationTime:     types.Time(types.NewMillisecondTimestampFromInt(1709645944272).Time()),
				UpdateTime:       types.Time(types.NewMillisecondTimestampFromInt(1709648518713).Time()),
			},
			{
				SubmitOrder: types.SubmitOrder{
					ClientOrderID: "bf3ba805-66bc-4ef6-bf34-d63d79dc2e4c",
					Symbol:        expBtcSymbol,
					Side:          types.SideTypeBuy,
					Type:          types.OrderTypeMarket,
					Quantity:      fixedpoint.MustNewFromString("0.000089"),
					Price:         fixedpoint.MustNewFromString("67360.87"),
					TimeInForce:   types.TimeInForceGTC,
				},
				Exchange:         types.ExchangeBitget,
				OrderID:          1148914989018062853,
				UUID:             "1148914989018062853",
				Status:           types.OrderStatusFilled,
				ExecutedQuantity: fixedpoint.MustNewFromString("0.000089"),
				IsWorking:        false,
				CreationTime:     types.Time(types.NewMillisecondTimestampFromInt(1709648599867).Time()),
				UpdateTime:       types.Time(types.NewMillisecondTimestampFromInt(1709648600016).Time()),
			},
		}
		assert.Equal(expOrder, orders)
	})

	t.Run("error", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/request_error.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusBadRequest, string(f)), nil
		})

		_, err = ex.QueryClosedOrders(context.Background(), expBtcSymbol, since, until, lastOrderId)
		assert.ErrorContains(err, "Invalid IP")
	})
}

func TestExchange_CancelOrders(t *testing.T) {
	var (
		assert         = assert.New(t)
		ex             = New("key", "secret", "passphrase")
		cancelOrderUrl = "/api/v2/spot/trade/cancel-order"
		order          = types.Order{
			OrderID: 1149899973610643488,
			SubmitOrder: types.SubmitOrder{
				ClientOrderID: "9471cf38-33c2-4aee-a2fb-fcf71629ffb7",
			},
		}
	)

	t.Run("order id first, either orderId or clientOid is required", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		// order history
		historyOrderFile, err := os.ReadFile("bitgetapi/v2/testdata/cancel_order_request.json")
		assert.NoError(err)

		transport.POST(cancelOrderUrl, func(req *http.Request) (*http.Response, error) {
			raw, err := io.ReadAll(req.Body)
			assert.NoError(err)

			type cancelOrder struct {
				OrderId       string `json:"orderId"`
				ClientOrderId string `json:"clientOrderId"`
			}
			reqq := &cancelOrder{}
			err = json.Unmarshal(raw, &reqq)
			assert.NoError(err)

			assert.Equal(strconv.FormatUint(order.OrderID, 10), reqq.OrderId)
			assert.Equal("", reqq.ClientOrderId)
			return httptesting.BuildResponseString(http.StatusOK, string(historyOrderFile)), nil
		})

		err = ex.CancelOrders(context.Background(), order)
		assert.NoError(err)
	})

	t.Run("unexpected order id and client order id", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		transport.POST(cancelOrderUrl, func(req *http.Request) (*http.Response, error) {
			raw, err := io.ReadAll(req.Body)
			assert.NoError(err)

			type cancelOrder struct {
				OrderId       string `json:"orderId"`
				ClientOrderId string `json:"clientOrderId"`
			}
			reqq := &cancelOrder{}
			err = json.Unmarshal(raw, &reqq)
			assert.NoError(err)

			assert.Equal(strconv.FormatUint(order.OrderID, 10), reqq.OrderId)
			assert.Equal("", reqq.ClientOrderId)

			reqq.OrderId = "123456789"
			reqq.ClientOrderId = "test wrong order client id"
			raw, err = json.Marshal(reqq)
			assert.NoError(err)
			apiResp := v2.APIResponse{Code: "00000", Data: raw}
			raw, err = json.Marshal(apiResp)
			assert.NoError(err)
			return httptesting.BuildResponseString(http.StatusOK, string(raw)), nil
		})

		err := ex.CancelOrders(context.Background(), order)
		assert.ErrorContains(err, "order id mismatch")
	})

	t.Run("client order id", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		// order history
		historyOrderFile, err := os.ReadFile("bitgetapi/v2/testdata/cancel_order_request.json")
		assert.NoError(err)

		transport.POST(cancelOrderUrl, func(req *http.Request) (*http.Response, error) {
			raw, err := io.ReadAll(req.Body)
			assert.NoError(err)

			type cancelOrder struct {
				OrderId       string `json:"orderId"`
				ClientOrderId string `json:"clientOid"`
			}
			reqq := &cancelOrder{}
			err = json.Unmarshal(raw, &reqq)
			assert.NoError(err)

			assert.Equal("", reqq.OrderId)
			assert.Equal(order.ClientOrderID, reqq.ClientOrderId)
			return httptesting.BuildResponseString(http.StatusOK, string(historyOrderFile)), nil
		})

		newOrder := order
		newOrder.OrderID = 0
		err = ex.CancelOrders(context.Background(), newOrder)
		assert.NoError(err)
	})

	t.Run("empty order id and client order id", func(t *testing.T) {
		newOrder := order
		newOrder.OrderID = 0
		newOrder.ClientOrderID = ""
		err := ex.CancelOrders(context.Background(), newOrder)
		assert.ErrorContains(err, "the order uuid and client order id are empty")
	})

	t.Run("error", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/request_error.json")
		assert.NoError(err)

		transport.POST(cancelOrderUrl, func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusBadRequest, string(f)), nil
		})

		err = ex.CancelOrders(context.Background(), order)
		assert.ErrorContains(err, "Invalid IP")
	})
}

func TestExchange_QueryTrades(t *testing.T) {
	var (
		assert       = assert.New(t)
		ex           = New("key", "secret", "passphrase")
		expApeSymbol = "APEUSDT"
		since        = types.NewMillisecondTimestampFromInt(1709645944272).Time()
		until        = since.Add(time.Hour)
		options      = &types.TradeQueryOptions{
			StartTime:   &since,
			EndTime:     &until,
			Limit:       queryLimit,
			LastTradeID: 0,
		}
		url      = "/api/v2/spot/trade/fills"
		expOrder = []types.Trade{
			{
				ID:            1149103068190019665,
				OrderID:       1149103067745689603,
				Exchange:      types.ExchangeBitget,
				Price:         fixedpoint.MustNewFromString("1.9959"),
				Quantity:      fixedpoint.MustNewFromString("2.98"),
				QuoteQuantity: fixedpoint.MustNewFromString("5.947782"),
				Symbol:        expApeSymbol,
				Side:          types.SideTypeSell,
				IsBuyer:       false,
				IsMaker:       false,
				Time:          types.Time(types.NewMillisecondTimestampFromInt(1709693441436).Time()),
				Fee:           fixedpoint.MustNewFromString("0.005947782"),
				FeeCurrency:   "USDT",
				FeeDiscounted: false,
			},
			{
				ID:            1149101368775479371,
				OrderID:       1149101366691176462,
				Exchange:      types.ExchangeBitget,
				Price:         fixedpoint.MustNewFromString("2.01"),
				Quantity:      fixedpoint.MustNewFromString("0.0013"),
				QuoteQuantity: fixedpoint.MustNewFromString("0.002613"),
				Symbol:        expApeSymbol,
				Side:          types.SideTypeBuy,
				IsBuyer:       true,
				IsMaker:       true,
				Time:          types.Time(types.NewMillisecondTimestampFromInt(1709693036264).Time()),
				Fee:           fixedpoint.MustNewFromString("0.0000013"),
				FeeCurrency:   "APE",
				FeeDiscounted: false,
			},
			{
				ID:            1149098107964166145,
				OrderID:       1149098107519836161,
				Exchange:      types.ExchangeBitget,
				Price:         fixedpoint.MustNewFromString("2.0087"),
				Quantity:      fixedpoint.MustNewFromString("2.987"),
				QuoteQuantity: fixedpoint.MustNewFromString("5.9999869"),
				Symbol:        expApeSymbol,
				Side:          types.SideTypeBuy,
				IsBuyer:       true,
				IsMaker:       false,
				Time:          types.Time(types.NewMillisecondTimestampFromInt(1709692258826).Time()),
				Fee:           fixedpoint.MustNewFromString("0.002987"),
				FeeCurrency:   "APE",
				FeeDiscounted: false,
			},
			{
				ID:            1149096769322684417,
				OrderID:       1149096768878354435,
				Exchange:      types.ExchangeBitget,
				Price:         fixedpoint.MustNewFromString("2.0068"),
				Quantity:      fixedpoint.MustNewFromString("2.9603"),
				QuoteQuantity: fixedpoint.MustNewFromString("5.94073004"),
				Symbol:        expApeSymbol,
				Side:          types.SideTypeSell,
				IsBuyer:       false,
				IsMaker:       false,
				Time:          types.Time(types.NewMillisecondTimestampFromInt(1709691939669).Time()),
				Fee:           fixedpoint.MustNewFromString("0.00594073004"),
				FeeCurrency:   "USDT",
				FeeDiscounted: false,
			},
		}
	)

	t.Run("succeeds", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		// order history
		historyOrderFile, err := os.ReadFile("bitgetapi/v2/testdata/get_trade_fills_request.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			assert.Len(query, 4)
			assert.Contains(query, "startTime")
			assert.Contains(query, "endTime")
			assert.Contains(query, "limit")
			assert.Contains(query, "symbol")
			assert.Equal(query["startTime"], []string{strconv.FormatInt(since.UnixNano()/int64(time.Millisecond), 10)})
			assert.Equal(query["endTime"], []string{strconv.FormatInt(until.UnixNano()/int64(time.Millisecond), 10)})
			assert.Equal(query["limit"], []string{strconv.FormatInt(queryLimit, 10)})
			assert.Equal(query["symbol"], []string{expApeSymbol})
			return httptesting.BuildResponseString(http.StatusOK, string(historyOrderFile)), nil
		})

		orders, err := ex.QueryTrades(context.Background(), expApeSymbol, options)
		assert.NoError(err)
		assert.Equal(expOrder, orders)
	})

	t.Run("succeeds with lastTradeId (not supported) and limit 0", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		// order history
		historyOrderFile, err := os.ReadFile("bitgetapi/v2/testdata/get_trade_fills_request.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			assert.Len(query, 4)
			assert.Contains(query, "startTime")
			assert.Contains(query, "endTime")
			assert.Contains(query, "limit")
			assert.Contains(query, "symbol")
			assert.Equal(query["startTime"], []string{strconv.FormatInt(since.UnixNano()/int64(time.Millisecond), 10)})
			assert.Equal(query["endTime"], []string{strconv.FormatInt(until.UnixNano()/int64(time.Millisecond), 10)})
			assert.Equal(query["limit"], []string{strconv.FormatInt(queryLimit, 10)})
			assert.Equal(query["symbol"], []string{expApeSymbol})
			return httptesting.BuildResponseString(http.StatusOK, string(historyOrderFile)), nil
		})

		newOpts := *options
		newOpts.LastTradeID = 50
		newOpts.Limit = 0
		orders, err := ex.QueryTrades(context.Background(), expApeSymbol, &newOpts)
		assert.NoError(err)
		assert.Equal(expOrder, orders)
	})

	t.Run("adjust time range since unexpected since and until", func(t *testing.T) {
		timeNow := time.Now().Truncate(time.Second)
		ex.timeNowFn = func() time.Time {
			return timeNow
		}
		defer func() { ex.timeNowFn = func() time.Time { return time.Now() } }()
		newSince := timeNow.Add(-maxHistoricalDataQueryPeriod * 2)
		expNewSince := timeNow.Add(-maxHistoricalDataQueryPeriod)
		newUntil := timeNow.Add(-maxHistoricalDataQueryPeriod + 24*time.Hour)
		newOpts := *options
		newOpts.StartTime = &newSince
		newOpts.EndTime = &newUntil

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		// order history
		historyOrderFile, err := os.ReadFile("bitgetapi/v2/testdata/get_trade_fills_request.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			assert.Len(query, 4)
			assert.Contains(query, "startTime")
			assert.Contains(query, "endTime")
			assert.Contains(query, "limit")
			assert.Contains(query, "symbol")
			assert.Equal(query["startTime"], []string{strconv.FormatInt(expNewSince.UnixNano()/int64(time.Millisecond), 10)})
			assert.Equal(query["endTime"], []string{strconv.FormatInt(newUntil.UnixNano()/int64(time.Millisecond), 10)})
			assert.Equal(query["limit"], []string{strconv.FormatInt(queryLimit, 10)})
			assert.Equal(query["symbol"], []string{expApeSymbol})
			return httptesting.BuildResponseString(http.StatusOK, string(historyOrderFile)), nil
		})

		orders, err := ex.QueryTrades(context.Background(), expApeSymbol, &newOpts)
		assert.NoError(err)
		assert.Equal(expOrder, orders)
	})

	t.Run("failed due to empty since", func(t *testing.T) {
		timeNow := time.Now().Truncate(time.Second)
		ex.timeNowFn = func() time.Time {
			return timeNow
		}
		defer func() { ex.timeNowFn = func() time.Time { return time.Now() } }()
		newSince := timeNow.Add(-maxHistoricalDataQueryPeriod * 2)
		newUntil := timeNow.Add(-maxHistoricalDataQueryPeriod - time.Second)
		newOpts := *options
		newOpts.StartTime = &newSince
		newOpts.EndTime = &newUntil

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		_, err := ex.QueryTrades(context.Background(), expApeSymbol, &newOpts)
		assert.ErrorContains(err, "before start")
	})

	t.Run("failed due to empty since", func(t *testing.T) {
		newOpts := *options
		newOpts.StartTime = nil

		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		_, err := ex.QueryTrades(context.Background(), expApeSymbol, &newOpts)
		assert.ErrorContains(err, "start time is required")
	})

	t.Run("error", func(t *testing.T) {
		transport := &httptesting.MockTransport{}
		ex.client.HttpClient.Transport = transport

		f, err := os.ReadFile("bitgetapi/v2/testdata/request_error.json")
		assert.NoError(err)

		transport.GET(url, func(req *http.Request) (*http.Response, error) {
			return httptesting.BuildResponseString(http.StatusBadRequest, string(f)), nil
		})

		_, err = ex.QueryTrades(context.Background(), expApeSymbol, options)
		assert.ErrorContains(err, "Invalid IP")
	})
}
