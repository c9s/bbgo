package glassnodeapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/c9s/requestgen"
)

const defaultHTTPTimeout = time.Second * 15
const glassnodeBaseURL = "https://api.glassnode.com"

type RestClient struct {
	BaseURL *url.URL
	Client  *http.Client

	apiKey string
}

func NewRestClient() *RestClient {
	u, err := url.Parse(glassnodeBaseURL)
	if err != nil {
		panic(err)
	}

	client := &RestClient{
		BaseURL: u,
		Client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}

	return client
}

func (c *RestClient) Auth(apiKey string) {
	c.apiKey = apiKey
}

func (c *RestClient) NewRequest(ctx context.Context, method string, refURL string, params url.Values, payload interface{}) (*http.Request, error) {
	rel, err := url.Parse(refURL)
	if err != nil {
		return nil, err
	}

	if params != nil {
		rel.RawQuery = params.Encode()
	}

	pathURL := c.BaseURL.ResolveReference(rel)

	body, err := castPayload(payload)
	if err != nil {
		return nil, err
	}

	return http.NewRequestWithContext(ctx, method, pathURL.String(), bytes.NewReader(body))
}

func (c *RestClient) SendRequest(req *http.Request) (*requestgen.Response, error) {
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	response, err := requestgen.NewResponse(resp)
	if err != nil {
		return response, err
	}

	// Check error, if there is an error, return the ErrorResponse struct type
	if response.IsError() {
		return response, errors.New(string(response.Body))
	}

	return response, nil
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
