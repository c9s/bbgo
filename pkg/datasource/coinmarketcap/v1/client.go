package v1

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/c9s/requestgen"
)

const baseURL = "https://pro-api.coinmarketcap.com"
const defaultHTTPTimeout = time.Second * 15

type RestClient struct {
	requestgen.BaseAPIClient

	apiKey string
}

func New() *RestClient {
	u, err := url.Parse(baseURL)
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

func (c *RestClient) Auth(apiKey string) {
	// pragma: allowlist nextline secret
	c.apiKey = apiKey
}

func (c *RestClient) NewAuthenticatedRequest(ctx context.Context, method, refURL string, params url.Values, payload interface{}) (*http.Request, error) {
	req, err := c.NewRequest(ctx, method, refURL, params, payload)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	// Attach API Key to header. https://coinmarketcap.com/api/documentation/v1/#section/Authentication
	req.Header.Add("X-CMC_PRO_API_KEY", c.apiKey)

	return req, nil
}
