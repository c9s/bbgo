package okexapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
	"github.com/pkg/errors"
)

const defaultHTTPTimeout = time.Second * 15
const RestBaseURL = "https://www.okex.com/"
const PublicWebSocketURL = "wss://ws.okx.com:8443/ws/v5/public"
const PrivateWebSocketURL = "wss://ws.okx.com:8443/ws/v5/private"

// Candlesticks and Block trading related websocket switch to below link
// https://www.okx.com/hk/help/changes-to-v5-api-websocket-subscription-parameter-and-url
const PublicCandlesticksWebSocketURL = "wss://ws.okx.com:8443/ws/v5/business"

type SideType string

const (
	SideTypeBuy  SideType = "buy"
	SideTypeSell SideType = "sell"
)

type OrderType string

const (
	OrderTypeMarket   OrderType = "market"
	OrderTypeLimit    OrderType = "limit"
	OrderTypePostOnly OrderType = "post_only"
	OrderTypeFOK      OrderType = "fok"
	OrderTypeIOC      OrderType = "ioc"
)

type InstrumentType string

const (
	InstrumentTypeSpot    InstrumentType = "SPOT"
	InstrumentTypeSwap    InstrumentType = "SWAP"
	InstrumentTypeFutures InstrumentType = "FUTURES"
	InstrumentTypeOption  InstrumentType = "OPTION"
	InstrumentTypeMARGIN  InstrumentType = "MARGIN"
)

type OrderState string

const (
	OrderStateCanceled        OrderState = "canceled"
	OrderStateLive            OrderState = "live"
	OrderStatePartiallyFilled OrderState = "partially_filled"
	OrderStateFilled          OrderState = "filled"
)

type RestClient struct {
	requestgen.BaseAPIClient

	Key, Secret, Passphrase string
}

var parsedBaseURL *url.URL

func init() {
	url, err := url.Parse(RestBaseURL)
	if err != nil {
		panic(err)
	}
	parsedBaseURL = url
}

func NewClient() *RestClient {
	client := &RestClient{
		BaseAPIClient: requestgen.BaseAPIClient{
			BaseURL: parsedBaseURL,
			HttpClient: &http.Client{
				Timeout: defaultHTTPTimeout,
			},
		},
	}
	return client
}

func (c *RestClient) Auth(key, secret, passphrase string) {
	c.Key = key
	// pragma: allowlist nextline secret
	c.Secret = secret
	c.Passphrase = passphrase
}

