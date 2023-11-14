package bitgetapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi"
)

const (
	PrivateWebSocketURL = "wss://ws.bitget.com/v2/ws/private"
)

type APIResponse = bitgetapi.APIResponse

type Client struct {
	Client requestgen.AuthenticatedAPIClient
}

func NewClient(client *bitgetapi.RestClient) *Client {
	return &Client{Client: client}
}
