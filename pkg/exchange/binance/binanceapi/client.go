package binanceapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/c9s/requestgen"
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/types"
)

const defaultHTTPTimeout = time.Second * 15
const RestBaseURL = "https://api.binance.com"
const SandboxRestBaseURL = "https://testnet.binance.vision"

type RestClient struct {
	BaseURL *url.URL

	client *http.Client

	Key, Secret, Passphrase string
	KeyVersion              string

	recvWindow int
	timeOffset int64
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

	// client.AccountService = &AccountService{client: client}
	return client
}

func (c *RestClient) Auth(key, secret string) {
	c.Key = key
	c.Secret = secret
}

// NewRequest create new API request. Relative url can be provided in refURL.
func (c *RestClient) NewRequest(ctx context.Context, method, refURL string, params url.Values, payload interface{}) (*http.Request, error) {
	rel, err := url.Parse(refURL)
	if err != nil {
		return nil, err
	}

	if params != nil {
		rel.RawQuery = params.Encode()
	}

	body, err := castPayload(payload)
	if err != nil {
		return nil, err
	}

	pathURL := c.BaseURL.ResolveReference(rel)
	return http.NewRequestWithContext(ctx, method, pathURL.String(), bytes.NewReader(body))
}

// sendRequest sends the request to the API server and handle the response
func (c *RestClient) SendRequest(req *http.Request) (*requestgen.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	// newResponse reads the response body and return a new Response object
	response, err := requestgen.NewResponse(resp)
	if err != nil {
		return response, err
	}

	// Check error, if there is an error, return the ErrorResponse struct type
	if response.IsError() {
		return response, errors.New(string(response.Body))
	}

	return response, nil
}

func (c *RestClient) SetTimeOffsetFromServer(ctx context.Context) error {
	req, err := c.NewRequest(ctx, "GET", "/api/v3/time", nil, nil)
	if err != nil {
		return err
	}

	resp, err := c.SendRequest(req)
	if err != nil {
		return err
	}

	var a struct {
		ServerTime types.MillisecondTimestamp `json:"serverTime"`
	}

	err = resp.DecodeJSON(&a)
	if err != nil {
		return err
	}

	c.timeOffset = currentTimestamp() - a.ServerTime.Time().UnixMilli()
	return nil
}

// newAuthenticatedRequest creates new http request for authenticated routes.
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

	if params == nil {
		params = url.Values{}
	}

	if c.recvWindow > 0 {
		params.Set("recvWindow", strconv.Itoa(c.recvWindow))
	}

	params.Set("timestamp", strconv.FormatInt(currentTimestamp()-c.timeOffset, 10))
	rawQuery := params.Encode()

	pathURL := c.BaseURL.ResolveReference(rel)
	body, err := castPayload(payload)
	if err != nil {
		return nil, err
	}

	toSign := rawQuery + string(body)
	signature := sign(c.Secret, toSign)

	// sv is the extra url parameters that we need to attach to the request
	sv := url.Values{}
	sv.Set("signature", signature)
	if rawQuery == "" {
		rawQuery = sv.Encode()
	} else {
		rawQuery = rawQuery + "&" + sv.Encode()
	}

	if rawQuery != "" {
		pathURL.RawQuery = rawQuery
	}

	req, err := http.NewRequestWithContext(ctx, method, pathURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// if our payload body is not an empty string
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	req.Header.Add("Accept", "application/json")

	// Build authentication headers
	req.Header.Add("X-MBX-APIKEY", c.Key)
	return req, nil
}

// sign uses sha256 to sign the payload with the given secret
func sign(secret, payload string) string {
	var sig = hmac.New(sha256.New, []byte(secret))
	_, err := sig.Write([]byte(payload))
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%x", sig.Sum(nil))
}

func currentTimestamp() int64 {
	return FormatTimestamp(time.Now())
}

// FormatTimestamp formats a time into Unix timestamp in milliseconds, as requested by Binance.
func FormatTimestamp(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
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

type APIResponse struct {
	Code    string          `json:"code"`
	Message string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}
