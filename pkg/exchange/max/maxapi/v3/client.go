package v3

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/envvar"
	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/util/apikey"
	"github.com/c9s/bbgo/pkg/version"
	"github.com/c9s/requestgen"
)

var log = logrus.WithFields(logrus.Fields{
	"exchange": "max",
	"api":      "v3",
})

const (
	// ProductionAPIURL is the official MAX API v2 Endpoint
	ProductionAPIURL = "https://max-api.maicoin.com/api/v3"

	UserAgent = "bbgo/" + version.Version

	defaultHTTPTimeout = time.Second * 60

	// 2018-09-01 08:00:00 +0800 CST
	TimestampSince = 1535760000

	maxAllowedNegativeTimeOffset = -20
)

// The following variables are used for nonce.

// ServerTimeOffset is used for nonce
var ServerTimeOffset int64 = 0

// ServerTimestamp is used for storing the server timestamp, default to Now
var ServerTimestamp = time.Now().Unix()

// reqCount is used for nonce, this variable counts the API request count.
var reqCount uint64 = 1

// globalTimeOffset is used for nonce
var globalTimeOffset int64 = 0

var disableUserAgentHeader = false

// create type alias
type WalletType = maxapi.WalletType
type OrderByType = maxapi.OrderByType
type OrderType = maxapi.OrderType

type Order = maxapi.Order
type Account = maxapi.Account

func init() {
	// update the server time offset in the background if API key is set
	if _, ok := os.LookupEnv("MAX_API_KEY"); ok {
		UpdateServerTimeAndOffset(context.Background())
	}

}

// UpdateServerTimeAndOffset updates the global server time and offset variables
// by querying the MAX server timestamp API endpoint.
// This function will keep retrying until it succeeds or the context is done.
func UpdateServerTimeAndOffset(baseCtx context.Context) {
	v3client := NewClient()
	ctx, cancel := context.WithTimeout(baseCtx, time.Minute)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			log.Errorf("unable to update server time offset due to timeout")
			return
		default:
		}

		resp, err := v3client.NewGetTimestampRequest().Do(ctx)
		if err != nil {
			log.WithError(err).Errorf("unable to get max exchange server timestamp")
			continue
		}

		clientTime := time.Now()
		serverTime := time.Unix(resp.TimestampInSeconds, 0)
		offset := serverTime.Unix() - clientTime.Unix()

		if offset < 0 {
			// avoid updating a negative offset: server time is before our local time
			if offset > maxAllowedNegativeTimeOffset {
				return
			}

			// if the offset is greater than 15 seconds, we should restart
			log.Panicf("max exchange server timestamp offset %d is less than the negative offset %d", offset, maxAllowedNegativeTimeOffset)
		}

		atomic.StoreInt64(&ServerTimestamp, serverTime.Unix())
		atomic.StoreInt64(&ServerTimeOffset, offset)
		log.Infof("max exchange server timestamp offset %d", offset)
		return
	}
}

type Client struct {
	requestgen.BaseAPIClient

	APIKey, APISecret string

	SubAccount string

	ApiKeyRotator *apikey.RoundTripBalancer

	apiKeyNonce map[string]int64

	mu sync.Mutex

	MarginService     *MarginService
	SubAccountService *SubAccountService
}

func NewClient() *Client {
	baseURL := ProductionAPIURL
	if override := os.Getenv("MAX_API_BASE_URL"); len(override) > 0 {
		baseURL = override
	}
	client := newClient(baseURL)
	return client
}

func newClient(baseURL string) *Client {
	u, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}
	client := &Client{
		BaseAPIClient: requestgen.BaseAPIClient{
			HttpClient: core.HttpClient,
			BaseURL:    u,
		},

		apiKeyNonce: make(map[string]int64),
	}
	client.MarginService = &MarginService{Client: client}
	client.SubAccountService = &SubAccountService{Client: client}

	if v, ok := envvar.Bool("MAX_ENABLE_API_KEY_ROTATION"); v && ok {
		loader := apikey.NewEnvKeyLoader("MAX_API_", "", "KEY", "SECRET")

		// this loads MAX_API_KEY_1, MAX_API_KEY_2, MAX_API_KEY_3, ...
		source, err := loader.Load(os.Environ())
		if err != nil {
			log.Panic(err)
		}

		// load the original one as index 0
		if client.APIKey != "" && client.APISecret != "" {
			source.Add(apikey.Entry{Index: 0, Key: client.APIKey, Secret: client.APISecret})
		}

		client.ApiKeyRotator = apikey.NewRoundTripBalancer(source)
	}

	return client
}

// Auth sets api key and secret for usage is requests that requires authentication.
func (c *Client) Auth(key string, secret string) *Client {
	// pragma: allowlist nextline secret
	c.APIKey = key
	// pragma: allowlist nextline secret
	c.APISecret = secret

	return c
}

