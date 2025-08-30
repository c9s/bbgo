package v3

import (
	"github.com/c9s/requestgen"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
)

// create type alias
type WalletType = maxapi.WalletType
type OrderByType = maxapi.OrderByType
type OrderType = maxapi.OrderType

type Order = maxapi.Order
type Account = maxapi.Account

type Client struct {
	Client requestgen.AuthenticatedAPIClient

	MarginService     *MarginService
	SubAccountService *SubAccountService
}

func NewClient(baseClient *maxapi.RestClient) *Client {
	client := &Client{
		Client:            baseClient,
		MarginService:     &MarginService{Client: baseClient},
		SubAccountService: &SubAccountService{Client: baseClient},
	}
	return client
}
