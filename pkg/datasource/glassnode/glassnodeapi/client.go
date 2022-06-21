package glassnodeapi

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/c9s/requestgen"
)

const defaultHTTPTimeout = time.Second * 15
const glassnodeBaseURL = "https://api.glassnode.com"

type RestClient struct {
	requestgen.BaseAPIClient

	apiKey string
}

func NewRestClient() *RestClient {
	u, err := url.Parse(glassnodeBaseURL)
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

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	// Attch API Key to header. https://docs.glassnode.com/basic-api/api-key#usage
	req.Header.Add("X-Api-Key", c.apiKey)

	return req, nil
}
