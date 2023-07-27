package bybit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	maxOrderIdLen         = 36
	defaultQueryClosedLen = 50
)

// https://bybit-exchange.github.io/docs/zh-TW/v5/rate-limit
// sharedRateLimiter indicates that the API belongs to the public API.
//
// The default order limiter apply 2 requests per second and a 2 initial bucket
// this includes QueryMarkets, QueryTicker
var (
	sharedRateLimiter = rate.NewLimiter(rate.Every(time.Second/2), 2)
	tradeRateLimiter  = rate.NewLimiter(rate.Every(time.Second/5), 5)
	orderRateLimiter  = rate.NewLimiter(rate.Every(100*time.Millisecond), 10)
	closedRateLimiter = rate.NewLimiter(rate.Every(time.Second), 1)

	log = logrus.WithFields(logrus.Fields{
		"exchange": "bybit",
	})
)

type Exchange struct {
	key, secret string
	client      *bybitapi.RestClient
}

func New(key, secret string) (*Exchange, error) {
	client, err := bybitapi.NewClient()
	if err != nil {
		return nil, err
	}

	if len(key) > 0 && len(secret) > 0 {
		client.Auth(key, secret)
	}

	return &Exchange{
		key: key,
		// pragma: allowlist nextline secret
		secret: secret,
		client: client,
	}, nil
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeBybit
}

// PlatformFeeCurrency returns empty string. The platform does not support "PlatformFeeCurrency" but instead charges
// fees using the native token.
func (e *Exchange) PlatformFeeCurrency() string {
	return ""
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	if err := sharedRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("markets rate limiter wait error: %w", err)
	}

	instruments, err := e.client.NewGetInstrumentsInfoRequest().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get instruments, err: %v", err)
	}

	marketMap := types.MarketMap{}
	for _, s := range instruments.List {
		marketMap.Add(toGlobalMarket(s))
	}

	return marketMap, nil
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	if err := sharedRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("ticker order rate limiter wait error: %w", err)
	}

	s, err := e.client.NewGetTickersRequest().Symbol(symbol).DoWithResponseTime(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call ticker, symbol: %s, err: %w", symbol, err)
	}

	if len(s.List) != 1 {
		return nil, fmt.Errorf("unexpected ticker lenght, exp:1, got:%d", len(s.List))
	}

	ticker := toGlobalTicker(s.List[0], s.ClosedTime.Time())
	return &ticker, nil
}

func (e *Exchange) QueryTickers(ctx context.Context, symbols ...string) (map[string]types.Ticker, error) {
	tickers := map[string]types.Ticker{}
	if len(symbols) > 0 {
		for _, s := range symbols {
			t, err := e.QueryTicker(ctx, s)
			if err != nil {
				return nil, err
			}

			tickers[s] = *t
		}

		return tickers, nil
	}

	if err := sharedRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("tickers rate limiter wait error: %w", err)
	}
	allTickers, err := e.client.NewGetTickersRequest().DoWithResponseTime(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call ticker, err: %w", err)
	}

	for _, s := range allTickers.List {
		tickers[s.Symbol] = toGlobalTicker(s, allTickers.ClosedTime.Time())
	}

	return tickers, nil
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	cursor := ""
	for {
		req := e.client.NewGetOpenOrderRequest().Symbol(symbol)
		if len(cursor) != 0 {
			// the default limit is 20.
			req = req.Cursor(cursor)
		}

		if err = tradeRateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("place order rate limiter wait error: %w", err)
		}
		res, err := req.Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query open orders, err: %w", err)
		}

		for _, order := range res.List {
			order, err := toGlobalOrder(order)
			if err != nil {
				return nil, fmt.Errorf("failed to convert order, err: %v", err)
			}

			orders = append(orders, *order)
		}

		if len(res.NextPageCursor) == 0 {
			break
		}
		cursor = res.NextPageCursor
	}

	return orders, nil
}

