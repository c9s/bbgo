package coinbase

// TODO: support "funds" for submitting orders

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	api "github.com/c9s/bbgo/pkg/exchange/coinbase/api/v1"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/c9s/bbgo/pkg/util/tradingutil"
	"github.com/c9s/requestgen"
	"github.com/sirupsen/logrus"
)

var (
	// https://docs.cdp.coinbase.com/exchange/docs/rate-limits
	// Rate Limit: 10 requests per 2 seconds, Rate limit rule: UserID

	// compile time check: implemented interface
	_ types.Exchange                             = &Exchange{}
	_ types.ExchangeMarketDataService            = &Exchange{}
	_ types.CustomIntervalProvider               = &Exchange{}
	_ tradingutil.CancelAllOrdersBySymbolService = &Exchange{}
	_ types.ExchangeDefaultFeeRates              = &Exchange{}
	_ types.ExchangeTradeHistoryService          = &Exchange{}
	_ types.ExchangeTransferHistoryService       = &Exchange{}
)

const (
	ID                = "coinbase"
	PlatformToken     = "COIN"
	PaginationLimit   = 100
	DefaultKLineLimit = 300
)

var logger = logrus.WithField("exchange", ID)
var warnFirstLogger = util.NewWarnFirstLogger(
	5, time.Minute, logger,
)

type Exchange struct {
	client *api.RestAPIClient

	// api keys
	apiKey        string
	apiSecret     string
	apiPassphrase string

	// order will be added to activeOrderStore when it is submitted successfully
	// once the websocket stream receives a done message of the order, it will be removed from activeOrderStore
	// the purpose of activeOrderStore is to serve as a cache so that we can retrieve the order info when processing the websocket feed
	// ex: received message
	activeOrderStore *ActiveOrderStore
}

func New(key, secret, passphrase string, timeout time.Duration) *Exchange {
	client := api.NewClient(key, secret, passphrase, timeout)
	return &Exchange{
		client: client,

		apiKey:           key,
		apiSecret:        secret,
		apiPassphrase:    passphrase,
		activeOrderStore: newActiveOrderStore(key),
	}
}

func (e *Exchange) Initialize(ctx context.Context) error {
	e.activeOrderStore.Start(ctx)
	return nil
}

func (e *Exchange) Client() *api.RestAPIClient {
	return e.client
}

// CustomIntervalProvider
var supportedIntervalMap = map[types.Interval]int{
	types.Interval1m:  60,
	types.Interval5m:  5 * 60,
	types.Interval15m: 15 * 60,
	types.Interval1h:  60 * 60,
	types.Interval6h:  6 * 60 * 60,
	types.Interval1d:  24 * 60 * 60,
}

func (e *Exchange) SupportedInterval() map[types.Interval]int {
	return supportedIntervalMap
}

func (e *Exchange) IsSupportedInterval(interval types.Interval) bool {
	_, ok := supportedIntervalMap[interval]
	return ok
}

// ExchangeMinimal
func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeCoinBase
}

// Empty string: Coinbase trading fee is dynamic and depends on the market conditions
// https://help.coinbase.com/en/exchange/trading-and-funding/exchange-fees
func (e *Exchange) PlatformFeeCurrency() string {
	return ""
}

// ExchangeAccountService
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
	req := e.client.NewGetBalancesRequest()
	accounts, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}
	balances := make(types.BalanceMap)
	for _, cbBalance := range accounts {
		cur := strings.ToUpper(cbBalance.Currency)
		balances[cur] = toGlobalBalance(cur, &cbBalance)
	}
	return balances, nil
}

func (e *Exchange) queryAccountIDsBySymbols(ctx context.Context, symbols []string) ([]string, error) {
	timedCtx, cancel := context.WithTimeout(ctx, time.Second*20)
	defer cancel()

	markets, err := e.QueryMarkets(timedCtx)
	if err != nil {
		return nil, fmt.Errorf("[coinbase] fail to query markets: %w", err)
	}
	// accountIDsMap is a map from currency (ex: BTC) to its account ID
	accountIDsMap, err := e.queryAccountIDsMap(timedCtx)
	if err != nil {
		return nil, fmt.Errorf("[coinbase] fail to query account IDs for private symbols: %w", err)
	}
	// deduplicate currencies of the symbols
	dedupAccountIDs := make(map[string]struct{})
	for _, symbol := range symbols {
		market, ok := markets[symbol]
		if !ok {
			logger.Warnf("fail to find market for symbol: %s", symbol)
			continue
		}
		if baseAccountId, ok := accountIDsMap[market.BaseCurrency]; ok {
			dedupAccountIDs[baseAccountId] = struct{}{}
		}
		if quoteAccountId, ok := accountIDsMap[market.QuoteCurrency]; ok {
			dedupAccountIDs[quoteAccountId] = struct{}{}
		}
	}
	// balanceAccountIDs is a map from currencies of the given symbols to its account ID
	balanceAccountIDs := make([]string, 0)
	for accountId := range dedupAccountIDs {
		balanceAccountIDs = append(balanceAccountIDs, accountId)
	}
	return balanceAccountIDs, nil
}

