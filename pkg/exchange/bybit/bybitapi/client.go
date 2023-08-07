package bybitapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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

const (
	defaultHTTPTimeout = time.Second * 15

	RestBaseURL         = "https://api.bybit.com"
	WsSpotPublicSpotUrl = "wss://stream.bybit.com/v5/public/spot"
	WsSpotPrivateUrl    = "wss://stream.bybit.com/v5/private"
)

// defaultRequestWindowMilliseconds specify how long an HTTP request is valid. It is also used to prevent replay attacks.
var defaultRequestWindowMilliseconds = fmt.Sprintf("%d", 5*time.Second.Milliseconds())

type RestClient struct {
	requestgen.BaseAPIClient

	key, secret string
}

func NewClient() (*RestClient, error) {
	u, err := url.Parse(RestBaseURL)
	if err != nil {
		return nil, err
	}

	return &RestClient{
		BaseAPIClient: requestgen.BaseAPIClient{
			BaseURL: u,
			HttpClient: &http.Client{
				Timeout: defaultHTTPTimeout,
			},
		},
	}, nil
}

func (c *RestClient) Auth(key, secret string) {
	c.key = key
	// pragma: allowlist secret
	c.secret = secret
}

// newAuthenticatedRequest creates new http request for authenticated routes.
func (c *RestClient) NewAuthenticatedRequest(ctx context.Context, method, refURL string, params url.Values, payload interface{}) (*http.Request, error) {
	if len(c.key) == 0 {
		return nil, errors.New("empty api key")
	}

	if len(c.secret) == 0 {
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

	t := time.Now().In(time.UTC)
	timestamp := strconv.FormatInt(t.UnixMilli(), 10)

	body, err := castPayload(payload)
	if err != nil {
		return nil, err
	}

	var signKey string
	switch method {
	case http.MethodPost:
		signKey = timestamp + c.key + defaultRequestWindowMilliseconds + string(body)
	case http.MethodGet:
		signKey = timestamp + c.key + defaultRequestWindowMilliseconds + rel.RawQuery
	default:
		return nil, fmt.Errorf("unexpected method: %s", method)
	}

	// See https://bybit-exchange.github.io/docs/v5/guide#create-a-request
	//
	// 1. timestamp + API key + (recv_window) + (queryString | jsonBodyString)
	// 2. Use the HMAC_SHA256 or RSA_SHA256 algorithm to sign the string in step 1, and convert it to a hex
	// string (HMAC_SHA256) / base64 (RSA_SHA256) to obtain the sign parameter.
	// 3. Append the sign parameter to request header, and send the HTTP request.
	signature := Sign(signKey, c.secret)

	req, err := http.NewRequestWithContext(ctx, method, pathURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-BAPI-API-KEY", c.key)
	req.Header.Add("X-BAPI-TIMESTAMP", timestamp)
	req.Header.Add("X-BAPI-SIGN", signature)
	req.Header.Add("X-BAPI-RECV-WINDOW", defaultRequestWindowMilliseconds)
	return req, nil
}

func Sign(payload string, secret string) string {
	var sig = hmac.New(sha256.New, []byte(secret))
	_, err := sig.Write([]byte(payload))
	if err != nil {
		return ""
	}

	return hex.EncodeToString(sig.Sum(nil))
}

func castPayload(payload interface{}) ([]byte, error) {
	if payload == nil {
		return nil, nil
	}

	switch v := payload.(type) {
	case string:
		return []byte(v), nil

	case []byte:
		return v, nil

	}
	return json.Marshal(payload)
}

/*
sample:

{
    "retCode": 0,
    "retMsg": "OK",
    "result": {
    },
    "retExtInfo": {},
    "time": 1671017382656
}
*/

type APIResponse struct {
	RetCode    uint            `json:"retCode"`
	RetMsg     string          `json:"retMsg"`
	Result     json.RawMessage `json:"result"`
	RetExtInfo json.RawMessage `json:"retExtInfo"`
	// Time is current timestamp (ms)
	Time types.MillisecondTimestamp `json:"time"`
}
