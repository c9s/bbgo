package amberdataapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// Do sends a request and decodes the envelope into out, mapping a non-2xx
// response to an *APIError.
func (c *RestClient) Do(req *http.Request, out any) error {
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Cap the read: a single response is capped at 10 MB server-side, so
	// anything much larger means something is wrong.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ParseAPIError(resp.StatusCode, body)
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("amberdata: decoding response: %w", err)
	}

	return nil
}
