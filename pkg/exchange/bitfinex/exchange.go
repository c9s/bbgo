package bitfinex

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/bitfinex/bfxapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID types.ExchangeName = "bitfinex"

func init() {
	_ = types.Exchange(&Exchange{})
	_ = types.ExchangeTradeHistoryService(&Exchange{})
	_ = types.ExchangeOrderQueryService(&Exchange{})

	if types.ExchangeBitfinex != ID {
		panic(fmt.Sprintf("exchange ID mismatch: expected %s, got %s", types.ExchangeBitfinex, ID))
	}
}

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
	return ID
}

// PlatformFeeCurrency returns the platform fee currency for Bitfinex.
func (e *Exchange) PlatformFeeCurrency() string {
	return ""
}

// NewStream ...
func (e *Exchange) NewStream() types.Stream {
	return NewStream(e)
}

// QueryMarkets queries available markets from Bitfinex.
func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	req := e.client.NewGetPairConfigRequest()
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query markets from bitfinex: %w", err)
	}

	markets := make(types.MarketMap)
	for _, pair := range resp.Pairs {
		if pair.Pair == "" {
			log.Errorf("empty pair found in bitfinex pair config, skipping")
			continue
		}

		symbol := toGlobalSymbol(pair.Pair)
		base, quote := splitLocalSymbol(pair.Pair)
		markets[symbol] = types.Market{
			Exchange:        ID,
			Symbol:          symbol,
			BaseCurrency:    toGlobalCurrency(base),
			QuoteCurrency:   toGlobalCurrency(quote),
			MinQuantity:     pair.MinOrderSize,
			MaxQuantity:     pair.MaxOrderSize,
			MinNotional:     fixedpoint.NewFromFloat(10.0),
			PricePrecision:  8,
			VolumePrecision: 8,
			StepSize:        fixedpoint.NewFromFloat(0.00001), // manually customized step size for bitfinex
		}
	}
	return markets, nil
}

// QueryTicker queries ticker for a symbol from Bitfinex.
func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	req := e.client.NewGetTickerRequest()
	req.Symbol(toLocalSymbol(symbol))
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, err
	} else if resp != nil {
		return convertTicker(*resp), nil
	}

	return nil, fmt.Errorf("ticker not found for symbol %s", symbol)
}

