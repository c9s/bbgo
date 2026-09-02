package backpack

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/exchange/backpack/backpackapi"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	ID = "backpack"

	// PlatformToken is the platform currency of Backpack. Backpack has no exchange token, and
	// every market is quoted in USDC, so USDC is the closest equivalent.
	PlatformToken = "USDC"

	// defaultQueryLimit is the page size used for the paginated history endpoints.
	defaultQueryLimit = 100

	// maxQueryLimit is the largest page size the history endpoints accept.
	maxQueryLimit = 1000
)

var log = logrus.WithFields(logrus.Fields{"exchange": ID})

var (
	// Backpack allows 2000 requests per minute per subaccount for the general endpoints,
	// and 30 requests per minute for the historical market data endpoints.
	queryMarketLimiter  = rate.NewLimiter(rate.Every(30*time.Millisecond), 1)
	queryTickerLimiter  = rate.NewLimiter(rate.Every(30*time.Millisecond), 1)
	queryTickersLimiter = rate.NewLimiter(rate.Every(30*time.Millisecond), 1)
	queryAccountLimiter = rate.NewLimiter(rate.Every(30*time.Millisecond), 1)
	queryOrderLimiter   = rate.NewLimiter(rate.Every(30*time.Millisecond), 1)
	placeOrderLimiter   = rate.NewLimiter(rate.Every(30*time.Millisecond), 1)
	cancelOrderLimiter  = rate.NewLimiter(rate.Every(30*time.Millisecond), 1)

	// the historical endpoints are limited to 30 requests per minute
	queryKLineLimiter   = rate.NewLimiter(rate.Every(2*time.Second), 1)
	queryHistoryLimiter = rate.NewLimiter(rate.Every(2*time.Second), 1)
)

// interface implementations, checked at compile time
var (
	_ types.Exchange                    = (*Exchange)(nil)
	_ types.ExchangeOrderQueryService   = (*Exchange)(nil)
	_ types.ExchangeTradeHistoryService = (*Exchange)(nil)
)

type Exchange struct {
	// FuturesSettings is embedded so that the symbol lookup can switch between the spot and the
	// perpetual market of the same global symbol. Only spot is implemented for now; see
	// QueryMarkets.
	types.FuturesSettings

	key, secret string

	client *backpackapi.RestClient
}

func New(key, secret string) (*Exchange, error) {
	client := backpackapi.NewClient()

	if len(key) > 0 && len(secret) > 0 {
		if err := client.Auth(key, secret); err != nil {
			return nil, err
		}
	}

	return &Exchange{
		key:    key,
		secret: secret,
		client: client,
	}, nil
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeBackpack
}

func (e *Exchange) PlatformFeeCurrency() string {
	return PlatformToken
}

// GetClient exposes the underlying REST client for callers that need the raw API.
func (e *Exchange) GetClient() *backpackapi.RestClient {
	return e.client
}

// NewStream returns the websocket stream.
func (e *Exchange) NewStream() types.Stream {
	return NewStream(e, e.client)
}

// getMarketType returns the market type the exchange currently operates on.
func (e *Exchange) getMarketType() backpackapi.MarketType {
	if e.IsFutures {
		return backpackapi.MarketTypePerp
	}

	return backpackapi.MarketTypeSpot
}

// getLocalSymbol converts a global symbol into the Backpack symbol for the market type this
// exchange operates on. Every outbound call must go through this rather than calling
// toLocalSymbol directly.
func (e *Exchange) getLocalSymbol(symbol string) string {
	return toLocalSymbol(symbol, e.getMarketType())
}

// QueryMarkets returns the spot markets.
//
// Only spot is supported for now. The perpetual markets are filtered out and the perp symbol
// map is left empty; adding futures support is a matter of querying the PERP market type here
// and populating that map, since every call site already resolves symbols through
// getLocalSymbol.
func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	if err := queryMarketLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("markets rate limiter wait error: %w", err)
	}

	req := e.client.NewGetMarketsRequest()
	req.MarketType(backpackapi.MarketTypeSpot)

	backpackMarkets, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	markets := types.MarketMap{}
	for i := range backpackMarkets {
		m := backpackMarkets[i]
		if m.MarketType != backpackapi.MarketTypeSpot {
			continue
		}

		market := toGlobalMarket(&m)
		markets[market.Symbol] = market
	}

	syncLocalSymbolMap(markets, backpackapi.MarketTypeSpot)

	return markets, nil
}

