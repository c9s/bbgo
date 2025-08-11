package bitfinex

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/bitfinex/bfxapi"
	"github.com/c9s/bbgo/pkg/types"
)

type Exchange struct {
	apiKey, apiSecret string

	client *bfxapi.Client
}

func New(apiKey, apiSecret string) *Exchange {
	client := bfxapi.NewClient()

	if apiKey != "" && apiSecret != "" {
		client.Auth(apiKey, apiSecret)
	}

	return &Exchange{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		client:    client,
	}
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeBitfinex
}

// PlatformFeeCurrency returns the platform fee currency for Bitfinex.
func (e *Exchange) PlatformFeeCurrency() string {
	return "USD"
}

// NewStream ...
func (e *Exchange) NewStream() types.Stream {
	//TODO implement me
	panic("implement me")
}

// QueryMarkets queries available markets from Bitfinex.
func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	req := e.client.NewGetPairConfigRequest()
	resp, err := req.Do(ctx)
	if err != nil {
		logrus.WithError(err).Error("failed to query markets from bitfinex")
		return nil, err
	}
	markets := make(types.MarketMap)
	for _, pair := range resp.Pairs {
		markets[pair.Pair] = types.Market{
			Symbol: pair.Pair,
			// FIXME: Base and Quote currencies are not available in the response.
			// BaseCurrency:  pair.Base,
			// QuoteCurrency: pair.Quote,
			MinQuantity: pair.MinOrderSize,
			MaxQuantity: pair.MaxOrderSize,
		}
	}
	return markets, nil
}

// QueryTicker queries ticker for a symbol from Bitfinex.
func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	req := e.client.NewGetTickerRequest()
	req.Symbol(symbol)
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, err
	} else if resp != nil {
		return convertTicker(*resp)
	}

	return nil, fmt.Errorf("ticker not found for symbol %s", symbol)
}

// QueryTickers queries tickers for multiple symbols from Bitfinex.
func (e *Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	req := e.client.NewGetTickersRequest()
	if len(symbol) == 0 {
		req.Symbols("ALL")
	} else {
		req.Symbols(strings.Join(symbol, ","))
	}

	resp, err := req.Do(ctx)
	if err != nil {
		logrus.WithError(err).Error("failed to query tickers from bitfinex")
		return nil, err
	}
	result := make(map[string]types.Ticker)
	for _, t := range resp.TradingTickers {
		ticker, err := convertTicker(t)
		if err != nil {
			return nil, err
		}

		result[t.Symbol] = *ticker
	}
	return result, nil
}

// QueryKLines queries historical kline/candle data from Bitfinex.
func (e *Exchange) QueryKLines(
	ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions,
) ([]types.KLine, error) {
	intervalStr := interval.String()

	switch intervalStr {
	case "1d", "1w":
		intervalStr = strings.ToUpper(intervalStr)
	}

	if !bfxapi.ValidateTimeFrame(bfxapi.TimeFrame(intervalStr)) {
		return nil, fmt.Errorf("invalid interval %s for bitfinex", intervalStr)
	}

	// format "trade:1m:tBTCUSD"
	candleKey := "trade:" + intervalStr + ":" + symbol
	req := e.client.NewGetCandlesRequest().
		Candle(candleKey).
		Section("hist")

	if options.Limit > 0 {
		req.Limit(int(options.Limit))
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query klines from bitfinex: %w", err)
	}

	var result []types.KLine
	for _, c := range resp {
		result = append(result, types.KLine{
			StartTime: types.Time(c.Time.Time()),
			EndTime:   types.Time(c.Time.Time().Add(interval.Duration() - time.Millisecond)),

			Open:   c.Open,
			High:   c.High,
			Low:    c.Low,
			Close:  c.Close,
			Volume: c.Volume,
		})
	}
	return result, nil
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	req := e.client.NewGetWalletsRequest()
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query account from bitfinex: %w", err)
	}

	account := types.NewAccount()

	for _, w := range resp {
		if w.Type == "exchange" || w.Type == "deposit" {
			account.SetBalance(w.Currency, types.Balance{
				Currency:  w.Currency,
				Available: w.AvailableBalance,
				Locked:    w.Balance.Sub(w.AvailableBalance),
				Interest:  w.UnsettledInterest,
			})
		}

	}
	return account, nil
}

// QueryAccountBalances queries account balances from Bitfinex.
func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	req := e.client.NewGetWalletsRequest()
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query account balances from bitfinex: %w", err)
	}

	balances := make(types.BalanceMap)
	for _, w := range resp {
		if w.Type == "exchange" || w.Type == "deposit" {
			balances[w.Currency] = types.Balance{
				Currency:  w.Currency,
				Available: w.AvailableBalance,
				Locked:    w.Balance.Sub(w.AvailableBalance),
				Interest:  w.UnsettledInterest,
			}
		}
	}
	return balances, nil
}

func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (createdOrder *types.Order, err error) {
	req := e.client.NewSubmitOrderRequest()
	req.Symbol(order.Symbol)

	switch order.Side {
	case types.SideTypeBuy:
		req.Amount(order.Quantity.String())
	case types.SideTypeSell:
		req.Amount(order.Quantity.Neg().String())
	}

	if !order.Price.IsZero() {
		req.Price(order.Price.String())
	}

	switch order.Type {
	case types.OrderTypeLimit:
		req.OrderType(bfxapi.OrderTypeExchangeLimit)
	case types.OrderTypeMarket:
		req.OrderType(bfxapi.OrderTypeExchangeMarket)
	case types.OrderTypeStopLimit:
		req.OrderType(bfxapi.OrderTypeExchangeStopLimit)
	case types.OrderTypeStopMarket:
		req.OrderType(bfxapi.OrderTypeExchangeStop)
	default:
		req.OrderType(bfxapi.OrderTypeExchangeLimit)
	}

	switch order.Type {
	case types.OrderTypeStopLimit, types.OrderTypeStopMarket:
		req.PriceAuxLimit(order.StopPrice.String())

	}

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to submit order to bitfinex: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no order data returned from bitfinex")
	}

	return convertOrder(resp.Data[0])
}

// QueryOpenOrders queries open orders for a symbol from Bitfinex.
func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	req := e.client.NewRetrieveOrderRequest()

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query open orders from bitfinex: %w", err)
	}

	for _, o := range resp.Orders {
		order, err := convertOrder(o)
		if err != nil {
			return nil, err
		}

		if symbol != "" && order.Symbol != symbol {
			// If a symbol is specified, filter out orders that do not match
			continue
		}

		orders = append(orders, *order)
	}
	return orders, nil
}

// CancelOrders cancels the given orders on Bitfinex.
func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	for _, order := range orders {
		req := e.client.NewCancelOrderRequest()
		req.OrderID(int64(order.OrderID))
		_, err := req.Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to cancel order %d: %w", order.OrderID, err)
		}
	}
	return nil
}

// QueryOrderTrades queries trades for a specific order using Bitfinex API.
func (e *Exchange) QueryOrderTrades(ctx context.Context, q types.OrderQuery) ([]types.Trade, error) {
	req := e.client.NewGetOrderTradesRequest()

	if q.Symbol != "" {
		req.Symbol(q.Symbol)
	}

	if q.OrderID != "" {
		orderID, err := strconv.ParseInt(q.OrderID, 10, 64)
		if err != nil {
			return nil, err
		}
		req.Id(orderID)
	}

	trades, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var result []types.Trade
	for _, t := range trades {
		trade, err := convertTrade(t)
		if err != nil {
			return nil, err
		}

		result = append(result, *trade)
	}
	return result, nil
}
