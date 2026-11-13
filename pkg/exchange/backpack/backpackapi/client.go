package backpackapi

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/c9s/requestgen"
	"github.com/pkg/errors"
)

const defaultHTTPTimeout = 15 * time.Second

// RestBaseURL is the production REST endpoint.
// See https://docs.backpack.exchange/
const RestBaseURL = "https://api.backpack.exchange"

// WebSocketURL is the production websocket endpoint.
// It is defined here so that all the endpoint constants stay in one place.
const WebSocketURL = "wss://ws.backpack.exchange"

// defaultWindow is the default value of the X-Window header, in milliseconds.
// From the docs: "Time window in milliseconds that the request is valid for, default is 5000
// and maximum is 60000."
const defaultWindow uint64 = 5000

// maxWindow is the maximum accepted value of the X-Window header, in milliseconds.
const maxWindow uint64 = 60000

var (
	errNoApiKey    = errors.New("empty api key")
	errNoApiSecret = errors.New("empty api secret")
)

var parsedBaseURL *url.URL

func init() {
	u, err := url.Parse(RestBaseURL)
	if err != nil {
		panic(err)
	}

	parsedBaseURL = u
}

type RestClient struct {
	requestgen.BaseAPIClient

	// key is the base64 encoded ed25519 verifying (public) key, sent as the X-API-Key header
	// verbatim.
	key string

	// privateKey is derived from the base64 encoded 32-byte ed25519 seed (the api secret).
	privateKey ed25519.PrivateKey

	window uint64
}

func NewClient() *RestClient {
	return &RestClient{
		BaseAPIClient: requestgen.BaseAPIClient{
			BaseURL: parsedBaseURL,

			// use a dedicated http.Client instead of http.DefaultClient, so that swapping the
			// transport (for example in the http record/replay tests) does not leak into the
			// rest of the process.
			HttpClient: &http.Client{
				Timeout: defaultHTTPTimeout,
			},
		},
		window: defaultWindow,
	}
}

// Auth configures the api credentials.
//
// key is the base64 encoded ed25519 verifying key and secret is the base64 encoded 32-byte
// ed25519 seed, both as issued by the Backpack Exchange web console.
//
// Unlike the other exchanges in bbgo, Auth returns an error: the secret has to be decodable
// into an ed25519 seed, and failing early with a clear message is far easier to debug than the
// signature rejections that would follow.
func (c *RestClient) Auth(key, secret string) error {
	if len(key) == 0 {
		return errNoApiKey
	}

	if len(secret) == 0 {
		return errNoApiSecret
	}

	// pragma: allowlist nextline secret
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(secret))
	if err != nil {
		return errors.Wrap(err, "unable to base64 decode the api secret")
	}

	if len(seed) != ed25519.SeedSize {
		return fmt.Errorf("invalid api secret: expected a %d-byte ed25519 seed, got %d bytes",
			ed25519.SeedSize, len(seed))
	}

	privateKey := ed25519.NewKeyFromSeed(seed)

	// the api key is documented as the base64 encoded verifying key of the same keypair,
	// so a mismatch here means the key and the secret do not belong together.
	key = strings.TrimSpace(key)
	derivedKey := base64.StdEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey))
	if derivedKey != key {
		return errors.New("api key does not match the verifying key derived from the api secret")
	}

	c.key = key
	c.privateKey = privateKey
	return nil
}

// SetWindow overrides the X-Window value (in milliseconds) used for signing.
func (c *RestClient) SetWindow(window uint64) {
	if window > maxWindow {
		window = maxWindow
	}

	c.window = window
}

func (c *RestClient) getWindow() uint64 {
	if c.window == 0 {
		return defaultWindow
	}

	return c.window
}

// instructionClient binds a Backpack "instruction" to a RestClient.
//
// Backpack signs every authenticated request with an endpoint-specific instruction name, but
// requestgen's AuthenticatedRequestBuilder interface has nowhere to carry it. Each
// New<...>Request constructor therefore wraps the client with the instruction of its endpoint,
// keeping the instruction next to the endpoint it belongs to.
//
// NewRequest and SendRequest are promoted from the embedded *RestClient.
type instructionClient struct {
	*RestClient

	instruction Instruction
}

func (c *RestClient) withInstruction(instruction Instruction) requestgen.AuthenticatedAPIClient {
	return &instructionClient{RestClient: c, instruction: instruction}
}

func (c *instructionClient) NewAuthenticatedRequest(
	ctx context.Context, method, refURL string, params url.Values, payload interface{},
) (*http.Request, error) {
	return c.RestClient.newAuthenticatedRequest(ctx, method, refURL, params, payload, c.instruction)
}

// NewAuthenticatedRequest is defined so that *RestClient itself satisfies
// requestgen.AuthenticatedAPIClient. It has no instruction to sign with, so it always fails:
// authenticated requests must be built through withInstruction.
func (c *RestClient) NewAuthenticatedRequest(
	ctx context.Context, method, refURL string, params url.Values, payload interface{},
) (*http.Request, error) {
	return nil, errors.New("backpackapi: authenticated requests require an instruction, " +
		"build the request with RestClient.withInstruction")
}

