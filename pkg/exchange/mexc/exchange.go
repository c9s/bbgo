package mexc

import (

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"

	"errors"
	"fmt"
	"io"
	"strconv"
	"context"
	"time"
	"crypto/sha256"
	"crypto/hmac"
	"net/http"
	"net/url"
	"encoding/json"
)

const MX = "MX"
var urlTemplate url.URL = url.URL{
	Scheme: "https",
	Host: "api.mexc.com",
	Path: "/api/v3",
	RawPath: "/api/v3",
}

var log = logrus.WithField("exchange", "mexc")

type Exchange struct {
	key, secret string
	client *http.Client
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeMEXC
}

func (e *Exchange) PlatformFeeCurrency() string {
	return MX
}

func (e *Exchange) publicRequest(ctx context.Context, method string, path string, params url.Values) ([]byte, error) {
	u := urlTemplate
	u.Path += path
	u.RawPath += path
	u.RawQuery = params.Encode()
	log.Println(u.String())
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-MEXC-APIKEY", e.key)
	if e.client == nil {
		e.client = &http.Client {
			Transport: &http.Transport{
				MaxIdleConns: 10,
				IdleConnTimeout: 30 * time.Second,
			},
		}
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode > 299 {
		return nil, errors.New(fmt.Sprintf("return status value: %d", resp.StatusCode))
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (e *Exchange) signRequest(ctx context.Context, method string, path string, params url.Values) ([]byte, error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	params.Add("timestamp", timestamp)
	queryString := params.Encode()
	sign := hmac.New(sha256.New, []byte(e.secret))
	sign.Write([]byte(queryString))
	signature := fmt.Sprintf("%x", sign.Sum([]byte{}))
	params.Add("signature", signature)
	u := urlTemplate
	u.Path += path
	u.RawPath += path
	u.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Context-Type", "application/json")
	req.Header.Add("X-MEXC-APIKEY", e.key)
	if e.client == nil {
		e.client = &http.Client {
			Transport: &http.Transport {
				MaxIdleConns: 10,
				IdleConnTimeout: 30 * time.Second,
			},
		}
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode > 299 {
		return nil, errors.New(fmt.Sprintf("return status value: %d", resp.StatusCode))
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (e *Exchange) ping(ctx context.Context) bool {
	_, err := e.publicRequest(ctx, "GET", "/ping", url.Values{})
	if err != nil {
		log.WithError(err).Errorf("ping error")
		return false
	}
	return true
}

type Time struct {
	ServerTime int64 `json:"serverTime"`
}

func (e *Exchange) time(ctx context.Context) (int64, error) {
	var t Time
	resp, err := e.publicRequest(ctx, "GET", "/time", url.Values{})
	if err != nil {
		log.WithError(err).Errorf("time error")
		return 0, err
	}
	json.Unmarshal(resp, &t)
	return t.ServerTime, nil
}

type ticker24hr struct {
	CloseTime int64 `json:"closeTime"`
	OpenTime int64 `json:"openTime"`
	QuoteVolume fixedpoint.Value `json:"quoteVolume"`
	Volume fixedpoint.Value `json:"volume"`
	LowPrice fixedpoint.Value `json:"lowPrice"`
	HighPrice fixedpoint.Value `json:"highPrice"`
	OpenPrice fixedpoint.Value `json:"openPrice"`
	AskPrice fixedpoint.Value `json:"askPrice"`
	BidPrice fixedpoint.Value `json:"bidPrice"`
	LastPrice fixedpoint.Value `json:"lastPrice"`
}

/*{"symbol":"APEUSDT","priceChange":"0.0852","priceChangePercent":"0.0099905","prevClosePrice":"8.5281","lastPrice":"8.6133","lastQty":"","bidPrice":"8.5983","bidQty":"","askPrice":"8.6242","askQty":"","openPrice":"8.5281","highPrice":"9.2478","lowPrice":"8.14","volume":"166512.31","quoteVolume":null,"openTime":1652869200000,"closeTime":1652869439540,"count":null}*/

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	v := url.Values{}
	v.Add("symbol", symbol)
	resp, err := e.publicRequest(ctx, "GET", "/ticker/24hr", v)
	if err != nil {
		log.WithError(err).Errorf("queryTicker error")
		return nil, err
	}
	t := ticker24hr{}
	json.Unmarshal(resp, &t)
	result := types.Ticker{
		Time: time.UnixMilli(t.OpenTime),
		Volume: t.Volume,
		Last: t.LastPrice,
		Open: t.OpenPrice,
		High: t.HighPrice,
		Low: t.LowPrice,
		Buy: t.BidPrice,
		Sell: t.AskPrice,
	}
	return &result, nil
}
/*
func (e *Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
}

func (e *Exchange) NewStream() types.Stream {
}

func (e *Exchange) QueryOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, error) {
}

func (e *Exchange) QueryClosedOrders(ctx context.Context, symbol string, since, util time.Time, lastOrderID uint64) ([]types.Order, error) {
}

func (e *Exchange) CancelAllOrders(ctx context.Context) ([]types.Order, error) {
}

func (e *Exchange) CancelOrdersBySymbol(ctx context.Context, symbol string) ([]types.Order, error) {
}

func (e *Exchange) CancelOrdersByGroupID(ctx context.Context, groupID uint32) ([]types.Order, error) {
}

func (e *Exchange) CancelOrders(ctx context.Context, ordrs ...types.Order) (err error) {
}

func (e *Exchange) Withdrawal(ctx context.Context, asset string, amount fixedpoint.Value, address string, options *types.WithdrawalOptions) error {
}

func (e *Exchange) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error) {
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
}

func (e *Exchange) QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []types.Withdraw, err error) {
}

func (e *Exchange) QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []types.Deposit, err error) {
}

func (e *Exchange) QueryAccountBalance(ctx contexxt.Context) (types.BalanceMap, error) {
}

func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) (trades []types.Trade, err error) {
}

func (e *Exchange) QueryRewards(ctx context.Context, startTime time.Time) ([]types.Reward, error) {
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
}

var _ types.Exchange = &Exchange{}*/
