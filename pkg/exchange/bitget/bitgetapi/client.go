package bitgetapi

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
	"strconv"
	"strings"
	"time"

	"github.com/c9s/requestgen"
	"github.com/pkg/errors"
)

const defaultHTTPTimeout = time.Second * 15
const RestBaseURL = "https://api.bitget.com"
const PublicWebSocketURL = "wss://ws.bitget.com/spot/v1/stream"
const PrivateWebSocketURL = "wss://ws.bitget.com/spot/v1/stream"

type RestClient struct {
	requestgen.BaseAPIClient

	key, secret, passphrase string
}

func NewClient() *RestClient {
	u, err := url.Parse(RestBaseURL)
	if err != nil {
		panic(err)
	}

	return &RestClient{
		BaseAPIClient: requestgen.BaseAPIClient{
			BaseURL: u,
			HttpClient: &http.Client{
				Timeout: defaultHTTPTimeout,
			},
		},
	}
}

func (c *RestClient) Auth(key, secret, passphrase string) {
	c.key = key
	c.secret = secret
	c.passphrase = passphrase
}

// newAuthenticatedRequest creates new http request for authenticated routes.
func (c *RestClient) NewAuthenticatedRequest(
	ctx context.Context, method, refURL string, params url.Values, payload interface{},
) (*http.Request, error) {
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

	// See https://bitgetlimited.github.io/apidoc/en/spot/#signature
	// Sign(
	//    timestamp +
	// 	  method.toUpperCase() +
	// 	  requestPath + "?" + queryString +
	// 	  body **string
	// )
	// (+ means string concat) encrypt by **HMAC SHA256 **algorithm, and encode the encrypted result through **BASE64.

	// set location to UTC so that it outputs "2020-12-08T09:08:57.715Z"
	t := time.Now().In(time.UTC)
	timestamp := strconv.FormatInt(t.UnixMilli(), 10)

	body, err := castPayload(payload)
	if err != nil {
		return nil, err
	}

	signKey := timestamp + strings.ToUpper(method) + path + string(body)
	signature := Sign(signKey, c.secret)

	req, err := http.NewRequestWithContext(ctx, method, pathURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("ACCESS-KEY", c.key)
	req.Header.Add("ACCESS-SIGN", signature)
	req.Header.Add("ACCESS-TIMESTAMP", timestamp)
	req.Header.Add("ACCESS-PASSPHRASE", c.passphrase)
	req.Header.Add("X-CHANNEL-API-CODE", "7575765263")
	return req, nil
}

func Sign(payload string, secret string) string {
	var sig = hmac.New(sha256.New, []byte(secret))
	_, err := sig.Write([]byte(payload))
	if err != nil {
		return ""
	}

	return base64.StdEncoding.EncodeToString(sig.Sum(nil))
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
	    "code": "00000",
	    "msg": "success",
	    "data": {
	        "user_id": "714229403",
	        "inviter_id": "682221498",
	        "ips": "172.23.88.91",
	        "authorities": [
	            "trade",
	            "readonly"
	        ],
	        "parentId":"566624801",
	        "trader":false
	    }
	}
*/

type APIResponse struct {
	Code    string          `json:"code"`
	Message string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

func (a APIResponse) Validate() error {
	// v1, v2 use the same success code.
	// https://www.bitget.com/api-doc/spot/error-code/restapi
	// https://bitgetlimited.github.io/apidoc/en/mix/#restapi-error-codes
	if a.Code != "00000" {
		return a.Error()
	}
	return nil
}

func (a APIResponse) Error() error {
	return fmt.Errorf("code: %s, msg: %s, data: %q", a.Code, a.Message, a.Data)
}
