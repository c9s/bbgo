package max

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/c9s/requestgen"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/util"
	"github.com/c9s/bbgo/pkg/version"
)

const (
	// ProductionAPIURL is the official MAX API v2 Endpoint
	ProductionAPIURL = "https://max-api.maicoin.com/api/v2"

	UserAgent = "bbgo/" + version.Version

	defaultHTTPTimeout = time.Second * 60

	// 2018-09-01 08:00:00 +0800 CST
	TimestampSince = 1535760000
)

var httpTransportMaxIdleConnsPerHost = http.DefaultMaxIdleConnsPerHost
var httpTransportMaxIdleConns = 100
var httpTransportIdleConnTimeout = 90 * time.Second

func init() {

	if val, ok := util.GetEnvVarInt("HTTP_TRANSPORT_MAX_IDLE_CONNS_PER_HOST"); ok {
		httpTransportMaxIdleConnsPerHost = val
	}

	if val, ok := util.GetEnvVarInt("HTTP_TRANSPORT_MAX_IDLE_CONNS"); ok {
		httpTransportMaxIdleConns = val
	}
	if val, ok := util.GetEnvVarDuration("HTTP_TRANSPORT_IDLE_CONN_TIMEOUT"); ok {
		httpTransportIdleConnTimeout = val
	}
}

var logger = log.WithField("exchange", "max")

var htmlTagPattern = regexp.MustCompile("<[/]?[a-zA-Z-]+.*?>")

// The following variables are used for nonce.

// timeOffset is used for nonce
var timeOffset int64 = 0

// serverTimestamp is used for storing the server timestamp, default to Now
var serverTimestamp = time.Now().Unix()

// reqCount is used for nonce, this variable counts the API request count.
var reqCount int64 = 1

// create an isolated http httpTransport rather than the default one
var httpTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          httpTransportMaxIdleConns,
	MaxIdleConnsPerHost:   httpTransportMaxIdleConnsPerHost,
	IdleConnTimeout:       httpTransportIdleConnTimeout,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

var defaultHttpClient = &http.Client{
	Timeout:   defaultHTTPTimeout,
	Transport: httpTransport,
}

type RestClient struct {
	requestgen.BaseAPIClient

	APIKey, APISecret string

	AccountService    *AccountService
	PublicService     *PublicService
	TradeService      *TradeService
	OrderService      *OrderService
	RewardService     *RewardService
	WithdrawalService *WithdrawalService
}

func NewRestClient(baseURL string) *RestClient {
	u, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}

	var client = &RestClient{
		BaseAPIClient: requestgen.BaseAPIClient{
			HttpClient: defaultHttpClient,
			BaseURL:    u,
		},
	}

	client.AccountService = &AccountService{client}
	client.TradeService = &TradeService{client}
	client.PublicService = &PublicService{client}
	client.OrderService = &OrderService{client}
	client.RewardService = &RewardService{client}
	client.WithdrawalService = &WithdrawalService{client}

	// defaultHttpClient.MaxTokenService = &MaxTokenService{defaultHttpClient}
	client.initNonce()
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

func (c *RestClient) initNonce() {
	var clientTime = time.Now()
	var err error
	serverTimestamp, err = c.PublicService.Timestamp()
	if err != nil {
		logger.WithError(err).Panic("failed to sync timestamp with max")
	}

	timeOffset = serverTimestamp - clientTime.Unix()
	logger.Infof("loaded max server timestamp: %d offset=%d", serverTimestamp, timeOffset)
}

func (c *RestClient) getNonce() int64 {
	// nonce 是以正整數表示的時間戳記，代表了從 Unix epoch 到當前時間所經過的毫秒數(ms)。
	// nonce 與伺服器的時間差不得超過正負30秒，每個 nonce 只能使用一次。
	var seconds = time.Now().Unix()
	var rc = atomic.AddInt64(&reqCount, 1)
	return (seconds+timeOffset)*1000 - 1 + int64(math.Mod(float64(rc), 1000.0))
}

func (c *RestClient) NewAuthenticatedRequest(ctx context.Context, m string, refURL string, params url.Values, payload interface{}) (*http.Request, error) {
	return c.newAuthenticatedRequest(ctx, m, refURL, params, payload, nil)
}

// newAuthenticatedRequest creates new http request for authenticated routes.
func (c *RestClient) newAuthenticatedRequest(ctx context.Context, m string, refURL string, params url.Values, data interface{}, rel *url.URL) (*http.Request, error) {
	if len(c.APIKey) == 0 {
		return nil, errors.New("empty api key")
	}

	if len(c.APISecret) == 0 {
		return nil, errors.New("empty api secret")
	}

	var err error
	if rel == nil {
		rel, err = url.Parse(refURL)
		if err != nil {
			return nil, err
		}
	}

	var p []byte
	var payload = map[string]interface{}{
		"nonce": c.getNonce(),
		"path":  c.BaseURL.ResolveReference(rel).Path,
	}

	switch d := data.(type) {
	case map[string]interface{}:
		for k, v := range d {
			payload[k] = v
		}
	}

	for k, vs := range params {
		k = strings.TrimSuffix(k, "[]")
		if len(vs) == 1 {
			payload[k] = vs[0]
		} else {
			payload[k] = vs
		}
	}

	p, err = castPayload(payload)
	if err != nil {
		return nil, err
	}

	req, err := c.NewRequest(ctx, m, refURL, params, p)
	if err != nil {
		return nil, err
	}

	encoded := base64.StdEncoding.EncodeToString(p)

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-MAX-ACCESSKEY", c.APIKey)
	req.Header.Add("X-MAX-PAYLOAD", encoded)
	req.Header.Add("X-MAX-SIGNATURE", signPayload(encoded, c.APISecret))

	return req, nil
}

func (c *RestClient) sendAuthenticatedRequest(m string, refURL string, data map[string]interface{}) (*requestgen.Response, error) {
	req, err := c.newAuthenticatedRequest(nil, m, refURL, nil, data, nil)
	if err != nil {
		return nil, err
	}

	return c.SendRequest(req)
}

// ErrorResponse is the custom error type that is returned if the API returns an
// error.
type ErrorField struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

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
