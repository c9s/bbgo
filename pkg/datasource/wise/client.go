package wise

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

const (
	defaultHTTPTimeout = time.Second * 15
	defaultBaseURL     = "https://api.transferwise.com"
	sandboxBaseURL     = "https://api.sandbox.transferwise.tech"
)

type Client struct {
	requestgen.BaseAPIClient

	token string
}

func NewClient() *Client {
	u, err := url.Parse(defaultBaseURL)
	if err != nil {
		panic(err)
	}

	return &Client{
		BaseAPIClient: requestgen.BaseAPIClient{
			BaseURL: u,
			HttpClient: &http.Client{
				Timeout: defaultHTTPTimeout,
			},
		},
	}
}

func (c *Client) Auth(token string) {
	c.token = token
}

func (c *Client) NewAuthenticatedRequest(ctx context.Context, method, refURL string, params url.Values, payload interface{}) (*http.Request, error) {
	req, err := c.NewRequest(ctx, method, refURL, params, payload)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return req, nil
}

func (c *Client) QueryRate(ctx context.Context, source string, target string) ([]Rate, error) {
	req := c.NewRateRequest().Source(source).Target(target)
	return req.Do(ctx)
}

func (c *Client) QueryRateHistory(ctx context.Context, source string, target string, from time.Time, to time.Time, interval types.Interval) ([]Rate, error) {
	req := c.NewRateRequest().Source(source).Target(target).From(from).To(to)

	switch interval {
	case types.Interval1h:
		req.Group(GroupHour)
	case types.Interval1d:
		req.Group(GroupDay)
	case types.Interval1m:
		req.Group(GroupMinute)
	default:
		return nil, fmt.Errorf("unsupported interval: %s", interval)
	}

	return req.Do(ctx)
}
