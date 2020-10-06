package max

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

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
	return toGlobalCurrency("MAX")
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

func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) (trades []types.Trade, err error) {
	req := e.client.TradeService.NewPrivateTradeRequest()
	req.Market(symbol)

	if options.Limit > 0 {
		req.Limit(options.Limit)
	}

	if options.LastTradeID > 0 {
		req.From(options.LastTradeID)
	}

	remoteTrades, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	for _, t := range remoteTrades {
		localTrade, err := convertRemoteTrade(t)
		if err != nil {
			logger.WithError(err).Errorf("can not convert binance trade: %+v", t)
			continue
		}
		trades = append(trades, *localTrade)
	}

	return trades, nil
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

func toGlobalSideType(v string) string {
	switch strings.ToLower(v) {
	case "bid":
		return "BUY"

	case "ask":
		return "SELL"

	}

	return strings.ToUpper(v)
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

func convertRemoteTrade(t maxapi.Trade) (*types.Trade, error) {
	// skip trade ID that is the same. however this should not happen
	var side = toGlobalSideType(t.Side)

	// trade time
	mts := time.Unix(0, t.CreatedAtMilliSeconds*int64(time.Millisecond))

	price, err := strconv.ParseFloat(t.Price, 64)
	if err != nil {
		return nil, err
	}

	quantity, err := strconv.ParseFloat(t.Volume, 64)
	if err != nil {
		return nil, err
	}

	quoteQuantity, err := strconv.ParseFloat(t.Funds, 64)
	if err != nil {
		return nil, err
	}

	fee, err := strconv.ParseFloat(t.Fee, 64)
	if err != nil {
		return nil, err
	}

	return &types.Trade{
		ID:            int64(t.ID),
		Price:         price,
		Symbol:        t.Market,
		Exchange:      "max",
		Quantity:      quantity,
		Side:          side,
		IsBuyer:       t.IsBuyer(),
		IsMaker:       t.IsMaker(),
		Fee:           fee,
		FeeCurrency:   t.FeeCurrency,
		QuoteQuantity: quoteQuantity,
		Time:          mts,
	}, nil
}