// NewAuthenticatedRequest creates new http request for authenticated routes.
func (c *RestClient) NewAuthenticatedRequest(ctx context.Context, method, refURL string, params url.Values, payload interface{}) (*http.Request, error) {
	if len(c.Key) == 0 {
		return nil, errors.New("empty api key")
	}

	if len(c.Secret) == 0 {
		return nil, errors.New("empty api secret")
	}

	rel, err := url.Parse(refURL)
	if err != nil {
		return nil, err
	}

	if params != nil {
		rel.RawQuery = params.Encode()
	}

	pathURL := c.BaseURL.ResolveReference(rel)
	path := pathURL.Path
	if rel.RawQuery != "" {
		path += "?" + rel.RawQuery
	}

	// set location to UTC so that it outputs "2020-12-08T09:08:57.715Z"
	t := time.Now().In(time.UTC)
	timestamp := t.Format("2006-01-02T15:04:05.999Z07:00")

	var body []byte

	if payload != nil {
		switch v := payload.(type) {
		case string:
			body = []byte(v)

		case []byte:
			body = v

		default:
			body, err = json.Marshal(v)
			if err != nil {
				return nil, err
			}
		}
	}

	signKey := timestamp + strings.ToUpper(method) + path + string(body)
	signature := Sign(signKey, c.Secret)

	req, err := http.NewRequest(method, pathURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("OK-ACCESS-KEY", c.Key)
	req.Header.Add("OK-ACCESS-SIGN", signature)
	req.Header.Add("OK-ACCESS-TIMESTAMP", timestamp)
	req.Header.Add("OK-ACCESS-PASSPHRASE", c.Passphrase)
	return req, nil
}

type BalanceDetail struct {
	Currency                string                     `json:"ccy"`
	Available               fixedpoint.Value           `json:"availEq"`
	CashBalance             fixedpoint.Value           `json:"cashBal"`
	OrderFrozen             fixedpoint.Value           `json:"ordFrozen"`
	Frozen                  fixedpoint.Value           `json:"frozenBal"`
	Equity                  fixedpoint.Value           `json:"eq"`
	EquityInUSD             fixedpoint.Value           `json:"eqUsd"`
	UpdateTime              types.MillisecondTimestamp `json:"uTime"`
	UnrealizedProfitAndLoss fixedpoint.Value           `json:"upl"`
}

type Account struct {
	TotalEquityInUSD fixedpoint.Value           `json:"totalEq"`
	UpdateTime       types.MillisecondTimestamp `json:"uTime"`
	Details          []BalanceDetail            `json:"details"`
}

func (c *RestClient) AccountBalances(ctx context.Context) (*Account, error) {
	req, err := c.NewAuthenticatedRequest(ctx, "GET", "/api/v5/account/balance", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := c.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var balanceResponse struct {
		Code    string    `json:"code"`
		Message string    `json:"msg"`
		Data    []Account `json:"data"`
	}

	if err := response.DecodeJSON(&balanceResponse); err != nil {
		return nil, err
	}

	if len(balanceResponse.Data) == 0 {
		return nil, errors.New("empty account data")
	}

	return &balanceResponse.Data[0], nil
}

type AssetBalance struct {
	Currency  string           `json:"ccy"`
	Balance   fixedpoint.Value `json:"bal"`
	Frozen    fixedpoint.Value `json:"frozenBal,omitempty"`
	Available fixedpoint.Value `json:"availBal,omitempty"`
}

type AssetBalanceList []AssetBalance

func (c *RestClient) AssetBalances(ctx context.Context) (AssetBalanceList, error) {
	req, err := c.NewAuthenticatedRequest(ctx, "GET", "/api/v5/asset/balances", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := c.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var balanceResponse struct {
		Code    string           `json:"code"`
		Message string           `json:"msg"`
		Data    AssetBalanceList `json:"data"`
	}
	if err := response.DecodeJSON(&balanceResponse); err != nil {
		return nil, err
	}

	return balanceResponse.Data, nil
}

type AssetCurrency struct {
	Currency               string           `json:"ccy"`
	Name                   string           `json:"name"`
	Chain                  string           `json:"chain"`
	CanDeposit             bool             `json:"canDep"`
	CanWithdraw            bool             `json:"canWd"`
	CanInternal            bool             `json:"canInternal"`
	MinWithdrawalFee       fixedpoint.Value `json:"minFee"`
	MaxWithdrawalFee       fixedpoint.Value `json:"maxFee"`
	MinWithdrawalThreshold fixedpoint.Value `json:"minWd"`
}

func (c *RestClient) AssetCurrencies(ctx context.Context) ([]AssetCurrency, error) {
	req, err := c.NewAuthenticatedRequest(ctx, "GET", "/api/v5/asset/currencies", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := c.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var currencyResponse struct {
		Code    string          `json:"code"`
		Message string          `json:"msg"`
		Data    []AssetCurrency `json:"data"`
	}

	if err := response.DecodeJSON(&currencyResponse); err != nil {
		return nil, err
	}

	return currencyResponse.Data, nil
}

type MarketTicker struct {
	InstrumentType string `json:"instType"`
	InstrumentID   string `json:"instId"`

	// last traded price
	Last fixedpoint.Value `json:"last"`

	// last traded size
	LastSize fixedpoint.Value `json:"lastSz"`

	AskPrice fixedpoint.Value `json:"askPx"`
	AskSize  fixedpoint.Value `json:"askSz"`

	BidPrice fixedpoint.Value `json:"bidPx"`
	BidSize  fixedpoint.Value `json:"bidSz"`

	Open24H           fixedpoint.Value `json:"open24h"`
	High24H           fixedpoint.Value `json:"high24H"`
	Low24H            fixedpoint.Value `json:"low24H"`
	Volume24H         fixedpoint.Value `json:"vol24h"`
	VolumeCurrency24H fixedpoint.Value `json:"volCcy24h"`

	// Millisecond timestamp
	Timestamp types.MillisecondTimestamp `json:"ts"`
}

func (c *RestClient) MarketTicker(ctx context.Context, instId string) (*MarketTicker, error) {
	// SPOT, SWAP, FUTURES, OPTION
	var params = url.Values{}
	params.Add("instId", instId)

	req, err := c.NewRequest(ctx, "GET", "/api/v5/market/ticker", params, nil)
	if err != nil {
		return nil, err
	}

	response, err := c.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var tickerResponse struct {
		Code    string         `json:"code"`
		Message string         `json:"msg"`
		Data    []MarketTicker `json:"data"`
	}
	if err := response.DecodeJSON(&tickerResponse); err != nil {
		return nil, err
	}

	if len(tickerResponse.Data) == 0 {
		return nil, fmt.Errorf("ticker of %s not found", instId)
	}

	return &tickerResponse.Data[0], nil
}

func (c *RestClient) MarketTickers(ctx context.Context, instType InstrumentType) ([]MarketTicker, error) {
	// SPOT, SWAP, FUTURES, OPTION
	var params = url.Values{}
	params.Add("instType", string(instType))

	req, err := c.NewRequest(ctx, "GET", "/api/v5/market/tickers", params, nil)
	if err != nil {
		return nil, err
	}

	response, err := c.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var tickerResponse struct {
		Code    string         `json:"code"`
		Message string         `json:"msg"`
		Data    []MarketTicker `json:"data"`
	}
	if err := response.DecodeJSON(&tickerResponse); err != nil {
		return nil, err
	}

	return tickerResponse.Data, nil
}

func Sign(payload string, secret string) string {
	var sig = hmac.New(sha256.New, []byte(secret))
	_, err := sig.Write([]byte(payload))
	if err != nil {
		return ""
	}

	return base64.StdEncoding.EncodeToString(sig.Sum(nil))
	// return hex.EncodeToString(sig.Sum(nil))
}

type APIResponse struct {
	Code    string          `json:"code"`
	Message string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}
