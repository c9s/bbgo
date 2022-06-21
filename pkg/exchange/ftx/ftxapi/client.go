package ftxapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Result
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Result
//go:generate -command DeleteRequest requestgen -method DELETE -responseType .APIResponse -responseDataField Result

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/c9s/requestgen"
	"github.com/pkg/errors"
)

const defaultHTTPTimeout = time.Second * 15
const RestBaseURL = "https://ftx.com/api"

type APIResponse struct {
	Success     bool            `json:"success"`
	Result      json.RawMessage `json:"result,omitempty"`
	HasMoreData bool            `json:"hasMoreData,omitempty"`
}

type RestClient struct {
	BaseURL *url.URL

	client *http.Client

	Key, Secret, subAccount string

	/*
		AccountService    *AccountService
		MarketDataService *MarketDataService
		TradeService      *TradeService
		BulletService     *BulletService
	*/
}

func NewClient() *RestClient {
	u, err := url.Parse(RestBaseURL)
	if err != nil {
		panic(err)
	}

	client := &RestClient{
		BaseURL: u,
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}

	/*
		client.AccountService = &AccountService{client: client}
		client.MarketDataService = &MarketDataService{client: client}
		client.TradeService = &TradeService{client: client}
		client.BulletService = &BulletService{client: client}
	*/
	return client
}

func (c *RestClient) Auth(key, secret, subAccount string) {
	c.Key = key
	// pragma: allowlist nextline secret
	c.Secret = secret
	c.subAccount = subAccount
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

	if params != nil {
		rel.RawQuery = params.Encode()
	}

	// pathURL is for sending request
	pathURL := c.BaseURL.ResolveReference(rel)

	// path here is used for auth header
	path := pathURL.Path
	if rel.RawQuery != "" {
		path += "?" + rel.RawQuery
	}

	body, err := castPayload(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, pathURL.String(), bytes.NewReader(body))
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
	millisecondTs := time.Now().UnixNano() / int64(time.Millisecond)
	ts := strconv.FormatInt(millisecondTs, 10)
	p := ts + method + path + string(body)
	signature := sign(c.Secret, p)
	req.Header.Set("FTX-KEY", c.Key)
	req.Header.Set("FTX-SIGN", signature)
	req.Header.Set("FTX-TS", ts)
	if c.subAccount != "" {
		req.Header.Set("FTX-SUBACCOUNT", c.subAccount)
	}
}

// sign uses sha256 to sign the payload with the given secret
func sign(secret, payload string) string {
	var sig = hmac.New(sha256.New, []byte(secret))
	_, err := sig.Write([]byte(payload))
	if err != nil {
		return ""
	}

	return hex.EncodeToString(sig.Sum(nil))
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