// queryAccountIDsMap queries the account IDs for all currencies
// It returns a map of currency to its account ID.
func (e *Exchange) queryAccountIDsMap(ctx context.Context) (map[string]string, error) {
	req := e.client.NewGetBalancesRequest()
	accounts, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}
	accountIDsMap := make(map[string]string)
	for _, account := range accounts {
		accountIDsMap[account.Currency] = account.ID
	}
	return accountIDsMap, nil
}

// ExchangeTradeService
// For the stop-limit order, we only support the long position
// For the market order, though Coinbase supports both funds and size, we only support size in order to simplify the stream handler logic
// We do not support limit order with funds.
func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (createdOrder *types.Order, err error) {
	if len(order.Market.Symbol) == 0 {
		return nil, fmt.Errorf("order.Market.Symbol is required: %+v", order)
	}
	if order.Quantity.IsZero() {
		return nil, fmt.Errorf("order.Quantity is required: %+v", order)
	}
	req := e.client.
		NewCreateOrderRequest().
		ProductID(toLocalSymbol(order.Market.Symbol)).
		Stp("cn") // cancel newest

	// set order type
	switch order.Type {
	case types.OrderTypeLimit:
		req.OrderType("limit")
	case types.OrderTypeMarket:
		req.OrderType("market")
	case types.OrderTypeStopLimit:
		req.OrderType("stop")
		switch order.Side {
		case types.SideTypeSell:
			req.Stop("entry") // trigger order when the price >= stop price
		case types.SideTypeBuy:
			req.Stop("loss") // trigger order when the price <= stop price
		default:
			return nil, fmt.Errorf("unsupported order side: %v", order.Side)
		}
	default:
		return nil, fmt.Errorf("unsupported order type: %v", order.Type)
	}
	// set side
	switch order.Side {
	case types.SideTypeBuy:
		req.Side("buy")
	case types.SideTypeSell:
		req.Side("sell")
	default:
		return nil, fmt.Errorf("unsupported order side: %v", order.Side)
	}
	// set price and quantity
	// NOTE: TruncateQuantity and TruncatePrice are must!
	// ex: price 10000.0869, resulting in 400 response, err msg: "price is too accurate. Smallest unit is 0.01"
	switch order.Type {
	case types.OrderTypeLimit:
		if order.Price.IsZero() || order.Quantity.IsZero() {
			return nil, fmt.Errorf("order.Price and order.Quantity are required for limit order: %+v", order)
		}
		req.Size(order.Market.FormatQuantity(order.Quantity))
		req.Price(order.Market.FormatPrice(order.Price))
	case types.OrderTypeMarket:
		req.Size(order.Market.FormatQuantity(order.Quantity))
		if !order.Price.IsZero() {
			logger.Warning("the price is ignored for market order")
		}
	case types.OrderTypeStopLimit:
		req.Size(order.Market.FormatQuantity(order.Quantity))
		req.StopPrice(order.Market.FormatPrice(order.StopPrice))
		req.Price(order.Market.FormatPrice(order.Price))
		req.StopLimitPrice(order.Market.FormatPrice(order.Price))
	default:
		return nil, fmt.Errorf("unsupported order type: %v", order.Type)
	}

	// set time in force, using default if not set
	if len(order.TimeInForce) > 0 {
		switch order.TimeInForce {
		case types.TimeInForceGTC:
			req.TimeInForce("GTC")
		case types.TimeInForceIOC:
			req.TimeInForce("IOC")
		case types.TimeInForceFOK:
			req.TimeInForce("FOK")
		case types.TimeInForceGTT:
			req.TimeInForce("GTT")
		default:
			return nil, fmt.Errorf("unsupported time in force: %v", order.TimeInForce)
		}
	}
	// client order id
	if len(order.ClientOrderID) > 0 {
		req.ClientOrderID(order.ClientOrderID)
	}

	// Record metrics timing
	requestTime := time.Now()
	res, err := req.Do(ctx)
	duration := time.Since(requestTime)
	var responseErr *requestgen.ErrResponse
	if errors.As(err, &responseErr) {
		recordFailedOrderSubmissionMetrics(order, responseErr)
	} else if err == nil {
		// no error => record success metrics
		recordSuccessOrderSubmissionMetrics(order, duration)
	}

	if err != nil {
		return nil, err
	}
	// note: Coinbase may adjust the order quantity, so we should
	if !order.Quantity.Eq(res.Size) {
		logger.Warnf("the order quantity has been adjusted by the server(%s): %s -> %s", res.ID, order.Quantity, res.Size)
		order.Quantity = res.Size
	}
	createdOrder = submitOrderToGlobalOrder(order, res)
	e.activeOrderStore.add(order, res)
	return createdOrder, nil
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) ([]types.Order, error) {
	cbOrders, err := e.queryOrdersByPagination(ctx, symbol, nil, nil, []string{"open"})
	if err != nil {
		return nil, err
	}
	orders := make([]types.Order, 0, len(cbOrders))
	for _, cbOrder := range cbOrders {
		orders = append(orders, toGlobalOrder(&cbOrder))
	}
	return orders, nil
}