// NewAuthenticatedRequest creates new http request for authenticated routes.
func (c *Client) NewAuthenticatedRequest(
	ctx context.Context, m string, refURL string, params url.Values, data interface{},
) (*http.Request, error) {
	apiKey, apiSecret := c.SelectApiKey()

	if len(apiKey) == 0 {
		return nil, errors.New("empty api key")
	}

	if len(apiSecret) == 0 {
		return nil, errors.New("empty api secret")
	}

	rel, err := url.Parse(refURL)
	if err != nil {
		return nil, err
	}

	nonce := c.GetNonce(apiKey)
	apiPath := c.BaseURL.ResolveReference(rel).Path

	// build query params (always include nonce)
	queryParams := url.Values{}
	queryParams.Add("nonce", strconv.FormatInt(nonce, 10))
	addValuesToQuery(queryParams, params)

	// for GET requests, move data map into query parameters
	if m == "GET" {
		if d, ok := data.(map[string]interface{}); ok {
			for k, v := range d {
				queryParams.Add(k, fmt.Sprintf("%v", v))
			}
		}
	}

	payload, payloadToSign := buildPayloads(nonce, apiPath, params, data, m)

	// encode and sign payloadToSign
	payloadToSignBytes, err := json.Marshal(payloadToSign)
	if err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(payloadToSignBytes)
	signature := signPayload(encoded, apiSecret)

	// marshal actual request body payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := c.NewRequest(ctx, m, refURL, queryParams, payloadBytes)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-MAX-ACCESSKEY", apiKey)
	req.Header.Add("X-MAX-PAYLOAD", encoded)
	req.Header.Add("X-MAX-SIGNATURE", signature)
	if c.SubAccount != "" {
		req.Header.Add("X-Sub-Account", c.SubAccount)
	}

	if disableUserAgentHeader {
		req.Header.Set("USER-AGENT", "")
		req.Header.Del("User-Agent")
	} else {
		req.Header.Set("USER-AGENT", UserAgent)
	}

	if false {
		out, _ := httputil.DumpRequestOut(req, true)
		fmt.Println(string(out))
	}

	return req, nil
}

func (c *Client) SelectApiKey() (string, string) {
	apiKey := c.APIKey
	apiSecret := c.APISecret

	if c.ApiKeyRotator != nil {
		apiKey, apiSecret = c.ApiKeyRotator.Next().GetKeySecret()
	}

	return apiKey, apiSecret
}

func (c *Client) GetNonce(apiKey string) int64 {
	// nonce 是以正整數表示的時間戳記，代表了從 Unix epoch 到當前時間所經過的毫秒數(ms)。
	// nonce 與伺服器的時間差不得超過正負30秒，每個 nonce 只能使用一次。
	c.mu.Lock()
	now := time.Now()
	seconds := now.Unix()
	offset := atomic.LoadInt64(&globalTimeOffset)
	next := (seconds + offset) * 1000
	if last, ok := c.apiKeyNonce[apiKey]; ok {
		if next <= last {
			next = last + 1
		}
	}

	c.apiKeyNonce[apiKey] = next
	c.mu.Unlock()

	// (seconds+offset)*1000 -> convert seconds to milliseconds
	return next
}

func (c *Client) SetSubAccount(subAccount string) *Client {
	c.SubAccount = subAccount
	return c
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

	body, err := json.Marshal(payload)
	return body, err
}

func signPayload(payload string, secret string) string {
	var sig = hmac.New(sha256.New, []byte(secret))
	_, err := sig.Write([]byte(payload))
	if err != nil {
		return ""
	}
	return hex.EncodeToString(sig.Sum(nil))
}

// addValuesToQuery appends all params into a query url.Values object.
func addValuesToQuery(dst url.Values, src url.Values) {
	for k, vs := range src {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}

// buildPayloads constructs the payload and payloadToSign maps according to MAX API rules.
// payload contains the actual request body (nonce + post-params for non-GET).
// payloadToSign contains fields used for signature (nonce, path and params).
func buildPayloads(nonce int64, apiPath string, params url.Values, data interface{}, method string) (map[string]interface{}, map[string]interface{}) {
	payload := map[string]interface{}{}
	payloadToSign := map[string]interface{}{"nonce": nonce, "path": apiPath}

	// include params (query) into payloadToSign; trim array suffix for signing key
	for k, vs := range params {
		kTrim := strings.TrimSuffix(k, "[]")
		if len(vs) == 1 {
			payloadToSign[kTrim] = vs[0]
		} else {
			payloadToSign[kTrim] = vs
		}
	}

	// for non-GET requests, include post-parameters into both payload and payloadToSign
	if method != "GET" {
		if d, ok := data.(map[string]interface{}); ok {
			for k, v := range d {
				payload[k] = v
				payloadToSign[k] = v
			}
		}
	}

	return payload, payloadToSign
}
