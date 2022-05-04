package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/c9s/requestgen"
)

const baseURL = "https://pro-api.coinmarketcap.com"

type RestClient struct {
	BaseURL *url.URL
	Client  *http.Client

	apiKey string
}

func New() *RestClient {
	u, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}

	return &RestClient{
		BaseURL: u,
		Client:  &http.Client{},
	}
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

	fmt.Println(pathURL.String())

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

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-CMC_PRO_API_KEY", c.apiKey)

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
