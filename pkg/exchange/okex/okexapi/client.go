package okexapi

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/util"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const defaultHTTPTimeout = time.Second * 15
const RestBaseURL = "https://www.okex.com/"
const PublicWebSocketURL = "wss://ws.okex.com:8443/ws/v5/public"
const PrivateWebSocketURL = "wss://ws.okex.com:8443/ws/v5/private"

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

// newAuthenticatedRequest creates new http request for authenticated routes.
func (c *RestClient) newAuthenticatedRequest(method, refURL string, params url.Values) (*http.Request, error) {
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
	log.Info(timestamp)

	payload := timestamp + strings.ToUpper(method) + path
	sign := signPayload(payload, c.Secret)

	var body []byte
	req, err := http.NewRequest(method, pathURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("OK-ACCESS-KEY", c.Key)
	req.Header.Add("OK-ACCESS-SIGN", sign)
	req.Header.Add("OK-ACCESS-TIMESTAMP", timestamp)
	req.Header.Add("OK-ACCESS-PASSPHRASE", c.Passphrase)

	return req, nil
}

type BalanceDetail struct {
	Currency                string `json:"ccy"`
	Available               string `json:"availEq"`
	CashBalance             string `json:"cashBal"`
	OrderFrozen             string `json:"ordFrozen"`
	Frozen                  string `json:"frozenBal"`
	Equity                  string `json:"eq"`
	EquityInUSD             string `json:"eqUsd"`
	UpdateTime              string `json:"uTime"`
	UnrealizedProfitAndLoss string `json:"upl"`
}

type BalanceSummary struct {
	TotalEquityInUSD string          `json:"totalEq"`
	UpdateTime       string          `json:"uTime"`
	Details          []BalanceDetail `json:"details"`
}

type BalanceSummaryList []BalanceSummary

func (c *RestClient) Balances() (BalanceSummaryList, error) {
	req, err := c.newAuthenticatedRequest("GET", "/api/v5/account/balance", nil)
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

func signPayload(payload string, secret string) string {
	var sig = hmac.New(sha256.New, []byte(secret))
	_, err := sig.Write([]byte(payload))
	if err != nil {
		return ""
	}

	return base64.StdEncoding.EncodeToString(sig.Sum(nil))
	// return hex.EncodeToString(sig.Sum(nil))
}