// if symbol is empty string, it will return all orders
func (e *Exchange) queryOrdersByPagination(ctx context.Context, symbol string, startTime, endTime *time.Time, status []string) ([]api.Order, error) {
	sortedBy := "created_at"
	sorting := "desc"
	localSymbol := toLocalSymbol(symbol)
	getOrdersReq := e.client.NewGetOrdersRequest()
	getOrdersReq.Status(status).SortedBy(sortedBy).Sorting(sorting).Limit(PaginationLimit)
	// query all orders if symbol is empty
	if len(localSymbol) > 0 {
		getOrdersReq.ProductID(localSymbol)
	}
	startDate := ""
	if startTime != nil {
		startDate = startTime.Format("2006-01-02")
		getOrdersReq.StartDate(startDate)
	}
	if endTime != nil {
		endDate := endTime.Format("2006-01-02")
		if endDate == startDate {
			// add 24 hours to the end date to include the end time
			endDate = endTime.Add(time.Hour * 24).Format("2006-01-02")
		}
		getOrdersReq.EndDate(endDate)
	}
	cbOrdersDirty, err := getOrdersReq.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}

	donePagination := len(cbOrdersDirty) < PaginationLimit
	for !donePagination {
		select {
		case <-ctx.Done():
			return cbOrdersDirty, ctx.Err()
		default:
			after := time.Time(cbOrdersDirty[len(cbOrdersDirty)-1].CreatedAt)
			getOrdersReq.After(after)
			newOrders, err := getOrdersReq.Do(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get orders while paginating: %w", err)
			}
			cbOrdersDirty = append(cbOrdersDirty, newOrders...)
			donePagination = len(newOrders) < PaginationLimit
		}
	}
	var cbOrders []api.Order
	for _, cbOrder := range cbOrdersDirty {
		if startTime != nil && cbOrder.CreatedAt.Before(*startTime) {
			continue
		}
		if endTime != nil && cbOrder.CreatedAt.After(*endTime) {
			continue
		}
		cbOrders = append(cbOrders, cbOrder)
	}
	return cbOrders, nil
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	var failedOrderIDs []string
	var cancelErrors []error
	for _, order := range orders {
		req := e.client.NewCancelOrderRequest().OrderID(order.UUID)
		// Track cancellation metrics per order
		startTime := time.Now()
		res, err := req.Do(ctx)

		// Record cancel metrics
		duration := time.Since(startTime)
		var responseErr *requestgen.ErrResponse
		if errors.As(err, &responseErr) {
			recordFailedOrderCancelMetrics(order, responseErr)
		} else if err == nil {
			// no error => record success metrics
			recordSuccessOrderCancelMetrics(order, duration)
		}

		if err != nil {
			if isNotFoundError(err) {
				logger.Warnf("order %v not found, consider it has been cancelled", order.UUID)
				e.activeOrderStore.markCanceled(order.UUID)
				continue
			}
			logger.WithError(err).Warnf("failed to cancel order: %v", order.UUID)
			failedOrderIDs = append(failedOrderIDs, order.UUID)
			cancelErrors = append(cancelErrors, err)
			continue
		} else {
			e.activeOrderStore.markCanceled(order.UUID)
			logger.Infof("order %v has been cancelled", *res)
		}
	}
	if len(cancelErrors) > 0 {
		joinedErr := errors.Join(cancelErrors...)
		return fmt.Errorf(
			"failed to cancel orders: %v, due to %w",
			failedOrderIDs,
			joinedErr)
	}
	return nil
}

