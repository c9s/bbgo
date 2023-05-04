package v3

import (
	"github.com/c9s/requestgen"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
)

// create type alias
type WalletType = maxapi.WalletType
type OrderType = maxapi.OrderType

type Order = maxapi.Order
type Account = maxapi.Account

type Client struct {
	Client requestgen.AuthenticatedAPIClient
}