// QueryTickers queries tickers for multiple symbols from Bitfinex.
func (e *Exchange) QueryTickers(ctx context.Context, symbols ...string) (map[string]types.Ticker, error) {
	req := e.client.NewGetTickersRequest()
	if len(symbols) == 0 {
		req.Symbols("ALL")
	} else {
		req.Symbols(strings.Join(MapSlice[string, string](symbols, func(s string) string {
			return toLocalSymbol(s)
		}), ","))
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query tickers from bitfinex: %w", err)
	}

	result := make(map[string]types.Ticker)
	for _, t := range resp.TradingTickers {
		ticker := convertTicker(t)
		result[toGlobalSymbol(t.Symbol)] = *ticker
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
	localSymbol := toLocalSymbol(symbol)
	candleKey := "trade:" + intervalStr + ":" + localSymbol
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

	var kLines []types.KLine
	for _, c := range resp {
		kLines = append(kLines, convertCandle(c, localSymbol, interval))
	}

	return kLines, nil
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	req := e.client.NewGetWalletsRequest()
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query account from bitfinex: %w", err)
	}

	account := types.NewAccount()

	for _, w := range resp {
		if w.Type == bfxapi.WalletTypeExchange {
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
		if w.Type == bfxapi.WalletTypeExchange {
			cu := toGlobalCurrency(w.Currency)
			balances[cu] = types.Balance{
				Currency:  cu,
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
	req.Symbol(toLocalSymbol(order.Symbol))

	quantity := order.Quantity
	if order.Market.Symbol != "" {
		quantity = order.Market.TruncateQuantity(quantity)
	}

	switch order.Side {
	case types.SideTypeBuy:
		req.Amount(quantity.String())
	case types.SideTypeSell:
		req.Amount(quantity.Neg().String())
	}

	if !order.Price.IsZero() {
		req.Price(order.Price.String())
	}

	switch order.Type {
	case types.OrderTypeLimit:
		switch order.TimeInForce {
		case types.TimeInForceIOC:
			req.OrderType(bfxapi.OrderTypeExchangeIOC)
		case types.TimeInForceFOK:
			req.OrderType(bfxapi.OrderTypeExchangeFOK)
		default:
			req.OrderType(bfxapi.OrderTypeExchangeLimit)
		}
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

	// TODO: support this time in force feature
	// Time-In-Force: datetime for automatic order cancellation (e.g. 2020-01-15 10:45:23).
	// req.Tif("")

	var clientOrderID int64
	if order.ClientOrderID != "" {
		if i, err := strconv.ParseInt(order.ClientOrderID, 10, 64); err != nil {
			return nil, err
		} else {
			clientOrderID = i
		}
	} else {
		clientOrderID = time.Now().UnixMilli()
	}

	// Update the client order ID in the order object
	order.ClientOrderID = strconv.FormatInt(clientOrderID, 10)

	if clientOrderID > 0 {
		req.ClientOrderId(clientOrderID)
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return order.AsOrder(), fmt.Errorf("failed to submit order to bitfinex: %w", err)
	}

	if len(resp.Data) == 0 {
		return order.AsOrder(), fmt.Errorf("no order data returned from bitfinex")
	}

	return toGlobalOrder(resp.Data[0]), nil
}

// QueryOpenOrders queries open orders for a symbol from Bitfinex.
func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	req := e.client.NewRetrieveOrderBySymbolRequest()
	req.Symbol(toLocalSymbol(symbol))

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query open orders from bitfinex: %w", err)
	}

	for _, o := range resp.Orders {
		order := toGlobalOrder(o)

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

func (e *Exchange) QueryOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	// check if it's still an active order
	req := e.client.NewRetrieveOrderBySymbolRequest()
	if q.Symbol != "" {
		req.Symbol(toLocalSymbol(q.Symbol))
	}

	if q.OrderID != "" {
		orderID, err := strconv.ParseInt(q.OrderID, 10, 64)
		if err != nil {
			return nil, err
		}

		req.AddId(orderID)
	} else if q.ClientOrderID != "" {
		clientOrderID, err := strconv.ParseInt(q.ClientOrderID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid client order ID %s: %w", q.ClientOrderID, err)
		}

		if clientOrderID <= 0 {
			return nil, fmt.Errorf("client order ID must be a positive integer, got %d", clientOrderID)
		}

		cidTime := time.UnixMilli(clientOrderID)
		cidDate := cidTime.Format("2006-01-02")
		req.Cid(q.ClientOrderID)
		req.CidDate(cidDate)
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query order from bitfinex: %w", err)
	}

	if len(resp.Orders) > 0 {
		return toGlobalOrder(resp.Orders[0]), nil
	}

	// fallback to query closed orders
	return e.queryClosedOrderByID(ctx, q)
}

func (e *Exchange) queryClosedOrderByID(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	if q.Symbol == "" {
		return nil, fmt.Errorf("symbol is required to query closed orders")
	}

	req := e.client.NewGetOrderHistoryBySymbolRequest().
		Symbol(toLocalSymbol(q.Symbol)).
		Limit(1)

	var orderID int64
	var err error
	if q.OrderID != "" {
		orderID, err = strconv.ParseInt(q.OrderID, 10, 64)
		if err != nil {
			return nil, err
		}

		req.AddOrderId(orderID)
	} else if q.ClientOrderID != "" {
		return nil, fmt.Errorf("client order ID is not supported for closed orders")
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query closed orders from bitfinex: %w", err)
	}

	for _, o := range resp {
		if o.OrderID == orderID {
			return toGlobalOrder(o), nil
		}
	}

	return nil, fmt.Errorf("order %d not found in closed orders", orderID)
}

// QueryOrderTrades queries trades for a specific order using Bitfinex API.
func (e *Exchange) QueryOrderTrades(ctx context.Context, q types.OrderQuery) ([]types.Trade, error) {
	req := e.client.NewGetOrderTradesRequest()

	if q.Symbol != "" {
		req.Symbol(toLocalSymbol(q.Symbol))
	}

	if q.OrderID != "" {
		orderID, err := strconv.ParseInt(q.OrderID, 10, 64)
		if err != nil {
			return nil, err
		}
		req.Id(orderID)
	}

	bfxOrderTrades, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var trades []types.Trade
	for _, t := range bfxOrderTrades {
		trade := convertTrade(t)
		trades = append(trades, *trade)
	}
	return trades, nil
}

func (e *Exchange) QueryTrades(
	ctx context.Context, symbol string, options *types.TradeQueryOptions,
) ([]types.Trade, error) {

	req := e.client.NewGetTradeHistoryBySymbolRequest().Symbol(symbol)

	if options != nil {
		if options.StartTime != nil {
			req.Start(*options.StartTime)
		}

		if options.EndTime != nil {
			req.End(*options.EndTime)
		}
	}

	if options != nil && options.Limit > 0 {
		req.Limit(int(options.Limit))
	} else {
		req.Limit(2500) // Default limit if no options provided
	}

	// +1: sort in ascending order | -1: sort in descending order (by MTS field).
	req.Sort(1)

	bfxTrades, err := req.Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to query trades from bitfinex: %w", err)
	}

	var trades []types.Trade
	for _, t := range bfxTrades {
		trade := convertTrade(t)

		if symbol != "" && trade.Symbol != symbol {
			// If a symbol is specified, filter out trades that do not match
			continue
		}

		if options != nil && options.LastTradeID > 0 && trade.ID < options.LastTradeID {
			// If lastTradeID is specified, filter out trades that are older than lastTradeID
			continue
		}

		trades = append(trades, *trade)
	}

	return trades, nil
}

func (e *Exchange) QueryClosedOrders(
	ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64,
) (orders []types.Order, err error) {
	req := e.client.NewGetOrderHistoryBySymbolRequest().
		Symbol(toLocalSymbol(symbol)).
		Limit(2500)

	if !since.IsZero() {
		req.Start(since)
	}

	if !until.IsZero() {
		req.End(until)
	}

	bfxOrders, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query trades from bitfinex: %w", err)
	}

	for _, o := range bfxOrders {
		order := toGlobalOrder(o)

		if lastOrderID > 0 && order.OrderID <= lastOrderID {
			// If lastOrderID is specified, filter out orders that are older than lastOrderID
			continue
		}

		orders = append(orders, *order)
	}

	return orders, nil
}

// QueryDepth query the order book depth of a symbol
func (e *Exchange) QueryDepth(
	ctx context.Context, symbol string,
) (snapshot types.SliceOrderBook, finalUpdateID int64, err error) {
	response, err := e.client.NewGetBookRequest().
		Symbol(toLocalSymbol(symbol)).
		Precision("P0").
		Length(100).
		Do(ctx)
	if err != nil {
		return snapshot, finalUpdateID, err
	}

	return convertDepth(response, symbol), 0, nil
}

func MapSlice[T, M any](input []T, f func(T) M) []M {
	result := make([]M, len(input))
	for i, v := range input {
		result[i] = f(v)
	}
	return result
}
