package bitgetapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi"
)

type APIResponse = bitgetapi.APIResponse

type Client struct {
	Client requestgen.AuthenticatedAPIClient
}

func NewClient(client *bitgetapi.RestClient) *Client {
	return &Client{Client: client}
}
