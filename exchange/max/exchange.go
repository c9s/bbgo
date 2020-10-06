package max

import (
	"context"
	"fmt"
	"strings"

	maxapi "github.com/c9s/bbgo/exchange/max/maxapi"
	"github.com/c9s/bbgo/types"
	"github.com/c9s/bbgo/util"
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

	req := e.client.OrderService.NewCreateOrderRequest().
		Market(order.Symbol).
		OrderType(string(orderType)).
		Side(toLocalSideType(order.Side)).
		Volume(order.QuantityString).
		Price(order.PriceString)

	retOrder, err := req.Do(ctx)
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

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	userInfo, err := e.client.AccountService.Me()
	if err != nil {
		return nil, err
	}

	var balances = make(types.BalanceMap)
	for _, a := range userInfo.Accounts {
		balances[toGlobalCurrency(a.Currency)] = types.Balance{
			Currency:  toGlobalCurrency(a.Currency),
			Available: util.MustParseFloat(a.Balance),
			Locked:    util.MustParseFloat(a.Locked),
		}
	}

	return &types.Account{
		MakerCommission: 15, // 0.15%
		TakerCommission: 15, // 0.15%
		Balances:        balances,
	}, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	accounts, err := e.client.AccountService.Accounts()
	if err != nil {
		return nil, err
	}

	var balances = make(types.BalanceMap)

	for _, a := range accounts {
		balances[toGlobalCurrency(a.Currency)] = types.Balance{
			Currency:  toGlobalCurrency(a.Currency),
			Available: util.MustParseFloat(a.Balance),
			Locked:    util.MustParseFloat(a.Locked),
		}
	}

	return balances, nil
}

func toGlobalCurrency(currency string) string {
	return strings.ToUpper(currency)
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
