package max

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/c9s/bbgo/pkg/util"
	"github.com/c9s/bbgo/pkg/version"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	// ProductionAPIURL is the official MAX API v2 Endpoint
	ProductionAPIURL = "https://max-api.maicoin.com/api/v2"

	UserAgent = "bbgo/" + version.Version

	defaultHTTPTimeout = time.Second * 30

	// 2018-09-01 08:00:00 +0800 CST
	TimestampSince = 1535760000
)

var debugRequestDump = false
var debugMaxRequestPayload = false
var addUserAgentHeader = true

var httpTransportMaxIdleConnsPerHost = http.DefaultMaxIdleConnsPerHost
var httpTransportMaxIdleConns = 100
var httpTransportIdleConnTimeout = 90 * time.Second

func init() {
	debugMaxRequestPayload, _ = util.GetEnvVarBool("DEBUG_MAX_REQUEST_PAYLOAD")
	debugRequestDump, _ = util.GetEnvVarBool("DEBUG_MAX_REQUEST")
	addUserAgentHeader, _ = util.GetEnvVarBool("DISABLE_MAX_USER_AGENT_HEADER")

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

type RestClient struct {
	client *http.Client

	BaseURL *url.URL

	// Authentication
	APIKey    string
	APISecret string

	AccountService    *AccountService
	PublicService     *PublicService
	TradeService      *TradeService
	OrderService      *OrderService
	RewardService     *RewardService
	WithdrawalService *WithdrawalService
	// OrderBookService *OrderBookService
	// MaxTokenService  *MaxTokenService
	// MaxKLineService  *KLineService
	// CreditService    *CreditService
}

func NewRestClientWithHttpClient(baseURL string, httpClient *http.Client) *RestClient {
	u, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}

	var client = &RestClient{
		client:  httpClient,
		BaseURL: u,
	}

	client.AccountService = &AccountService{client}
	client.TradeService = &TradeService{client}
	client.PublicService = &PublicService{client}
	client.OrderService = &OrderService{client}
	client.RewardService = &RewardService{client}
	client.WithdrawalService = &WithdrawalService{client}

	// client.MaxTokenService = &MaxTokenService{client}
	client.initNonce()
	return client
}

func NewRestClient(baseURL string) *RestClient {
	// create an isolated http transport rather than the default one
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          httpTransportMaxIdleConns,
		MaxIdleConnsPerHost:   httpTransportMaxIdleConnsPerHost,
		IdleConnTimeout:       httpTransportIdleConnTimeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Timeout:   defaultHTTPTimeout,
		Transport: transport,
	}

	return NewRestClientWithHttpClient(baseURL, client)
}

