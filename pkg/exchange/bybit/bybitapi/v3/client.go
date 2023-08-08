package v3

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
)

type APIResponse = bybitapi.APIResponse

type Client struct {
	Client requestgen.AuthenticatedAPIClient
}

func NewClient(client *bybitapi.RestClient) *Client {
	return &Client{Client: client}
}