// ExchangeMarketDataService
func (e *Exchange) NewStream() types.Stream {
	return NewStream(e, e.apiKey, e.apiSecret, e.apiPassphrase)
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	reqMarketInfo := e.client.NewGetMarketInfoRequest()
	markets, err := reqMarketInfo.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get markets: %w", err)
	}

	marketMap := make(types.MarketMap)
	for _, m := range markets {
		// skip products that is not online
		if m.Status != "online" {
			continue
		}
		marketMap[toGlobalSymbol(m.ID)] = toGlobalMarket(&m)
	}
	return marketMap, nil
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	req := e.client.NewGetTickerRequest().ProductID(toLocalSymbol(symbol))
	cbTicker, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get ticker(%s): %w", symbol, err)
	}
	ticker := toGlobalTicker(cbTicker)
	return &ticker, nil
}

func (e *Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	tickers := make(map[string]types.Ticker)
	for _, s := range symbol {
		ticker, err := e.QueryTicker(ctx, s)
		if err != nil {
			return nil, fmt.Errorf("failed to get ticker for %s: %w", s, err)
		}
		tickers[s] = *ticker
	}
	return tickers, nil
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	if !e.IsSupportedInterval(interval) {
		return nil, fmt.Errorf("unsupported interval: %v", interval)
	}
	// default limit is 300, which is the maximum limit of the Coinbase Exchange API
	if options.Limit == 0 {
		options.Limit = DefaultKLineLimit
	}
	if options.Limit > DefaultKLineLimit {
		logger.Warnf("limit %d is greater than the maximum limit 300, set to 300", options.Limit)
		options.Limit = DefaultKLineLimit
	}
	granularity := fmt.Sprintf("%d", interval.Seconds())
	req := e.client.NewGetCandlesRequest().ProductID(toLocalSymbol(symbol)).Granularity(granularity)
	if options.StartTime != nil {
		req.Start(*options.StartTime)
	}
	if options.EndTime != nil {
		req.End(*options.EndTime)
	}
	rawCandles, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get klines(%s, %v): %w",
			symbol,
			interval,
			err,
		)
	}
	candles := make([]api.Candle, 0, len(rawCandles))
	for _, rawCandle := range rawCandles {
		candle, err := rawCandle.Candle()
		if err != nil {
			logger.Warnf("invalid raw candle detected, skipping: %v", rawCandle)
			continue
		}
		candles = append(candles, *candle)
	}

	klines := make([]types.KLine, 0, len(candles))
	for _, candle := range candles {
		kline := toGlobalKline(symbol, interval, &candle)
		klines = append(klines, kline)
	}
	return klines, nil
}

// ExchangeOrderQueryService
func (e *Exchange) QueryOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	req := e.client.NewSingleOrderRequst()
	var orderID string
	if len(q.OrderUUID) == 0 {
		orderID = q.OrderID
	} else {
		orderID = q.OrderUUID
	}
	req = req.OrderID(orderID)

	cbOrder, err := req.Do(ctx)
	if err != nil {
		if activeOrder, found := e.activeOrderStore.get(orderID); found && isNotFoundError(err) {
			// order is found in the active order store but not found on the server -> assume canceled
			// 1. restore the order to return
			order := submitOrderToGlobalOrder(activeOrder.submitOrder, activeOrder.rawOrder)
			// 2. set status to canceled
			order.Status = types.OrderStatusCanceled
			// 3. mark it as canceled
			e.activeOrderStore.markCanceled(orderID)
			return order, nil
		}
		return nil, fmt.Errorf("failed to get order %+v: %w", q, err)
	}
	order := toGlobalOrder(cbOrder)
	return &order, nil
}

func (e *Exchange) QueryOrderTrades(ctx context.Context, q types.OrderQuery) ([]types.Trade, error) {
	cbTrades, err := e.queryOrderTradesByPagination(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("failed to get order trades %+v: %w", q, err)
	}
	trades := make([]types.Trade, 0, len(cbTrades))
	for _, cbTrade := range cbTrades {
		trades = append(trades, toGlobalTrade(&cbTrade))
	}
	return trades, nil
}

