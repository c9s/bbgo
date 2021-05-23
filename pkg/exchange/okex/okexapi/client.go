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
	"github.com/c9s/bbgo/pkg/util"
	"github.com/pkg/errors"
)

const defaultHTTPTimeout = time.Second * 15
const RestBaseURL = "https://www.okex.com/"
const PublicWebSocketURL = "wss://ws.okex.com:8443/ws/v5/public"
const PrivateWebSocketURL = "wss://ws.okex.com:8443/ws/v5/private"

type SideType string

const (
	SideTypeBuy  SideType = "buy"
	SideTypeSell SideType = "sell"
)

type OrderType string

const (
	OrderTypeMarket   OrderType = "market"
	OrderTypeLimit              = "limit"
	OrderTypePostOnly           = "post_only"
	OrderTypeFOK                = "fok"
	OrderTypeIOC                = "ioc"
)

type RestClient struct {
	BaseURL *url.URL

	client *http.Client

	Key, Secret, Passphrase string
}

func NewClient() *RestClient {
	u, err := url.Parse(RestBaseURL)
	if err != nil {
		panic(err)
	}

	return &RestClient{
		BaseURL: u,
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
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

	req, err := http.NewRequest(method, pathURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	return req, nil
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
	signature := sign(signKey, c.Secret)

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

type BalanceSummary struct {
	TotalEquityInUSD fixedpoint.Value `json:"totalEq"`
	UpdateTime       string           `json:"uTime"`
	Details          []BalanceDetail  `json:"details"`
}

type BalanceSummaryList []BalanceSummary

func (c *RestClient) AccountBalances() (BalanceSummaryList, error) {
	req, err := c.newAuthenticatedRequest("GET", "/api/v5/account/balance", nil, nil)
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
		Data    []BalanceSummary `json:"data"`
	}
	if err := response.DecodeJSON(&balanceResponse); err != nil {
		return nil, err
	}

	return balanceResponse.Data, nil
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

	req, err := c.newAuthenticatedRequest("GET", "/api/v5/market/ticker", params, nil)
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

func (c *RestClient) MarketTickers(instType string) ([]MarketTicker, error) {
	// SPOT, SWAP, FUTURES, OPTION
	var params = url.Values{}
	params.Add("instType", instType)

	req, err := c.newAuthenticatedRequest("GET", "/api/v5/market/tickers", params, nil)
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

	return tickerResponse.Data, nil
}

type OrderResponse struct {
	OrderID       string `json:"ordId"`
	ClientOrderID string `json:"clOrdId"`
	Tag           string `json:"tag"`
	Code          string `json:"sCode"`
	Message       string `json:"sMsg"`
}

type PlaceOrderRequest struct {
	client *RestClient

	instId string

	// tdMode
	// margin mode: "cross", "isolated"
	// non-margin mode cash
	tdMode string

	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 32 characters.
	clientOrderID *string

	// A combination of case-sensitive alphanumerics, all numbers, or all letters of up to 8 characters.
	tag *string

	// "buy" or "sell"
	side SideType

	ordType OrderType

	// sz Quantity
	sz string

	// price
	px *string
}

func (r *PlaceOrderRequest) InstrumentID(instID string) *PlaceOrderRequest {
	r.instId = instID
	return r
}

func (r *PlaceOrderRequest) TradeMode(mode string) *PlaceOrderRequest {
	r.tdMode = mode
	return r
}

func (r *PlaceOrderRequest) ClientOrderID(clientOrderID string) *PlaceOrderRequest {
	r.clientOrderID = &clientOrderID
	return r
}

func (r *PlaceOrderRequest) Side(side SideType) *PlaceOrderRequest {
	r.side = side
	return r
}

func (r *PlaceOrderRequest) Quantity(quantity string) *PlaceOrderRequest {
	r.sz = quantity
	return r
}

func (r *PlaceOrderRequest) Price(price string) *PlaceOrderRequest {
	r.px = &price
	return r
}

func (r *PlaceOrderRequest) OrderType(orderType OrderType) *PlaceOrderRequest {
	r.ordType = orderType
	return r
}

func (r *PlaceOrderRequest) Parameters() map[string]interface{} {
	payload := map[string]interface{}{}

	payload["instId"] = r.instId

	if r.tdMode == "" {
		payload["tdMode"] = "cash"
	} else {
		payload["tdMode"] = r.tdMode
	}

	if r.clientOrderID != nil {
		payload["clOrdId"] = r.clientOrderID
	}

	payload["side"] = r.side
	payload["ordType"] = r.ordType
	payload["sz"] = r.sz
	if r.px != nil {
		payload["px"] = r.px
	}

	if r.tag != nil {
		payload["tag"] = r.tag
	}

	return payload
}

func (r *PlaceOrderRequest) Do(ctx context.Context) (*OrderResponse, error) {
	payload := r.Parameters()
	req, err := r.client.newAuthenticatedRequest("POST", "/api/v5/trade/order", nil, payload)
	if err != nil {
		return nil, err
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var orderResponse struct {
		Code    string          `json:"code"`
		Message string          `json:"msg"`
		Data    []OrderResponse `json:"data"`
	}
	if err := response.DecodeJSON(&orderResponse); err != nil {
		return nil, err
	}

	if len(orderResponse.Data) == 0 {
		return nil, errors.New("order create error")
	}

	return &orderResponse.Data[0], nil
}

func (c *RestClient) NewPlaceOrderRequest() *PlaceOrderRequest {
	return &PlaceOrderRequest{
		client: c,
	}
}

type BatchPlaceOrderRequest struct {
	client *RestClient

	reqs []*PlaceOrderRequest
}

func (r *BatchPlaceOrderRequest) Add(reqs ...*PlaceOrderRequest) *BatchPlaceOrderRequest {
	r.reqs = append(r.reqs, reqs...)
	return r
}

func (r *BatchPlaceOrderRequest) Do(ctx context.Context) (*OrderResponse, error) {
	var parameterList []map[string]interface{}

	for _, req := range r.reqs {
		params := req.Parameters()
		parameterList = append(parameterList, params)
	}

	req, err := r.client.newAuthenticatedRequest("POST", "/api/v5/trade/batch-orders", nil, parameterList)
	if err != nil {
		return nil, err
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var orderResponse struct {
		Code    string          `json:"code"`
		Message string          `json:"msg"`
		Data    []OrderResponse `json:"data"`
	}
	if err := response.DecodeJSON(&orderResponse); err != nil {
		return nil, err
	}

	if len(orderResponse.Data) == 0 {
		return nil, errors.New("order create error")
	}

	return &orderResponse.Data[0], nil
}

func (c *RestClient) NewBatchPlaceOrderRequest() *BatchPlaceOrderRequest {
	return &BatchPlaceOrderRequest{
		client: c,
	}
}

type CancelOrderRequest struct {
	client *RestClient

	instId string
	ordId  *string
	clOrdId *string
}

func (r *CancelOrderRequest) InstrumentID(instId string) *CancelOrderRequest {
	r.instId = instId
	return r
}

func (r *CancelOrderRequest) OrderID(orderID string) *CancelOrderRequest {
	r.ordId = &orderID
	return r
}

func (r *CancelOrderRequest) ClientOrderID(clientOrderID string) *CancelOrderRequest {
	r.clOrdId = &clientOrderID
	return r
}

func (r *CancelOrderRequest) Do(ctx context.Context) (*OrderResponse, error) {
	var payload = map[string]interface{}{
		"instId": r.instId,
	}

	if r.ordId != nil {
		payload["ordId"] = r.ordId
	} else if r.clOrdId != nil {
		payload["clOrdId"] = r.clOrdId
	} else {
		return nil, errors.New("either orderID or clientOrderID is required for canceling order")
	}

	req, err := r.client.newAuthenticatedRequest("POST", "/api/v5/trade/cancel-order", nil, payload)
	if err != nil {
		return nil, err
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var orderResponse struct {
		Code    string          `json:"code"`
		Message string          `json:"msg"`
		Data    []OrderResponse `json:"data"`
	}
	if err := response.DecodeJSON(&orderResponse); err != nil {
		return nil, err
	}

	if len(orderResponse.Data) == 0 {
		return nil, errors.New("order create error")
	}

	return &orderResponse.Data[0], nil
}

func (c *RestClient) NewCancelOrderRequest() *CancelOrderRequest {
	return &CancelOrderRequest{
		client: c,
	}
}

func sign(payload string, secret string) string {
	var sig = hmac.New(sha256.New, []byte(secret))
	_, err := sig.Write([]byte(payload))
	if err != nil {
		return ""
	}

	return base64.StdEncoding.EncodeToString(sig.Sum(nil))
	// return hex.EncodeToString(sig.Sum(nil))
}