// QueryDepth returns an order book snapshot together with its last update id.
//
// The websocket depth stream sends incremental updates carrying a first/last update id range,
// so the snapshot id is what lets depth.Buffer line the two up.
func (e *Exchange) QueryDepth(ctx context.Context, symbol string) (types.SliceOrderBook, int64, error) {
	if err := queryTickerLimiter.Wait(ctx); err != nil {
		return types.SliceOrderBook{}, 0, fmt.Errorf("depth rate limiter wait error: %w", err)
	}

	req := e.client.NewGetDepthRequest()
	req.Symbol(e.getLocalSymbol(symbol))

	depth, err := req.Do(ctx)
	if err != nil {
		return types.SliceOrderBook{}, 0, err
	}

	return toGlobalOrderBook(symbol, depth), int64(depth.LastUpdateId), nil
}

// QueryTicker returns the 24h ticker of a symbol.
//
// The Backpack ticker endpoint carries no bid/ask, so the top of the order book is fetched
// separately to fill types.Ticker.Buy and Sell.
func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	if err := queryTickerLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("ticker rate limiter wait error: %w", err)
	}

	localSymbol := e.getLocalSymbol(symbol)

	req := e.client.NewGetTickerRequest()
	req.Symbol(localSymbol)

	backpackTicker, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	ticker := toGlobalTicker(backpackTicker, time.Now())

	// fill the best bid/ask from the order book
	depthReq := e.client.NewGetDepthRequest()
	depthReq.Symbol(localSymbol).Limit(backpackapi.DepthLimit5)

	depth, err := depthReq.Do(ctx)
	if err != nil {
		// the ticker itself is still usable without the bid/ask
		log.WithError(err).Warnf("unable to query the order book of %s for the ticker bid/ask", symbol)
		return &ticker, nil
	}

	book := toGlobalOrderBook(symbol, depth)
	if bestBid, ok := book.BestBid(); ok {
		ticker.Buy = bestBid.Price
	}

	if bestAsk, ok := book.BestAsk(); ok {
		ticker.Sell = bestAsk.Price
	}

	return &ticker, nil
}

// QueryTickers returns the 24h tickers.
//
// Unlike QueryTicker, the returned tickers have no Buy/Sell price: the Backpack ticker endpoint
// does not carry the bid/ask and querying the order book of every market is not viable.
func (e *Exchange) QueryTickers(ctx context.Context, symbols ...string) (map[string]types.Ticker, error) {
	if err := queryTickersLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("tickers rate limiter wait error: %w", err)
	}

	backpackTickers, err := e.client.NewGetTickersRequest().Do(ctx)
	if err != nil {
		return nil, err
	}

	// build the requested set so that the response can be filtered
	requested := make(map[string]struct{}, len(symbols))
	for _, symbol := range symbols {
		requested[symbol] = struct{}{}
	}

	marketType := e.getMarketType()

	now := time.Now()
	tickers := make(map[string]types.Ticker)
	for i := range backpackTickers {
		t := backpackTickers[i]

		// the endpoint has no market type filter and returns both the spot and the perpetual
		// market; without this they would collapse onto the same global symbol
		if !isLocalSymbolOfMarketType(t.Symbol, marketType) {
			continue
		}

		symbol := toGlobalSymbol(t.Symbol)
		if len(requested) > 0 {
			if _, ok := requested[symbol]; !ok {
				continue
			}
		}

		tickers[symbol] = toGlobalTicker(&t, now)
	}

	return tickers, nil
}

// QueryKLines returns the klines of a symbol.
//
// Backpack has no limit parameter on the kline endpoint, so options.Limit is applied by
// truncating the response. startTime is required by the API; when the caller does not provide
// one it is derived from the limit and the interval.
func (e *Exchange) QueryKLines(
	ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions,
) ([]types.KLine, error) {
	localInterval, err := toLocalInterval(interval)
	if err != nil {
		return nil, err
	}

	if err := queryKLineLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("klines rate limiter wait error: %w", err)
	}

	limit := options.Limit
	if limit <= 0 {
		limit = defaultQueryLimit
	}

	req := e.client.NewGetKlinesRequest()
	req.Symbol(e.getLocalSymbol(symbol)).Interval(localInterval)

	// the API requires a start time
	if options.StartTime != nil {
		req.StartTime(*options.StartTime)
	} else if options.EndTime != nil {
		req.StartTime(options.EndTime.Add(-interval.Duration() * time.Duration(limit)))
	} else {
		req.StartTime(time.Now().Add(-interval.Duration() * time.Duration(limit)))
	}

	if options.EndTime != nil {
		req.EndTime(*options.EndTime)
	}

	backpackKLines, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var kLines []types.KLine
	for i := range backpackKLines {
		k := backpackKLines[i]
		kLines = append(kLines, toGlobalKLine(symbol, interval, &k))
	}

	// the endpoint has no limit parameter, keep the most recent ones
	if options.Limit > 0 && len(kLines) > options.Limit {
		kLines = kLines[len(kLines)-options.Limit:]
	}

	return kLines, nil
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	balances, err := e.QueryAccountBalances(ctx)
	if err != nil {
		return nil, err
	}

	account := types.NewAccount()
	account.UpdateBalances(balances)
	return account, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	if err := queryAccountLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("account rate limiter wait error: %w", err)
	}

	balances, err := e.client.NewGetBalancesRequest().Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalBalance(balances), nil
}

