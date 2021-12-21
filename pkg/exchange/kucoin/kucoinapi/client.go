package kucoinapi

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/pkg/errors"
)

const defaultHTTPTimeout = time.Second * 15
const RestBaseURL = "https://api.kucoin.com/api"
const SandboxRestBaseURL = "https://openapi-sandbox.kucoin.com/api"

type RestClient struct {
	BaseURL *url.URL

	client *http.Client

	Key, Secret, Passphrase string
	KeyVersion              string

	AccountService    *AccountService
	MarketDataService *MarketDataService
	TradeService      *TradeService
}

func NewClient() *RestClient {
	u, err := url.Parse(RestBaseURL)
	if err != nil {
		panic(err)
	}

	client := &RestClient{
		BaseURL:    u,
		KeyVersion: "2",
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}

	client.AccountService = &AccountService{client: client}
	client.MarketDataService = &MarketDataService{client: client}
	client.TradeService = &TradeService{client: client}
	return client
}

func (c *RestClient) Auth(key, secret, passphrase string) {
	c.Key = key
	c.Secret = secret
	c.Passphrase = passphrase
}

// NewRequest create new API request. Relative url can be provided in refURL.
func (c *RestClient) newRequest(method, refURL string, params url.Values, body []byte) (*http.Request, error) {
	rel, err := url.Parse(refURL)
	if err != nil {
		return nil, err
	}

	if params != nil {
		rel.RawQuery = params.Encode()
	}

	pathURL := c.BaseURL.ResolveReference(rel)
	return http.NewRequest(method, pathURL.String(), bytes.NewReader(body))
}

// sendRequest sends the request to the API server and handle the response
func (c *RestClient) sendRequest(req *http.Request) (*util.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	// newResponse reads the response body and return a new Response object
	response, err := util.NewResponse(resp)
	if err != nil {
		return response, err
	}

	// Check error, if there is an error, return the ErrorResponse struct type
	if response.IsError() {
		return response, errors.New(string(response.Body))
	}

	return response, nil
}

// newAuthenticatedRequest creates new http request for authenticated routes.
func (c *RestClient) newAuthenticatedRequest(method, refURL string, params url.Values, payload interface{}) (*http.Request, error) {
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
	// timestamp := t.Format("2006-01-02T15:04:05.999Z07:00")
	timestamp := strconv.FormatInt(t.UnixMilli(), 10)

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
	signature := sign(c.Secret, signKey)

	req, err := http.NewRequest(method, pathURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("KC-API-KEY", c.Key)
	req.Header.Add("KC-API-SIGN", signature)
	req.Header.Add("KC-API-TIMESTAMP", timestamp)
	req.Header.Add("KC-API-PASSPHRASE", sign(c.Secret, c.Passphrase))
	req.Header.Add("KC-API-KEY-VERSION", c.KeyVersion)
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

type AssetBalance struct {
	Currency  string           `json:"ccy"`
	Balance   fixedpoint.Value `json:"bal"`
	Frozen    fixedpoint.Value `json:"frozenBal,omitempty"`
	Available fixedpoint.Value `json:"availBal,omitempty"`
}

type AssetBalanceList []AssetBalance

func (c *RestClient) AssetBalances() (AssetBalanceList, error) {
	req, err := c.newAuthenticatedRequest("GET", "/api/v5/asset/balances", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := c.sendRequest(req)
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

func (c *RestClient) AssetCurrencies() ([]AssetCurrency, error) {
	req, err := c.newAuthenticatedRequest("GET", "/api/v5/asset/currencies", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := c.sendRequest(req)
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

func (c *RestClient) MarketTicker(instId string) (*MarketTicker, error) {
	// SPOT, SWAP, FUTURES, OPTION
	var params = url.Values{}
	params.Add("instId", instId)

	req, err := c.newRequest("GET", "/api/v5/market/ticker", params, nil)
	if err != nil {
		return nil, err
	}

	response, err := c.sendRequest(req)
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

func sign(secret, payload string) string {
	var sig = hmac.New(sha256.New, []byte(secret))
	_, err := sig.Write([]byte(payload))
	if err != nil {
		return ""
	}

	return base64.StdEncoding.EncodeToString(sig.Sum(nil))
}