func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
	if len(order.Market.Symbol) == 0 {
		return nil, fmt.Errorf("order.Market.Symbol is required: %+v", order)
	}

	req := e.client.NewPlaceOrderRequest()
	req.Symbol(order.Symbol)

	// set order type
	orderType, err := toLocalOrderType(order.Type)
	if err != nil {
		return nil, err
	}
	req.OrderType(orderType)

	// set side
	side, err := toLocalSide(order.Side)
	if err != nil {
		return nil, err
	}
	req.Side(side)

	// set quantity
	req.Qty(order.Market.FormatQuantity(order.Quantity))

	// set price
	switch order.Type {
	case types.OrderTypeLimit:
		req.Price(order.Market.FormatPrice(order.Price))
	}

	// set timeInForce
	switch order.TimeInForce {
	case types.TimeInForceFOK:
		req.TimeInForce(bybitapi.TimeInForceFOK)
	case types.TimeInForceIOC:
		req.TimeInForce(bybitapi.TimeInForceIOC)
	default:
		req.TimeInForce(bybitapi.TimeInForceGTC)
	}

	// set client order id
	if len(order.ClientOrderID) > maxOrderIdLen {
		return nil, fmt.Errorf("unexpected length of order id, got: %d", len(order.ClientOrderID))
	}
	req.OrderLinkId(order.ClientOrderID)

	if err := orderRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("place order rate limiter wait error: %w", err)
	}
	res, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to place order, order: %#v, err: %w", order, err)
	}

	if len(res.OrderId) == 0 || res.OrderLinkId != order.ClientOrderID {
		return nil, fmt.Errorf("unexpected order id, resp: %#v, order: %#v", res, order)
	}

	ordersResp, err := e.client.NewGetOpenOrderRequest().OrderLinkId(res.OrderLinkId).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query order by client order id: %s, err: %w", res.OrderLinkId, err)
	}

	if len(ordersResp.List) != 1 {
		return nil, fmt.Errorf("unexpected order length, client order id: %s", res.OrderLinkId)
	}

	return toGlobalOrder(ordersResp.List[0])
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) (errs error) {
	if len(orders) == 0 {
		return nil
	}

	for _, order := range orders {
		req := e.client.NewCancelOrderRequest()

		switch {
		case len(order.ClientOrderID) != 0:
			req.OrderLinkId(order.ClientOrderID)
		case len(order.UUID) != 0 && order.OrderID != 0:
			req.OrderId(order.UUID)
		default:
			errs = multierr.Append(
				errs,
				fmt.Errorf("the order uuid and client order id are empty, order: %#v", order),
			)
			continue
		}

		req.Symbol(order.Market.Symbol)

		if err := orderRateLimiter.Wait(ctx); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("cancel order rate limiter wait, order id: %s, error: %w", order.ClientOrderID, err))
			continue
		}
		res, err := req.Do(ctx)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("failed to cancel order id: %s, err: %w", order.ClientOrderID, err))
			continue
		}
		if res.OrderId != order.UUID || res.OrderLinkId != order.ClientOrderID {
			errs = multierr.Append(errs, fmt.Errorf("unexpected order id, resp: %#v, order: %#v", res, order))
			continue
		}
	}

	return errs
}

func (e *Exchange) QueryClosedOrders(ctx context.Context, symbol string, since, util time.Time, lastOrderID uint64) (orders []types.Order, err error) {
	if !since.IsZero() || !util.IsZero() {
		log.Warn("!!!BYBIT EXCHANGE API NOTICE!!! the since/until conditions will not be effected on SPOT account, bybit exchange does not support time-range-based query currently")
	}

	if err := closedRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("query closed order rate limiter wait error: %w", err)
	}
	res, err := e.client.NewGetOrderHistoriesRequest().
		Symbol(symbol).
		Cursor(strconv.FormatUint(lastOrderID, 10)).
		Limit(defaultQueryClosedLen).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call get order histories error: %w", err)
	}

	for _, order := range res.List {
		o, err2 := toGlobalOrder(order)
		if err2 != nil {
			err = multierr.Append(err, err2)
			continue
		}

		if o.Status.Closed() {
			orders = append(orders, *o)
		}
	}
	if err != nil {
		return nil, err
	}

	return types.SortOrdersAscending(orders), nil
}