func (c *RestClient) newAuthenticatedRequest(
	ctx context.Context, method, refURL string, params url.Values, payload interface{},
	instruction Instruction,
) (*http.Request, error) {
	if len(c.key) == 0 {
		return nil, errNoApiKey
	}

	if c.privateKey == nil {
		return nil, errNoApiSecret
	}

	timestamp := time.Now().UnixMilli()
	window := c.getWindow()

	signingString := buildSigningString(instruction, signingParams(params, payload), timestamp, window)
	signature := c.sign(signingString)

	req, err := c.BaseAPIClient.NewRequest(ctx, method, refURL, params, payload)
	if err != nil {
		return nil, err
	}

	setAuthHeaders(req, c.key, signature, timestamp, window)
	return req, nil
}

func (c *RestClient) sign(payload string) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(c.privateKey, []byte(payload)))
}

func setAuthHeaders(req *http.Request, key, signature string, timestamp int64, window uint64) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", key)
	req.Header.Set("X-Signature", signature)
	req.Header.Set("X-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Window", strconv.FormatUint(window, 10))
}

// signingParams returns the request parameters that participate in the signature, as an
// alphabetically ordered "key=value" slice.
//
// requestgen hands us the parameters in one of two places: query parameters for GET requests
// (params) and a map for the JSON body of everything else (payload). Only one of them is ever
// populated for a given request.
func signingParams(params url.Values, payload interface{}) []string {
	if m, ok := payload.(map[string]interface{}); ok && len(m) > 0 {
		pairs := make([]string, 0, len(m))
		for k, v := range m {
			pairs = append(pairs, k+"="+formatSigningValue(v))
		}

		sort.Strings(pairs)
		return pairs
	}

	if len(params) > 0 {
		pairs := make([]string, 0, len(params))
		for k := range params {
			pairs = append(pairs, k+"="+params.Get(k))
		}

		sort.Strings(pairs)
		return pairs
	}

	return nil
}

func formatSigningValue(v interface{}) string {
	switch tv := v.(type) {
	case string:
		return tv
	case bool:
		return strconv.FormatBool(tv)
	case float64:
		// values that went through a JSON round trip arrive as float64; format them without
		// an exponent so that large ids such as clientId stay in their integer form.
		return strconv.FormatFloat(tv, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(tv), 'f', -1, 32)
	}

	return fmt.Sprintf("%v", v)
}

// buildSigningString assembles the string that gets signed:
//
//	instruction=<name>&<params sorted alphabetically>&timestamp=<ms>&window=<ms>
//
// The window is always part of the signing string, even when the X-Window header is omitted.
func buildSigningString(instruction Instruction, params []string, timestamp int64, window uint64) string {
	var sb strings.Builder
	sb.WriteString(buildSigningStringPrefix(instruction, params))
	sb.WriteString("&timestamp=")
	sb.WriteString(strconv.FormatInt(timestamp, 10))
	sb.WriteString("&window=")
	sb.WriteString(strconv.FormatUint(window, 10))
	return sb.String()
}

// SendRequest overrides requestgen.BaseAPIClient.SendRequest to decode the Backpack error
// response body into a typed *APIError.
//
// Backpack does not wrap successful responses in an envelope, so unlike the other exchanges
// there is no response type with a Validate() method to hook into; the error has to be
// recognized at the HTTP layer instead.
func (c *RestClient) SendRequest(req *http.Request) (*requestgen.Response, error) {
	resp, err := c.BaseAPIClient.SendRequest(req)
	if err == nil {
		return resp, nil
	}

	if resp == nil || len(resp.Body) == 0 {
		return resp, err
	}

	var apiErr APIError
	if jsonErr := json.Unmarshal(resp.Body, &apiErr); jsonErr != nil || len(apiErr.Code) == 0 {
		return resp, err
	}

	apiErr.StatusCode = resp.StatusCode
	return resp, &apiErr
}

// APIError is the error response returned by the Backpack API.
type APIError struct {
	Code    ApiErrorCode `json:"code"`
	Message string       `json:"message"`

	// StatusCode is the HTTP status code, it is not part of the response body.
	StatusCode int `json:"-"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("backpack api error: code=%s, message=%s, status=%d", e.Code, e.Message, e.StatusCode)
}

// Ping calls GET /api/v1/ping. The endpoint replies with the plain text body "pong", which is
// not valid JSON, so it can not be generated by requestgen.
func (c *RestClient) Ping(ctx context.Context) error {
	req, err := c.NewRequest(ctx, http.MethodGet, "/api/v1/ping", nil, nil)
	if err != nil {
		return err
	}

	resp, err := c.SendRequest(req)
	if err != nil {
		return err
	}

	if body := strings.TrimSpace(resp.String()); body != "pong" {
		return fmt.Errorf("unexpected ping response: %q", body)
	}

	return nil
}

// GetServerTime calls GET /api/v1/time. The endpoint replies with a plain text unix
// millisecond timestamp.
func (c *RestClient) GetServerTime(ctx context.Context) (time.Time, error) {
	req, err := c.NewRequest(ctx, http.MethodGet, "/api/v1/time", nil, nil)
	if err != nil {
		return time.Time{}, err
	}

	resp, err := c.SendRequest(req)
	if err != nil {
		return time.Time{}, err
	}

	body := strings.TrimSpace(resp.String())
	ms, err := strconv.ParseInt(body, 10, 64)
	if err != nil {
		return time.Time{}, errors.Wrapf(err, "unable to parse the server time response: %q", body)
	}

	return time.UnixMilli(ms), nil
}