func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
	side, err := toLocalSide(order.Side)
	if err != nil {
		return nil, err
	}

	orderType, postOnly, err := toLocalOrderType(order.Type)
	if err != nil {
		return nil, err
	}

	req := e.client.NewPlaceOrderRequest()
	req.Symbol(e.getLocalSymbol(order.Symbol)).
		Side(side).
		OrderType(orderType)

	req.Quantity(order.Market.FormatQuantity(order.Quantity))

	switch order.Type {
	case types.OrderTypeLimit, types.OrderTypeLimitMaker, types.OrderTypeStopLimit:
		req.Price(order.Market.FormatPrice(order.Price))
	}

	if postOnly {
		req.PostOnly(true)
	}

	if order.ReduceOnly {
		req.ReduceOnly(true)
	}

	if len(order.TimeInForce) > 0 {
		tif, err := toLocalTimeInForce(order.TimeInForce)
		if err != nil {
			return nil, err
		}

		req.TimeInForce(tif)
	}

	// Backpack's client order id is a uint32 rather than a free-form string
	if len(order.ClientOrderID) > 0 {
		clientID, err := parseClientOrderID(order.ClientOrderID)
		if err != nil {
			return nil, err
		}

		req.ClientId(clientID)
	}

	if err := placeOrderLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("place order rate limiter wait error: %w", err)
	}

	backpackOrder, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalOrder(backpackOrder)
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) ([]types.Order, error) {
	if err := queryOrderLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("open orders rate limiter wait error: %w", err)
	}

	req := e.client.NewGetOpenOrdersRequest()
	req.MarketType(e.getMarketType())

	if len(symbol) > 0 {
		req.Symbol(e.getLocalSymbol(symbol))
	}

	backpackOrders, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var orders []types.Order
	for i := range backpackOrders {
		order, err := toGlobalOrder(&backpackOrders[i])
		if err != nil {
			return nil, err
		}

		orders = append(orders, *order)
	}

	return orders, nil
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	for _, order := range orders {
		if err := cancelOrderLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("cancel order rate limiter wait error: %w", err)
		}

		req := e.client.NewCancelOrderRequest()
		req.Symbol(e.getLocalSymbol(order.Symbol))

		switch {
		case len(order.UUID) > 0:
			req.OrderId(order.UUID)

		case len(order.ClientOrderID) > 0:
			clientID, err := parseClientOrderID(order.ClientOrderID)
			if err != nil {
				return err
			}

			req.ClientId(clientID)

		default:
			// the numeric OrderID is a hash of the UUID and cannot be reversed
			return fmt.Errorf("backpack: cancelling an order requires its UUID or its client order id, order: %+v", order)
		}

		if _, err := req.Do(ctx); err != nil {
			return err
		}
	}

	return nil
}

// QueryOrder implements types.ExchangeOrderQueryService.
//
// The query has to carry the order UUID or the client order id: types.Order.OrderID is an FNV
// hash of the UUID and cannot be turned back into it.
func (e *Exchange) QueryOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	if len(q.Symbol) == 0 {
		return nil, fmt.Errorf("backpack: symbol is required for querying an order")
	}

	if err := queryOrderLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("query order rate limiter wait error: %w", err)
	}

	req := e.client.NewGetOrderRequest()
	req.Symbol(e.getLocalSymbol(q.Symbol))

	// exactly one of orderId / clientId may be sent, sending both breaks the signature
	switch {
	case len(q.OrderUUID) > 0:
		req.OrderId(q.OrderUUID)

	case len(q.OrderID) > 0:
		req.OrderId(q.OrderID)

	case len(q.ClientOrderID) > 0:
		clientID, err := parseClientOrderID(q.ClientOrderID)
		if err != nil {
			return nil, err
		}

		req.ClientId(clientID)

	default:
		return nil, fmt.Errorf("backpack: one of orderUUID, orderID or clientOrderID is required")
	}

	backpackOrder, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalOrder(backpackOrder)
}

