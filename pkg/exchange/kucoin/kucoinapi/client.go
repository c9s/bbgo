package kucoinapi

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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
	BulletService     *BulletService
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
	client.BulletService = &BulletService{client: client}
	return client
}

func (c *RestClient) Auth(key, secret, passphrase string) {
	c.Key = key
	c.Secret = secret
	c.Passphrase = passphrase
}

// NewRequest create new API request. Relative url can be provided in refURL.
func (c *RestClient) NewRequest(method, refURL string, params url.Values, body []byte) (*http.Request, error) {
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
func (c *RestClient) SendRequest(req *http.Request) (*util.Response, error) {
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
func (c *RestClient) NewAuthenticatedRequest(method, refURL string, params url.Values, payload interface{}) (*http.Request, error) {
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

	body, err := castPayload(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, pathURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	// Build authentication headers
	c.attachAuthHeaders(req, method, path, body)
	return req, nil
}

func (c *RestClient) attachAuthHeaders(req *http.Request, method string, path string, body []byte) {
	// Set location to UTC so that it outputs "2020-12-08T09:08:57.715Z"
	t := time.Now().In(time.UTC)
	// timestamp := t.Format("2006-01-02T15:04:05.999Z07:00")
	timestamp := strconv.FormatInt(t.UnixNano()/int64(time.Millisecond), 10)
	signKey := timestamp + strings.ToUpper(method) + path + string(body)
	signature := sign(c.Secret, signKey)

	req.Header.Add("KC-API-KEY", c.Key)
	req.Header.Add("KC-API-SIGN", signature)
	req.Header.Add("KC-API-TIMESTAMP", timestamp)
	req.Header.Add("KC-API-PASSPHRASE", sign(c.Secret, c.Passphrase))
	req.Header.Add("KC-API-KEY-VERSION", c.KeyVersion)
}

// sign uses sha256 to sign the payload with the given secret
func sign(secret, payload string) string {
	var sig = hmac.New(sha256.New, []byte(secret))
	_, err := sig.Write([]byte(payload))
	if err != nil {
		return ""
	}

	return base64.StdEncoding.EncodeToString(sig.Sum(nil))
}

func castPayload(payload interface{}) ([]byte, error) {
	if payload != nil {
		switch v := payload.(type) {
		case string:
			return []byte(v), nil

		case []byte:
			return v, nil

		default:
			body, err := json.Marshal(v)
			return body, err
		}
	}

	return nil, nil
}
