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

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/httptesting"
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
				assert.Len(query, 6)
				assert.Contains(query, "begin")
				assert.Contains(query, "end")
				assert.Contains(query, "limit")
				assert.Contains(query, "instId")
				assert.Contains(query, "instType")
				assert.Contains(query, "before")
				assert.Equal(query["begin"], []string{strconv.FormatInt(since.UnixNano()/int64(time.Millisecond), 10)})
				assert.Equal(query["end"], []string{strconv.FormatInt(until.UnixNano()/int64(time.Millisecond), 10)})
				assert.Equal(query["limit"], []string{strconv.FormatInt(defaultQueryLimit, 10)})
				assert.Equal(query["instId"], []string{expLocalBtcSymbol})
				assert.Equal(query["instType"], []string{string(okexapi.InstrumentTypeSpot)})
				assert.Equal(query["before"], []string{"0"})
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
				expTrades = append(expTrades, tradeToGlobal(*trade))
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
				assert.Len(query, 6)

				if query["before"][0] == "0" {
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
				assert.Equal(query["before"], []string{strconv.FormatInt(int64(billId+defaultQueryLimit-1), 10)})

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
				assert.Len(query, 6)
				assert.Contains(query, "begin")
				assert.Contains(query, "end")
				assert.Contains(query, "limit")
				assert.Contains(query, "instId")
				assert.Contains(query, "instType")
				assert.Contains(query, "before")
				assert.Equal(query["begin"], []string{strconv.FormatInt(newSince.UnixNano()/int64(time.Millisecond), 10)})
				assert.Equal(query["end"], []string{strconv.FormatInt(until.UnixNano()/int64(time.Millisecond), 10)})
				assert.Equal(query["limit"], []string{strconv.FormatInt(defaultQueryLimit, 10)})
				assert.Equal(query["instId"], []string{expLocalBtcSymbol})
				assert.Equal(query["instType"], []string{string(okexapi.InstrumentTypeSpot)})
				assert.Equal(query["before"], []string{"0"})
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
				expTrades = append(expTrades, tradeToGlobal(*trade))
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
				assert.Len(query, 6)

				if query["before"][0] == "0" {
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
				assert.Equal(query["before"], []string{strconv.FormatInt(int64(billId+defaultQueryLimit-1), 10)})

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
			assert.Len(query, 6)
			assert.Contains(query, "begin")
			assert.Contains(query, "end")
			assert.Contains(query, "limit")
			assert.Contains(query, "instId")
			assert.Contains(query, "instType")
			assert.Contains(query, "before")
			assert.Equal(query["begin"], []string{strconv.FormatInt(expSinceTime.UnixNano()/int64(time.Millisecond), 10)})
			assert.Equal(query["end"], []string{strconv.FormatInt(until.UnixNano()/int64(time.Millisecond), 10)})
			assert.Equal(query["limit"], []string{strconv.FormatInt(defaultQueryLimit, 10)})
			assert.Equal(query["instId"], []string{expLocalBtcSymbol})
			assert.Equal(query["instType"], []string{string(okexapi.InstrumentTypeSpot)})
			assert.Equal(query["before"], []string{"0"})
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