func (e *Exchange) queryOrderTradesByPagination(ctx context.Context, q types.OrderQuery) (api.TradeSnapshot, error) {
	req := e.client.NewGetOrderTradesRequest().Limit(PaginationLimit)
	if len(q.OrderUUID) == 0 {
		req.OrderID(q.OrderID)
	} else {
		req.OrderID(q.OrderUUID)
	}
	if len(q.Symbol) > 0 {
		req.ProductID(toLocalSymbol(q.Symbol))
	}
	cbTrades, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	donePagination := len(cbTrades) < PaginationLimit
	for !donePagination {
		select {
		case <-ctx.Done():
			return cbTrades, ctx.Err()
		default:
			lastTrade := cbTrades[len(cbTrades)-1]
			req.After(lastTrade.TradeID)
			newTrades, err := req.Do(ctx)
			if err != nil {
				return nil, err
			}
			cbTrades = append(cbTrades, newTrades...)
			donePagination = len(newTrades) < PaginationLimit
		}
	}
	return cbTrades, nil
}

// tradingutil.CancelAllOrdersBySymbolService
func (e *Exchange) CancelOrdersBySymbol(ctx context.Context, symbol string) ([]types.Order, error) {
	cancelReq := e.client.NewCancelAllOrdersRequest()
	cancelReq.ProductID(toLocalSymbol(symbol))
	canceledOrderIds, err := cancelReq.Do(ctx)
	if err != nil {
		return nil, err
	}
	if len(canceledOrderIds) == 0 {
		return nil, nil
	}

	// Once the order is cancelled and it has no matches, it can not be queried by the API (always 404 error)
	// If we want to retrieve the original order at this point, we also need to check if it was filled or not.
	// The overhead of doing this is huge.
	// As a result, we simply construct empty orders here (without price, quantity, side ...etc)
	orders := make([]types.Order, 0, len(canceledOrderIds))
	for _, orderID := range canceledOrderIds {
		orders = append(orders, types.Order{
			OrderID:   util.FNV64(orderID),
			Exchange:  types.ExchangeCoinBase,
			UUID:      orderID,
			Status:    types.OrderStatusCanceled,
			IsWorking: false,
		})
		e.activeOrderStore.markCanceled(orderID)
	}
	return orders, nil
}

// Coinbase trading fee is dynamic and depends on the market conditions
// Set to tier 2 as a fallback fee rate
// https://help.coinbase.com/en/exchange/trading-and-funding/exchange-fees
func (e *Exchange) DefaultFeeRates() types.ExchangeFee {
	return types.ExchangeFee{
		MakerFeeRate: fixedpoint.NewFromFloat(0.25 * 0.01), // 25 bps = 0.25%
		TakerFeeRate: fixedpoint.NewFromFloat(0.4 * 0.01),  // 40 bps = 0.4%
	}
}

// ExchangeTradeHistoryService
// QueryClosedOrders queries closed orders for the given symbol and time range.
// startTime and endTime are inclusive.
func (e *Exchange) QueryClosedOrders(ctx context.Context, symbol string, startTime, endTime time.Time, lastOrderID uint64) ([]types.Order, error) {
	if lastOrderID > 0 {
		logger.Warning("lastOrderID is not supported for Coinbase, it will be ignored")
	}
	cbOrders, err := e.queryOrdersByPagination(ctx, symbol, &startTime, &endTime, []string{"done"})
	if err != nil {
		return nil, fmt.Errorf("failed to query closed orders for %s: %w", symbol, err)
	}
	orders := make([]types.Order, 0)
	for _, cbOrder := range cbOrders {
		order := toGlobalOrder(&cbOrder)
		orders = append(orders, order)
	}
	return orders, nil
}

func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
	cbTrades, err := e.queryProductTradesByPagination(
		ctx, symbol, options,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query trades for %s: %w", symbol, err)
	}
	var trades []types.Trade
	for _, cbTrade := range cbTrades {
		trades = append(trades, toGlobalTrade(&cbTrade))
	}
	return trades, nil
}

