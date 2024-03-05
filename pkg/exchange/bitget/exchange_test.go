package bitget

import (
	"context"
	"math"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

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
