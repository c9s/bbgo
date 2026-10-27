package amberdataapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/c9s/requestgen"
)

const (
	// DefaultBaseURL is the market data host. Everything lives under /markets.
	DefaultBaseURL = "https://api.amberdata.com"

	// DefaultAPIVersion pins the response schema. It is optional to the server
	// but sent always, so a server-side default change cannot silently alter
	// field names underneath a running backfill.
	DefaultAPIVersion = "2023-09-30"

	// defaultHTTPTimeout exceeds the server's own 28 second limit, so a slow
	// query surfaces as the API's 504 — which the paginator handles by
	// shrinking the window — rather than as a client timeout.
	defaultHTTPTimeout = 35 * time.Second
)

// KeyTier is inferred from an API key's prefix and gives the documented rate
// limits, so a client can throttle itself correctly without being told.
type KeyTier struct {
	Name           string
	RequestsPerSec float64
	RequestsPerDay int
}

var keyTiers = map[string]KeyTier{
	"UAT": {Name: "trial", RequestsPerSec: 15, RequestsPerDay: 20_000},
	"UAO": {Name: "on-demand", RequestsPerSec: 20, RequestsPerDay: 250_000},
	"UAK": {Name: "enterprise", RequestsPerSec: 60, RequestsPerDay: 0},
}

// TierFromKey returns the documented limits for a key, and false when the prefix
// is not recognized.
func TierFromKey(apiKey string) (KeyTier, bool) {
	if len(apiKey) < 3 {
		return KeyTier{}, false
	}
	tier, ok := keyTiers[strings.ToUpper(apiKey[:3])]
	return tier, ok
}

// RestClient talks to the AmberData market data API.
//
// Authentication is a single static header. There is no signing, no timestamp
// and no nonce, so there is nothing to get wrong beyond sending the key.
type RestClient struct {
	requestgen.BaseAPIClient

	apiKey     string
	apiVersion string
}

func NewRestClient() *RestClient {
	u, err := url.Parse(DefaultBaseURL)
	if err != nil {
		// A constant that does not parse is a programming error.
		panic(err)
	}

	return &RestClient{
		BaseAPIClient: requestgen.BaseAPIClient{
			BaseURL:    u,
			HttpClient: &http.Client{Timeout: defaultHTTPTimeout},
		},
		apiVersion: DefaultAPIVersion,
	}
}

// Auth sets the API key.
func (c *RestClient) Auth(apiKey string) {
	// pragma: allowlist nextline secret
	c.apiKey = apiKey
}

// SetBaseURL overrides the host, for tests and for any future regional endpoint.
func (c *RestClient) SetBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	c.BaseURL = u
	return nil
}

// Tier returns the documented limits for the configured key.
func (c *RestClient) Tier() (KeyTier, bool) { return TierFromKey(c.apiKey) }

// NewAuthenticatedRequest builds a request with the three headers the API needs.
func (c *RestClient) NewAuthenticatedRequest(
	ctx context.Context, method, refURL string, params url.Values, payload any,
) (*http.Request, error) {
	req, err := c.NewRequest(ctx, method, refURL, params, payload)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	// Documented as required by the API.
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("api-version", c.apiVersion)
	req.Header.Set("x-api-key", c.apiKey)

	return req, nil
}

// NewCursorRequest builds a request for a pagination cursor URL.
//
// metadata.next is a complete absolute URL carrying an opaque compressed cursor.
// It must be re-issued verbatim with the same headers: the cursor cannot be
// reconstructed, and appending parameters to it is not documented to work.
func (c *RestClient) NewCursorRequest(ctx context.Context, cursorURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cursorURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("api-version", c.apiVersion)
	req.Header.Set("x-api-key", c.apiKey)

	return req, nil
}

// SendRequest overrides requestgen.BaseAPIClient.SendRequest so that every
// generated request builder inherits this package's error handling.
//
// The embedded implementation returns its own *requestgen.ErrResponse for a
// non-2xx response, which would lose the classification the paginator depends
// on: whether a failure means "halve the window" (the 10 MB cap, the 28 second
// timeout) or "stop" (a 403). Translating here means the generated Do methods
// need no special casing.
func (c *RestClient) SendRequest(req *http.Request) (*requestgen.Response, error) {
	resp, err := c.BaseAPIClient.SendRequest(req)
	if err == nil {
		return resp, nil
	}

	var errResponse *requestgen.ErrResponse
	if errors.As(err, &errResponse) && errResponse.Response != nil {
		return resp, ParseAPIError(errResponse.Response.StatusCode, errResponse.Body)
	}

	return resp, err
}
