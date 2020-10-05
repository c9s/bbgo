package max

import (
	"context"
	"fmt"
	"strings"

	maxapi "github.com/c9s/bbgo/exchange/max/maxapi"
	"github.com/c9s/bbgo/types"
)

type Exchange struct {
	client      *maxapi.RestClient
	key, secret string
}

func New(key, secret string) *Exchange {
	client := maxapi.NewRestClient(maxapi.ProductionAPIURL)
	client.Auth(key, secret)
	return &Exchange{
		client: client,
		key:    key,
		secret: secret,
	}
}

func (e *Exchange) NewStream() types.Stream {
	return NewStream(e.key, e.secret)
}

func (e *Exchange) SubmitOrder(ctx context.Context, order *types.SubmitOrder) error {
	orderType, err := toLocalOrderType(order.Type)
	if err != nil {
		return err
	}

	retOrder, err := e.client.OrderService.Create(
		order.Symbol,
		toLocalSideType(order.Side),
		order.Quantity,
		order.Price,
		string(orderType),
		maxapi.Options{})

	if err != nil {
		return err
	}

	logger.Infof("order created: %+v", retOrder)
	return err
}

// PlatformFeeCurrency
func (e *Exchange) PlatformFeeCurrency() string {
	return "max"
}

func toLocalCurrency(currency string) string {
	return strings.ToLower(currency)
}

func toLocalSideType(side types.SideType) string {
	return strings.ToLower(string(side))
}

func toLocalOrderType(orderType types.OrderType) (maxapi.OrderType, error) {
	switch orderType {
	case types.OrderTypeLimit:
		return maxapi.OrderTypeLimit, nil

	case types.OrderTypeMarket:
		return maxapi.OrderTypeMarket, nil
	}

	return "", fmt.Errorf("order type %s not supported", orderType)
}
