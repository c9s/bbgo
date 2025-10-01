package maxapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/c9s/requestgen"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/envvar"
	"github.com/c9s/bbgo/pkg/util/apikey"
	"github.com/c9s/bbgo/pkg/version"
)

const (
	// ProductionAPIURL is the official MAX API v2 Endpoint
	ProductionAPIURL = "https://max-api.maicoin.com/api/v2"

	UserAgent = "bbgo/" + version.Version

	// 2018-09-01 08:00:00 +0800 CST
	TimestampSince = 1535760000

	maxAllowedNegativeTimeOffset = -20
)

var disableUserAgentHeader = false

var logger = log.WithField("exchange", "max")

var htmlTagPattern = regexp.MustCompile("<[/]?[a-zA-Z-]+.*?>")

// The following variables are used for nonce:

// globalTimeOffset is used for nonce
var globalTimeOffset int64 = 0

// globalServerTimestamp is used for storing the server timestamp, default to Now
var globalServerTimestamp = time.Now().Unix()

func init() {
	if val, ok := envvar.Bool("DISABLE_MAX_USER_AGENT_HEADER"); ok {
		disableUserAgentHeader = val
	}

	client := NewRestClientDefault()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			logger.Error("unable to update server time offset due to timeout")
			return
		default:

		}
		serverTime, err := client.NewGetTimestampRequest().Do(context.Background())
		if err != nil {
			logger.WithError(err).Error("unable to get max exchange server timestamp")
			continue
		} else if serverTime == nil {
			logger.Error("unable to get max exchange server timestamp: empty response")
			continue
		}

		globalServerTimestamp = int64(*serverTime)
		clientTime := time.Now()
		offset := globalServerTimestamp - clientTime.Unix()
		if offset < 0 {
			// avoid updating a negative offset: server time is before our local time
			if offset > maxAllowedNegativeTimeOffset {
				break
			}
			logger.Warnf("server time is behind local time, offset: %d", offset)
		}

		globalTimeOffset = offset
		logger.Infof("updated server time offset: %d", offset)
		break
	}
}

type RestClient struct {
	requestgen.BaseAPIClient

	APIKey, APISecret string

	SubAccount string

	PublicService     *PublicService
	RewardService     *RewardService
	WithdrawalService *WithdrawalService

	ApiKeyRotator *apikey.RoundTripBalancer

	apiKeyNonce map[string]int64

	mu sync.Mutex
}

func NewRestClientDefault() *RestClient {
	baseURL := ProductionAPIURL
	if override := os.Getenv("MAX_API_BASE_URL"); len(override) > 0 {
		baseURL = override
	}

	return NewRestClient(baseURL)
}

func NewRestClient(baseURL string) *RestClient {
	u, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}

	client := &RestClient{
		BaseAPIClient: requestgen.BaseAPIClient{
			HttpClient: core.HttpClient,
			BaseURL:    u,
		},

		apiKeyNonce: make(map[string]int64),
	}

	client.PublicService = &PublicService{client}
	client.RewardService = &RewardService{client}
	client.WithdrawalService = &WithdrawalService{client}

	if v, ok := envvar.Bool("MAX_ENABLE_API_KEY_ROTATION"); v && ok {
		loader := apikey.NewEnvKeyLoader("MAX_API_", "", "KEY", "SECRET")

		// this loads MAX_API_KEY_1, MAX_API_KEY_2, MAX_API_KEY_3, ...
		source, err := loader.Load(os.Environ())
		if err != nil {
			logger.Panic(err)
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
func (c *RestClient) Auth(key string, secret string) *RestClient {
	// pragma: allowlist nextline secret
	c.APIKey = key
	// pragma: allowlist nextline secret
	c.APISecret = secret

	return c
}

func (c *RestClient) SetSubAccount(subAccount string) *RestClient {
	c.SubAccount = subAccount
	return c
}

func (c *RestClient) GetNonce(apiKey string) int64 {
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

func (c *RestClient) SelectApiKey() (string, string) {
	apiKey := c.APIKey
	apiSecret := c.APISecret

	if c.ApiKeyRotator != nil {
		apiKey, apiSecret = c.ApiKeyRotator.Next().GetKeySecret()
	}

	return apiKey, apiSecret
}

// NewAuthenticatedRequest creates new http request for authenticated routes.
func (c *RestClient) NewAuthenticatedRequest(
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
	payload := map[string]interface{}{
		"nonce": nonce,
		"path":  c.BaseURL.ResolveReference(rel).Path,
	}

	switch d := data.(type) {
	case map[string]interface{}:
		for k, v := range d {
			payload[k] = v
		}
	default:
	}

	for k, vs := range params {
		k = strings.TrimSuffix(k, "[]")
		if len(vs) == 1 {
			payload[k] = vs[0]
		} else {
			payload[k] = vs
		}
	}

	p, err := castPayload(payload)
	if err != nil {
		return nil, err
	}

	req, err := c.NewRequest(ctx, m, refURL, params, p)
	if err != nil {
		return nil, err
	}

	encoded := base64.StdEncoding.EncodeToString(p)

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-MAX-ACCESSKEY", apiKey)
	req.Header.Add("X-MAX-PAYLOAD", encoded)
	req.Header.Add("X-MAX-SIGNATURE", signPayload(encoded, apiSecret))
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

type ErrorField struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse is the custom error type that is returned if the API returns an
// error.
type ErrorResponse struct {
	*requestgen.Response
	Err ErrorField `json:"error"`
}

func (r *ErrorResponse) Error() string {
	return fmt.Sprintf("%s %s: %d %d %s",
		r.Response.Response.Request.Method,
		r.Response.Response.Request.URL.String(),
		r.Response.Response.StatusCode,
		r.Err.Code,
		r.Err.Message,
	)
}

// ToErrorResponse tries to convert/parse the server response to the standard Error interface object
func ToErrorResponse(response *requestgen.Response) (errorResponse *ErrorResponse, err error) {
	errorResponse = &ErrorResponse{Response: response}

	contentType := response.Header.Get("content-type")
	switch contentType {
	case "text/json", "application/json", "application/json; charset=utf-8":
		var err = response.DecodeJSON(errorResponse)
		if err != nil {
			return errorResponse, errors.Wrapf(err, "failed to decode json for response: %d %s", response.StatusCode, string(response.Body))
		}
		return errorResponse, nil
	case "text/html":
		// convert 5xx error from the HTML page to the ErrorResponse
		errorResponse.Err.Message = htmlTagPattern.ReplaceAllLiteralString(string(response.Body), "")
		return errorResponse, nil
	case "text/plain":
		errorResponse.Err.Message = string(response.Body)
		return errorResponse, nil
	}

	return errorResponse, fmt.Errorf("unexpected response content type %s", contentType)
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