// QueryOrderTrades implements types.ExchangeOrderQueryService.
func (e *Exchange) QueryOrderTrades(ctx context.Context, q types.OrderQuery) ([]types.Trade, error) {
	orderID := q.OrderUUID
	if len(orderID) == 0 {
		orderID = q.OrderID
	}

	if len(orderID) == 0 {
		return nil, fmt.Errorf("backpack: orderUUID or orderID is required for querying the trades of an order")
	}

	if err := queryHistoryLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("order trades rate limiter wait error: %w", err)
	}

	req := e.client.NewGetFillHistoryRequest()
	req.OrderId(orderID).Limit(maxQueryLimit)

	if len(q.Symbol) > 0 {
		req.Symbol(e.getLocalSymbol(q.Symbol))
	}

	fills, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalTrades(fills), nil
}

// QueryTrades implements types.ExchangeTradeHistoryService.
//
// Backpack paginates with limit/offset and filters on a millisecond time range; the fill
// history has no "since trade id" cursor, so options.LastTradeID is ignored.
func (e *Exchange) QueryTrades(
	ctx context.Context, symbol string, options *types.TradeQueryOptions,
) ([]types.Trade, error) {
	if err := queryHistoryLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("trades rate limiter wait error: %w", err)
	}

	req := e.client.NewGetFillHistoryRequest()
	req.Symbol(e.getLocalSymbol(symbol)).
		MarketType(e.getMarketType()).
		FillType(backpackapi.FillTypeUser).
		SortDirection(backpackapi.SortDirectionAsc)

	limit := uint64(defaultQueryLimit)
	if options != nil {
		if options.Limit > 0 {
			limit = uint64(options.Limit)
		}

		if options.StartTime != nil {
			req.From(*options.StartTime)
		}

		if options.EndTime != nil {
			req.To(*options.EndTime)
		}
	}

	if limit > maxQueryLimit {
		limit = maxQueryLimit
	}

	req.Limit(limit)

	fills, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalTrades(fills), nil
}

// QueryClosedOrders implements types.ExchangeTradeHistoryService.
//
// The Backpack order history endpoint has no time range filter, only limit/offset, so the
// since/until window and the lastOrderID cursor are applied client-side.
func (e *Exchange) QueryClosedOrders(
	ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64,
) ([]types.Order, error) {
	if err := queryHistoryLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("closed orders rate limiter wait error: %w", err)
	}

	req := e.client.NewGetOrderHistoryRequest()
	req.Symbol(e.getLocalSymbol(symbol)).
		MarketType(e.getMarketType()).
		Limit(maxQueryLimit).
		SortDirection(backpackapi.SortDirectionAsc)

	entries, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var orders []types.Order
	for i := range entries {
		order, err := orderHistoryEntryToGlobalOrder(&entries[i])
		if err != nil {
			log.WithError(err).Warnf("unable to convert the historical order %s, skipped", entries[i].Id)
			continue
		}

		// the endpoint has no time filter, apply the window here
		createdAt := order.CreationTime.Time()
		if !since.IsZero() && createdAt.Before(since) {
			continue
		}

		if !until.IsZero() && createdAt.After(until) {
			continue
		}

		if lastOrderID > 0 && order.OrderID <= lastOrderID {
			continue
		}

		if order.IsWorking {
			continue
		}

		orders = append(orders, *order)
	}

	return orders, nil
}

func toGlobalTrades(fills []backpackapi.OrderFill) []types.Trade {
	var trades []types.Trade
	for i := range fills {
		trades = append(trades, toGlobalTrade(&fills[i]))
	}

	return trades
}

// parseClientOrderID converts the bbgo client order id string into the uint32 Backpack expects.
func parseClientOrderID(clientOrderID string) (uint32, error) {
	var clientID uint64
	if _, err := fmt.Sscanf(clientOrderID, "%d", &clientID); err != nil {
		return 0, fmt.Errorf("backpack: the client order id must be a number that fits in a uint32, got %q: %w",
			clientOrderID, err)
	}

	if clientID > math.MaxUint32 {
		return 0, fmt.Errorf("backpack: the client order id %q exceeds the uint32 range", clientOrderID)
	}

	return uint32(clientID), nil
}
