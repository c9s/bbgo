package max

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
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
	req.Market(toLocalSymbol(symbol))

	if options.Limit > 0 {
		req.Limit(options.Limit)
	}

	if options.LastTradeID > 0 {
		req.From(options.LastTradeID)
	}

	// make it compatible with binance, we need the last trade id for the next page.
	req.OrderBy("asc")

	remoteTrades, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	for _, t := range remoteTrades {
		localTrade, err := convertRemoteTrade(t)
		if err != nil {
			logger.WithError(err).Errorf("can not convert trade: %+v", t)
			continue
		}

		logger.Infof("T: id=%d % 4s %s P=%f Q=%f %s", localTrade.ID, localTrade.Symbol, localTrade.Side, localTrade.Price, localTrade.Quantity, localTrade.Time)

		trades = append(trades, *localTrade)
	}

	return trades, nil
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol, interval string, options types.KLineQueryOptions) ([]types.KLine, error) {
	var limit = 5000
	if options.Limit > 0 {
		// default limit == 500
		limit = options.Limit
	}

	if options.StartTime == nil {
		return nil, errors.New("start time can not be empty")
	}

	if options.EndTime != nil {
		return nil, errors.New("end time is not supported")
	}

	log.Infof("querying kline %s %s %v", symbol, interval, options)

	// avoid rate limit
	time.Sleep(100 * time.Millisecond)

	localKlines, err := e.client.PublicService.KLines(toLocalSymbol(symbol), interval, *options.StartTime, limit)
	if err != nil {
		return nil, err
	}

	var kLines []types.KLine
	for _, k := range localKlines {
		kLines = append(kLines, k.KLine())
	}

	return kLines, nil
}

func (e *Exchange) QueryAveragePrice(ctx context.Context, symbol string) (float64, error) {
	ticker, err := e.client.PublicService.Ticker(toLocalSymbol(symbol))
	if err != nil {
		return 0, err
	}

	return (util.MustParseFloat(ticker.Sell) + util.MustParseFloat(ticker.Buy)) / 2, nil
}

func toGlobalCurrency(currency string) string {
	return strings.ToUpper(currency)
}

func toLocalCurrency(currency string) string {
	return strings.ToLower(currency)
}

func toLocalSymbol(symbol string) string {
	return strings.ToLower(symbol)
}

func toGlobalSymbol(symbol string) string {
	return strings.ToUpper(symbol)
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

	case "self-trade":
		return "SELF"

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
		Symbol:        toGlobalSymbol(t.Market),
		Exchange:      "max",
		Quantity:      quantity,
		Side:          side,
		IsBuyer:       t.IsBuyer(),
		IsMaker:       t.IsMaker(),
		Fee:           fee,
		FeeCurrency:   toGlobalCurrency(t.FeeCurrency),
		QuoteQuantity: quoteQuantity,
		Time:          mts,
	}, nil
}