// Auth sets api key and secret for usage is requests that requires authentication.
func (c *RestClient) Auth(key string, secret string) *RestClient {
	c.APIKey = key
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

// NewRequest create new API request. Relative url can be provided in refURL.
func (c *RestClient) newRequest(method string, refURL string, params url.Values, body []byte) (*http.Request, error) {
	rel, err := url.Parse(refURL)
	if err != nil {
		return nil, err
	}
	if params != nil {
		rel.RawQuery = params.Encode()
	}
	var req *http.Request
	u := c.BaseURL.ResolveReference(rel)

	req, err = http.NewRequest(method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	if addUserAgentHeader {
		req.Header.Add("User-Agent", UserAgent)
	}
	return req, nil
}

// newAuthenticatedRequest creates new http request for authenticated routes.
func (c *RestClient) newAuthenticatedRequest(m string, refURL string, data interface{}, rel *url.URL) (*http.Request, error) {
	var err error
	if rel == nil {
		rel, err = url.Parse(refURL)
		if err != nil {
			return nil, err
		}
	}

	var p []byte

	switch d := data.(type) {

	case nil:
		payload := map[string]interface{}{
			"nonce": c.getNonce(),
			"path":  c.BaseURL.ResolveReference(rel).Path,
		}
		p, err = json.Marshal(payload)

	case map[string]interface{}:
		payload := map[string]interface{}{
			"nonce": c.getNonce(),
			"path":  c.BaseURL.ResolveReference(rel).Path,
		}

		for k, v := range d {
			payload[k] = v
		}

		p, err = json.Marshal(payload)

	default:
		params, err := getPrivateRequestParamsObject(data)
		if err != nil {
			return nil, errors.Wrapf(err, "unsupported payload type: %T", d)
		}

		params.Nonce = c.getNonce()
		params.Path = c.BaseURL.ResolveReference(rel).Path

		p, err = json.Marshal(d)
	}

	if debugMaxRequestPayload {
		log.Infof("request payload: %s", p)
	}

	if err != nil {
		return nil, err
	}

	if len(c.APIKey) == 0 {
		return nil, errors.New("empty api key")
	}

	if len(c.APISecret) == 0 {
		return nil, errors.New("empty api secret")
	}

	req, err := c.newRequest(m, refURL, nil, p)
	if err != nil {
		return nil, err
	}

	encoded := base64.StdEncoding.EncodeToString(p)

	req.Header.Add("Content-Type", "application/json")
	// accept is not necessary
	// req.Header.Add("Accept", "application/json")
	req.Header.Add("X-MAX-ACCESSKEY", c.APIKey)
	req.Header.Add("X-MAX-PAYLOAD", encoded)
	req.Header.Add("X-MAX-SIGNATURE", signPayload(encoded, c.APISecret))

	if debugRequestDump {
		dump, err2 := httputil.DumpRequestOut(req, true)
		if err2 != nil {
			log.Errorf("dump request error: %v", err2)
		} else {
			fmt.Printf("REQUEST:\n%s", dump)
		}
	}

	return req, nil
}

func getPrivateRequestParamsObject(v interface{}) (*PrivateRequestParams, error) {
	vt := reflect.ValueOf(v)

	if vt.Kind() == reflect.Ptr {
		vt = vt.Elem()
	}

	if vt.Kind() != reflect.Struct {
		return nil, errors.New("reflect error: given object is not a struct" + vt.Kind().String())
	}

	if !vt.CanSet() {
		return nil, errors.New("reflect error: can not set object")
	}

	field := vt.FieldByName("PrivateRequestParams")
	if !field.IsValid() {
		return nil, errors.New("reflect error: field PrivateRequestParams not found")
	}

	if field.IsNil() {
		field.Set(reflect.ValueOf(&PrivateRequestParams{}))
	}

	params, ok := field.Interface().(*PrivateRequestParams)
	if !ok {
		return nil, errors.New("reflect error: failed to cast value to *PrivateRequestParams")
	}

	return params, nil
}

func signPayload(payload string, secret string) string {
	var sig = hmac.New(sha256.New, []byte(secret))
	_, err := sig.Write([]byte(payload))
	if err != nil {
		return ""
	}
	return hex.EncodeToString(sig.Sum(nil))
}

func (c *RestClient) Do(req *http.Request) (resp *http.Response, err error) {
	return c.client.Do(req)
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
		errorResponse, err := ToErrorResponse(response)
		if err != nil {
			return response, err
		}
		return response, errorResponse
	}

	return response, nil
}

func (c *RestClient) sendAuthenticatedRequest(m string, refURL string, data map[string]interface{}) (*util.Response, error) {
	req, err := c.newAuthenticatedRequest(m, refURL, data, nil)
	if err != nil {
		return nil, err
	}
	response, err := c.sendRequest(req)
	if err != nil {
		return nil, err
	}
	return response, err
}

// FIXME: should deprecate the polling usage from the websocket struct
func (c *RestClient) GetTrades(market string, lastTradeID int64) ([]byte, error) {
	params := url.Values{}
	params.Add("market", market)
	if lastTradeID > 0 {
		params.Add("from", strconv.Itoa(int(lastTradeID)))
	}

	return c.get("/trades", params)
}

// get sends GET http request to the api endpoint, the urlPath must start with a slash '/'
func (c *RestClient) get(urlPath string, values url.Values) ([]byte, error) {
	var reqURL = c.BaseURL.String() + urlPath

	// Create request
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("could not init request: %s", err.Error())
	}

	req.URL.RawQuery = values.Encode()
	req.Header.Add("User-Agent", UserAgent)

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not execute request: %s", err.Error())
	}
	defer resp.Body.Close()

	// Load request
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read response: %s", err.Error())
	}

	return body, nil
}

// ErrorResponse is the custom error type that is returned if the API returns an
// error.
type ErrorField struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	*util.Response
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
func ToErrorResponse(response *util.Response) (errorResponse *ErrorResponse, err error) {
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
	}

	return errorResponse, fmt.Errorf("unexpected response content type %s", contentType)
}
