package bfxapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/c9s/requestgen"
)

const productionBaseURL = "https://api.bitfinex.com/v2"

type Client struct {
	requestgen.BaseAPIClient

	// base members for synchronous API
	apiKey    string
	apiSecret string

	nonce *Nonce
}

// mock me in tests
func NewClient() *Client {
	u, err := url.Parse(productionBaseURL)
	if err != nil {
		panic(err)
	}

	return &Client{
		BaseAPIClient: requestgen.BaseAPIClient{
			BaseURL:    u,
			HttpClient: http.DefaultClient,
		},
		nonce: NewNonce(time.Now()),
	}
}

func (c *Client) Funding() *FundingService {
	return &FundingService{
		Client: c,
	}
}

func (c *Client) Auth(key string, secret string) *Client {
	c.apiKey = key
	c.apiSecret = secret
	return c
}

func (c *Client) sign(msg string) (string, error) {
	sig := hmac.New(sha512.New384, []byte(c.apiSecret))
	_, err := sig.Write([]byte(msg))
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(sig.Sum(nil)), nil
}

// NewAuthenticatedRequest creates new http request for authenticated routes.
// Create a new authenticated GET request with the given permission type and endpoint url
// For example permissionType = "r" and refUrl = "/orders" then the target endpoint will be
// https://api.bitfinex.com/v2/auth/r/orders/:Symbol
func (c *Client) NewAuthenticatedRequest(
	ctx context.Context, method, refURL string, params url.Values, payload interface{},
) (*http.Request, error) {
	body, err := castPayload(payload)
	if err != nil {
		return nil, err
	}

	return c.newAuthenticatedRequest(ctx, method, refURL, params, body)
}

// Create a new authenticated POST request with the given permission type,endpoint url and data (bytes) as the body
// For example permissionType = "r" and refUrl = "/orders" then the target endpoint will be
// https://api.bitfinex.com/v2/auth/r/orders/:Symbol
func (c *Client) newAuthenticatedRequest(
	ctx context.Context, method string, refURL string, params url.Values, data []byte,
) (*http.Request, error) {
	rel, err := url.Parse(refURL)
	if err != nil {
		return nil, err
	}

	pathURL := c.BaseURL.ResolveReference(rel)

	nonce := c.nonce.GetString()
	// sign("/api" + apiPath + nonce + data in JSON format)
	msg := "/api" + pathURL.Path + nonce + string(data)
	sig, err := c.sign(msg)
	if err != nil {
		return nil, err
	}

	rawQuery := params.Encode()
	if rawQuery != "" {
		pathURL.RawQuery = rawQuery
	}

	req, err := http.NewRequestWithContext(ctx, method, pathURL.String(), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("bfx-nonce", nonce)
	req.Header.Set("bfx-apikey", c.apiKey)
	req.Header.Set("bfx-signature", sig)
	return req, nil
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
