package v3

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
)

type APIResponse = bybitapi.APIResponse

type Client struct {
	Client requestgen.AuthenticatedAPIClient
}