func (e *Exchange) queryProductTradesByPagination(
	ctx context.Context, symbol string, options *types.TradeQueryOptions,
) (cbTrades api.TradeSnapshot, err error) {
	defer func() {
		if err == nil && options.Limit > 0 && len(cbTrades) >= int(options.Limit) {
			cbTrades = cbTrades[:options.Limit]
		}
	}()
	req := e.client.NewGetOrderTradesRequest().Limit(PaginationLimit)
	req.ProductID(toLocalSymbol(symbol))
	if options.StartTime != nil {
		req.StartDate(
			options.StartTime.Format("2006-01-02"),
		)
	}
	if options.EndTime != nil {
		// end_date is exclusive, add one day to the end date
		req.EndDate(
			options.EndTime.AddDate(0, 0, 1).Format("2006-01-02"),
		)
	}
	cbTradesDirty, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}
	// do pagination to get all trades within the start_date and end_date
	// pagination done if the trades are less than PaginationLimit -> have reached the end of the pagination
	donePagination := len(cbTradesDirty) < PaginationLimit
	for !donePagination {
		select {
		case <-ctx.Done():
			return cbTradesDirty, ctx.Err()
		default:
			lastTrade := cbTradesDirty[len(cbTradesDirty)-1]
			req.After(lastTrade.TradeID)
			newTrades, err := req.Do(ctx)
			if err != nil {
				return nil, err
			}
			cbTradesDirty = append(cbTradesDirty, newTrades...)
			donePagination = len(newTrades) < PaginationLimit
		}
	}
	for _, trade := range cbTradesDirty {
		if options.LastTradeID != 0 && trade.TradeID <= options.LastTradeID {
			continue
		}
		cbTrades = append(cbTrades, trade)
	}
	return cbTrades, nil
}

// ExchangeTransferHistoryService
func (e *Exchange) QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) ([]types.Deposit, error) {
	transfers, err := e.queryTransferHistoryByPagination(ctx, asset, since, until, api.TransferTypeDeposit)
	if err != nil {
		return nil, fmt.Errorf("failed to query deposit history for asset %s(%s ~ %s): %w", asset, since, until, err)
	}
	deposits := make([]types.Deposit, 0, len(transfers))
	for _, transfer := range transfers {
		deposits = append(deposits, toGlobalDeposit(&transfer))
	}
	return deposits, nil
}

func (e *Exchange) QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) ([]types.Withdraw, error) {
	transfers, err := e.queryTransferHistoryByPagination(ctx, asset, since, until, api.TransferTypeWithdraw)
	if err != nil {
		return nil, fmt.Errorf("failed to query withdraw history for asset %s(%s ~ %s): %w", asset, since, until, err)
	}
	withdraws := make([]types.Withdraw, 0, len(transfers))
	for _, transfer := range transfers {
		withdraws = append(withdraws, toGlobalWithdraw(&transfer))
	}
	return withdraws, nil
}

func (e *Exchange) queryTransferHistoryByPagination(ctx context.Context, asset string, since, until time.Time, transferType api.TransferType) ([]api.Transfer, error) {
	req := e.client.NewGetTransfersRequest().
		TransferType(transferType).
		Limit(PaginationLimit)
	if len(asset) > 0 {
		req.Currency(strings.ToUpper(asset))
	}
	if !since.IsZero() {
		req.Before(since)
	}
	if !until.IsZero() {
		// `after` is exclusive, add one day to the until date
		req.After(until.AddDate(0, 0, 1))
	}
	transfersDirty, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query transfer history for asset %s: %w", asset, err)
	}
	seenTransfers := make(map[string]struct{})
	for _, transfer := range transfersDirty {
		seenTransfers[transfer.ID] = struct{}{}
	}

	donePagination := len(transfersDirty) < PaginationLimit
	for !donePagination {
		select {
		case <-ctx.Done():
			return transfersDirty, ctx.Err()
		default:
			lastTransfer := transfersDirty[len(transfersDirty)-1]
			lastTime := lastTransfer.CreatedAt.Time()
			// `after` is exclusive, we overlap the pagination to prevent missing transfers
			req.After(lastTime.AddDate(0, 0, 1))
			newTransfers, err := req.Do(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to query transfer history for asset %s while paginating: %w", asset, err)
			}
			// deduplicate transfers for the overlapping pagination
			for _, transfer := range newTransfers {
				if _, seen := seenTransfers[transfer.ID]; seen {
					continue
				}
				transfersDirty = append(transfersDirty, transfer)
				seenTransfers[transfer.ID] = struct{}{}
			}
			donePagination = (len(newTransfers) < PaginationLimit) || lastTime.Before(since)
		}
	}
	// filter transfers by `since` and `until`
	// since `before` and `after` have granularity of day not second or millisecond.
	transfers := make([]api.Transfer, 0, len(transfersDirty))
	for _, transfer := range transfersDirty {
		createTime := transfer.CreatedAt.Time()
		if createTime.Before(since) || createTime.After(until) {
			continue
		}
		transfers = append(transfers, transfer)
	}
	return transfers, nil
}
