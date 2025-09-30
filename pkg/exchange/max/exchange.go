package max

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/c9s/requestgen"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
	"golang.org/x/time/rate"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	v3 "github.com/c9s/bbgo/pkg/exchange/max/maxapi/v3"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var log = logrus.WithField("exchange", "max")

func init() {
	_ = types.ExchangeTradeHistoryService(&Exchange{})
}

type Exchange struct {
	types.MarginSettings

	key, secret string

	client *maxapi.RestClient

	v3client *v3.Client

	submitOrderLimiter, queryTradeLimiter, accountQueryLimiter, closedOrderQueryLimiter, marketDataLimiter *rate.Limiter
}

func New(key, secret, subAccount string) *Exchange {
	client := maxapi.NewRestClientDefault()
	client.Auth(key, secret)

	if subAccount != "" {
		client.SetSubAccount(subAccount)
	}

	v3client := v3.NewClient(client)
	return &Exchange{
		client: client,
		key:    key,
		// pragma: allowlist nextline secret
		secret:   secret,
		v3client: v3client,

		queryTradeLimiter: rate.NewLimiter(rate.Every(250*time.Millisecond), 2),

		// 1200 cpm (1200 requests per minute = 20 requests per second)
		submitOrderLimiter: rate.NewLimiter(rate.Every(50*time.Millisecond), 20),

		// closedOrderQueryLimiter is used for the closed orders query rate limit, 1 request per second
		closedOrderQueryLimiter: rate.NewLimiter(rate.Every(1*time.Second), 1),
		accountQueryLimiter:     rate.NewLimiter(rate.Every(250*time.Millisecond), 1),
		marketDataLimiter:       rate.NewLimiter(rate.Every(2*time.Second), 10),
	}
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeMax
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	req := e.v3client.NewGetTickerRequest()
	req.Market(toLocalSymbol(symbol))

	ticker, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	return &types.Ticker{
		Time:   ticker.At.Time(),
		Volume: ticker.Volume,
		Last:   ticker.Last,
		Open:   ticker.Open,
		High:   ticker.High,
		Low:    ticker.Low,
		Buy:    ticker.Buy,
		Sell:   ticker.Sell,
	}, nil
}

func (e *Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	if err := e.marketDataLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	var tickers = make(map[string]types.Ticker)
	if len(symbol) == 1 {
		ticker, err := e.QueryTicker(ctx, symbol[0])
		if err != nil {
			return nil, err
		}

		tickers[toGlobalSymbol(symbol[0])] = *ticker
	} else {
		maxMarkets, err := e.v3client.NewGetMarketsRequest().Do(ctx)
		if err != nil {
			return nil, err
		}

		var marketIdSlice []string
		for _, m := range maxMarkets {
			marketIdSlice = append(marketIdSlice, m.ID)
		}

		req := e.v3client.NewGetTickersRequest()
		req.Markets(marketIdSlice)
		maxTickers, err := req.Do(ctx)
		if err != nil {
			return nil, err
		}

		for _, v := range maxTickers {
			marketId := toGlobalSymbol(v.Market)
			tickers[marketId] = types.Ticker{
				Time:   v.At.Time(),
				Volume: v.Volume,
				Last:   v.Last,
				Open:   v.Open,
				High:   v.High,
				Low:    v.Low,
				Buy:    v.Buy,
				Sell:   v.Sell,
			}
		}
	}

	return tickers, nil
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	req := e.v3client.NewGetMarketsRequest()
	remoteMarkets, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	markets := types.MarketMap{}
	for _, m := range remoteMarkets {
		symbol := toGlobalSymbol(m.ID)

		market := types.Market{
			Exchange:        types.ExchangeMax,
			Symbol:          symbol,
			LocalSymbol:     m.ID,
			PricePrecision:  m.QuoteUnitPrecision,
			VolumePrecision: m.BaseUnitPrecision,
			QuoteCurrency:   toGlobalCurrency(m.QuoteUnit),
			BaseCurrency:    toGlobalCurrency(m.BaseUnit),
			MinNotional:     m.MinQuoteAmount,
			MinAmount:       m.MinQuoteAmount,

			MinQuantity: m.MinBaseAmount,
			MaxQuantity: fixedpoint.NewFromInt(10000),
			// make it like 0.0001
			StepSize: fixedpoint.NewFromFloat(1.0 / math.Pow10(m.BaseUnitPrecision)),
			// used in the price formatter
			MinPrice: fixedpoint.NewFromFloat(1.0 / math.Pow10(m.QuoteUnitPrecision)),
			MaxPrice: fixedpoint.NewFromInt(10000),
			TickSize: fixedpoint.NewFromFloat(1.0 / math.Pow10(m.QuoteUnitPrecision)),
		}

		markets[symbol] = market
	}

	return markets, nil
}

func (e *Exchange) NewStream() types.Stream {
	stream := NewStream(e)
	stream.MarginSettings = e.MarginSettings
	return stream
}

func (e *Exchange) QueryOrderTrades(ctx context.Context, q types.OrderQuery) ([]types.Trade, error) {
	if q.OrderID == "" {
		return nil, errors.New("max.QueryOrder: OrderID is required parameter")
	}

	orderID, err := strconv.ParseInt(q.OrderID, 10, 64)
	if err != nil {
		return nil, err
	}

	maxTrades, err := e.v3client.NewGetOrderTradesRequest().OrderID(uint64(orderID)).Do(ctx)
	if err != nil {
		return nil, err
	}

	var trades []types.Trade
	for _, t := range maxTrades {
		localTrades, err := toGlobalTradeV3(t)
		if err != nil {
			log.WithError(err).Errorf("can not convert trade: %+v", t)
			continue
		}

		// because self-trades will contains ask and bid orders in its struct
		// we need to make sure the trade's order is what we want
		for _, localTrade := range localTrades {
			if localTrade.OrderID == uint64(orderID) {
				trades = append(trades, localTrade)
			}
		}
	}

	// ensure everything is sorted ascending
	trades = types.SortTradesAscending(trades)
	return trades, nil
}

func (e *Exchange) QueryOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	if len(q.OrderID) == 0 && len(q.ClientOrderID) == 0 {
		return nil, errors.New("max.QueryOrder: one of OrderID/ClientOrderID is required parameter")
	}

	request := e.v3client.NewGetOrderRequest()

	if len(q.OrderID) > 0 {
		orderID, err := strconv.ParseInt(q.OrderID, 10, 64)
		if err != nil {
			return nil, err
		}

		request.Id(uint64(orderID))
	} else if len(q.ClientOrderID) > 0 {
		request.ClientOrderID(q.ClientOrderID)
	} else {
		return nil, fmt.Errorf("max.QueryOrder: OrderID or ClientOrderID is required, got OrderQuery: %+v", q)
	}

	maxOrder, err := request.Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalOrder(*maxOrder)
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) ([]types.Order, error) {
	market := toLocalSymbol(symbol)
	walletType := e.getWalletType()

	// timestamp can't be negative, so we need to use time which epochtime is > 0
	since, err := e.getLaunchDate()
	if err != nil {
		return nil, err
	}

	// use types.OrderMap because the timestamp params is inclusive. We will get the duplicated order if we use the last order as new since.
	// If we use since = since + 1ms, we may miss some orders with the same created_at.
	// As a result, we use OrderMap to avoid duplicated or missing order.
	var orderMap types.OrderMap = make(types.OrderMap)
	var limit uint = 1000
	for {
		req := e.v3client.NewGetWalletOpenOrdersRequest(walletType).Market(market).Timestamp(since).OrderBy(maxapi.OrderByAsc).Limit(limit)
		maxOrders, err := req.Do(ctx)
		if err != nil {
			return nil, err
		}

		numUniqueOrders := 0
		for _, maxOrder := range maxOrders {
			createdAt := maxOrder.CreatedAt.Time()
			if createdAt.After(since) {
				since = createdAt
			}

			order, err := toGlobalOrder(maxOrder)
			if err != nil {
				return nil, err
			}

			if _, exist := orderMap[order.OrderID]; !exist {
				orderMap[order.OrderID] = *order
				numUniqueOrders++
			}
		}

		if len(maxOrders) < int(limit) {
			break
		}

		if numUniqueOrders == 0 {
			break
		}
	}

	return orderMap.Orders(), err
}

func (e *Exchange) QueryClosedOrders(
	ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64,
) ([]types.Order, error) {
	if !since.IsZero() || !until.IsZero() {
		return e.queryClosedOrdersByTime(ctx, symbol, since, until, maxapi.OrderByAsc)
	}

	return e.queryClosedOrdersByLastOrderID(ctx, symbol, lastOrderID)
}

func (e *Exchange) QueryClosedOrdersDesc(
	ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64,
) ([]types.Order, error) {
	closedOrders, err := e.queryClosedOrdersByTime(ctx, symbol, since, until, maxapi.OrderByDesc)
	if lastOrderID == 0 {
		return closedOrders, err
	}

	var filterClosedOrders []types.Order
	for _, closedOrder := range closedOrders {
		if closedOrder.OrderID > lastOrderID {
			filterClosedOrders = append(filterClosedOrders, closedOrder)
		}
	}

	return filterClosedOrders, err
}

func (e *Exchange) queryClosedOrdersByLastOrderID(
	ctx context.Context, symbol string, lastOrderID uint64,
) (orders []types.Order, err error) {
	if err := e.closedOrderQueryLimiter.Wait(ctx); err != nil {
		return orders, err
	}

	market := toLocalSymbol(symbol)
	walletType := e.getWalletType()

	if lastOrderID == 0 {
		lastOrderID = 1
	}

	req := e.v3client.NewGetWalletOrderHistoryRequest(walletType).
		Market(market).
		FromID(lastOrderID).
		Limit(1000)

	maxOrders, err := req.Do(ctx)
	if err != nil {
		return orders, err
	}

	for _, maxOrder := range maxOrders {
		order, err2 := toGlobalOrder(maxOrder)
		if err2 != nil {
			err = multierr.Append(err, err2)
			continue
		}

		orders = append(orders, *order)
	}

	if err != nil {
		return nil, err
	}
	return types.SortOrdersAscending(orders), nil
}

func (e *Exchange) queryClosedOrdersByTime(
	ctx context.Context, symbol string, since, until time.Time, orderByType maxapi.OrderByType,
) (orders []types.Order, err error) {
	if err := e.closedOrderQueryLimiter.Wait(ctx); err != nil {
		return orders, err
	}

	// there is since limit for closed orders API. If the since is before launch date, it will respond error
	sinceLimit, err := e.getLaunchDate()
	if err != nil {
		return orders, err
	}
	if since.Before(sinceLimit) {
		since = sinceLimit
	}

	if until.IsZero() {
		until = time.Now()
	}

	market := toLocalSymbol(symbol)

	walletType := e.getWalletType()

	req := e.v3client.NewGetWalletClosedOrdersRequest(walletType).
		Market(market).
		Limit(1000).
		OrderBy(orderByType)

	switch orderByType {
	case maxapi.OrderByAsc:
		req.Timestamp(since)
	case maxapi.OrderByDesc:
		req.Timestamp(until)
	case maxapi.OrderByAscUpdatedAt:
		// not implement yet
		return nil, fmt.Errorf("unsupported order by type: %s", orderByType)
	case maxapi.OrderByDescUpdatedAt:
		// not implement yet
		return nil, fmt.Errorf("unsupported order by type: %s", orderByType)
	default:
		return nil, fmt.Errorf("unsupported order by type: %s", orderByType)
	}

	maxOrders, err := req.Do(ctx)
	if err != nil {
		return orders, err
	}

	for _, maxOrder := range maxOrders {
		createdAt := maxOrder.CreatedAt.Time()
		if createdAt.Before(since) || createdAt.After(until) {
			continue
		}

		order, err2 := toGlobalOrder(maxOrder)
		if err2 != nil {
			err = multierr.Append(err, err2)
			continue
		}

		orders = append(orders, *order)
	}

	if err != nil {
		return nil, err
	}
	return orders, nil
}

func (e *Exchange) CancelAllOrders(ctx context.Context) ([]types.Order, error) {
	walletType := e.getWalletType()
	req := e.v3client.NewCancelWalletOrderAllRequest(walletType)
	orderResponses, err := req.Do(ctx)
	return convertCancelOrderResponses(orderResponses, err)
}

func (e *Exchange) CancelOrdersBySymbol(ctx context.Context, symbol string) ([]types.Order, error) {
	walletType := e.getWalletType()
	req := e.v3client.NewCancelWalletOrderAllRequest(walletType)
	req.Market(toLocalSymbol(symbol))
	orderResponses, err := req.Do(ctx)
	return convertCancelOrderResponses(orderResponses, err)
}

func (e *Exchange) CancelOrdersBySymbolSide(
	ctx context.Context, symbol string, side types.SideType,
) ([]types.Order, error) {
	walletType := e.getWalletType()
	req := e.v3client.NewCancelWalletOrderAllRequest(walletType)
	req.Market(toLocalSymbol(symbol)).
		Side(toLocalSideType(side))

	orderResponses, err := req.Do(ctx)
	return convertCancelOrderResponses(orderResponses, err)
}

func (e *Exchange) CancelOrdersByGroupID(ctx context.Context, groupID uint32) ([]types.Order, error) {
	walletType := e.getWalletType()
	req := e.v3client.NewCancelWalletOrderAllRequest(walletType)
	req.GroupID(groupID)

	orderResponses, err := req.Do(ctx)
	return convertCancelOrderResponses(orderResponses, err)
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) (err2 error) {
	var walletType = e.getWalletType()
	var groupIDs = make(map[uint32]struct{})
	var orphanOrders []types.Order
	for _, o := range orders {
		// only the order's OrderID and ClientOrderID are unknown, we will think we want to cancel the group of orders
		if o.GroupID > 0 && o.OrderID == 0 && len(o.ClientOrderID) == 0 {
			groupIDs[o.GroupID] = struct{}{}
		} else {
			orphanOrders = append(orphanOrders, o)
		}
	}

	if len(groupIDs) > 0 {
		for groupID := range groupIDs {
			req := e.v3client.NewCancelWalletOrderAllRequest(walletType)
			req.GroupID(groupID)

			if _, err := req.Do(ctx); err != nil {
				log.WithError(err).Errorf("group id %d order cancel error", groupID)
				err2 = err
			}
		}
	}

	for _, o := range orphanOrders {
		logFields := logrus.Fields{}
		req := e.v3client.NewCancelOrderRequest()
		if o.OrderID > 0 {
			req.Id(o.OrderID)
			logFields["order_id"] = o.OrderID
		} else if len(o.ClientOrderID) > 0 && o.ClientOrderID != types.NoClientOrderID {
			req.ClientOrderID(o.ClientOrderID)
			logFields["client_order_id"] = o.ClientOrderID
		} else {
			return fmt.Errorf("order id or client order id is not defined, order=%+v", o)
		}

		if _, err := req.Do(ctx); err != nil {
			log.WithError(err).WithFields(logFields).Errorf("order cancel error")
			err2 = err
		}
	}

	return err2
}

func (e *Exchange) Withdraw(
	ctx context.Context, asset string, amount fixedpoint.Value, address string, options *types.WithdrawalOptions,
) error {
	asset = toLocalCurrency(asset)

	addresses, err := e.client.WithdrawalService.NewGetWithdrawalAddressesRequest().
		Currency(asset).
		Do(ctx)

	if err != nil {
		return err
	}

	var whitelistAddress maxapi.WithdrawalAddress
	for _, a := range addresses {
		if a.Address == address {
			whitelistAddress = a
			break
		}
	}

	if whitelistAddress.Address != address {
		return fmt.Errorf("address %s is not in the whitelist", address)
	}

	if whitelistAddress.UUID == "" {
		return errors.New("address UUID can not be empty")
	}

	response, err := e.client.WithdrawalService.NewWithdrawalRequest().
		Currency(asset).
		Amount(amount.Float64()).
		AddressUUID(whitelistAddress.UUID).
		Do(ctx)

	if err != nil {
		return err
	}

	log.Infof("withdrawal request response: %+v", response)
	return nil
}

func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (createdOrder *types.Order, err error) {
	if err := e.submitOrderLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	walletType := maxapi.WalletTypeSpot
	if e.MarginSettings.IsMargin {
		walletType = maxapi.WalletTypeMargin
	}

	o := order
	orderType, err := toLocalOrderType(o.Type)
	if err != nil {
		return createdOrder, err
	}

	// case IOC type
	if orderType == maxapi.OrderTypeLimit && o.TimeInForce == types.TimeInForceIOC {
		orderType = maxapi.OrderTypeIOCLimit
	}

	var quantityString string
	if o.Market.Symbol != "" {
		quantityString = o.Market.FormatQuantity(o.Quantity)
	} else {
		quantityString = o.Quantity.String()
	}

	o.ClientOrderID = newClientOrderID(o.ClientOrderID)

	req := e.v3client.NewCreateWalletOrderRequest(walletType)
	req.Market(toLocalSymbol(o.Symbol)).
		Side(toLocalSideType(o.Side)).
		Volume(quantityString).
		OrderType(orderType)

	if o.ClientOrderID != "" {
		req.ClientOrderID(o.ClientOrderID)
	}

	if o.GroupID > 0 {
		req.GroupID(strconv.FormatUint(uint64(o.GroupID%math.MaxInt32), 10))
	}

	switch o.Type {
	case types.OrderTypeStopLimit, types.OrderTypeLimit, types.OrderTypeLimitMaker:
		if o.Market.Symbol != "" {
			req.Price(o.Market.FormatPrice(o.Price))
		} else {
			req.Price(o.Price.String())
		}
	}

	// set stop price field for limit orders
	switch o.Type {
	case types.OrderTypeStopLimit, types.OrderTypeStopMarket:
		if o.Market.Symbol != "" {
			req.StopPrice(o.Market.FormatPrice(o.StopPrice))
		} else {
			req.StopPrice(o.StopPrice.String())
		}
	}

	logger := log.WithFields(o.LogFields())
	retOrder, err := req.Do(ctx)
	if err != nil {
		logger.WithError(err).Warnf("submit order error, now trying to recover the order: %+v", o)

		if errors.Is(err, io.EOF) {
			recoveredOrder, recoverErr := e.recoverOrder(ctx, o, err)
			if recoverErr != nil {
				return nil, recoverErr
			}

			// note, recoveredOrder could be nil if the order is not found
			return recoveredOrder, nil
		}

		var responseErr *requestgen.ErrResponse
		if errors.As(err, &responseErr) {
			logger.WithError(err).Warnf("submit order api responded an error: %s, status code: %d, now trying to recover the order with query %+v",
				responseErr.Error(),
				responseErr.StatusCode,
				o.AsQuery(),
			)

			if responseErr.StatusCode >= 400 {
				if responseErr.IsJSON() {
					var maxError maxapi.ErrorResponse
					if decodeErr := responseErr.DecodeJSON(&maxError); decodeErr != nil {
						logger.WithError(decodeErr).Errorf("unable to decode max api error response: %s", responseErr.Response.Body)
						return nil, errors.Join(err, decodeErr)
					}

					switch maxError.Err.Code {
					// 2007: invalid nonce error
					// 2006: The nonce has already been used by access key.

					case 2016: // 2016 amount too small
						return nil, err

					}
				}

				recoveredOrder, recoverErr := e.recoverOrder(ctx, o, err)
				if recoverErr != nil {
					return nil, recoverErr
				}

				// note, recoveredOrder could be nil if the order is not found
				return recoveredOrder, nil
			}
		}

		return createdOrder, err
	}

	if retOrder == nil {
		return createdOrder, errors.New("api returned a nil order")
	}

	createdOrder, err = toGlobalOrder(*retOrder)
	return createdOrder, err
}

func (e *Exchange) recoverOrder(
	ctx context.Context, orderForm types.SubmitOrder, originalErr error,
) (*types.Order, error) {
	var queryErr error
	var order *types.Order
	var query = orderForm.AsQuery()
	var logFields = orderForm.LogFields()

	log.WithFields(logFields).Warnf("start recovering the order with query %+v", query)

	var op = func() error {
		order, queryErr = e.QueryOrder(ctx, query)
		if queryErr == nil {
			return nil
		}

		var responseErr *requestgen.ErrResponse
		if errors.As(queryErr, &responseErr) {
			log.WithFields(logFields).Warnf("recover query order error: %s, status code: %d", responseErr.Status, responseErr.StatusCode)

			switch responseErr.StatusCode {
			case 404:
				// 404 not found, stop retrying
				return nil
			default:
				if responseErr.StatusCode >= 400 && responseErr.StatusCode < 500 {
					return queryErr
				}
			}
		}

		return queryErr
	}

	// at this point, order and finalErr can be both nil since we try to return an error to break the retry
	finalErr := retry.GeneralBackoff(ctx, op)
	if order != nil {
		return order, nil
	} else if finalErr != nil {
		return nil, types.NewRecoverOrderError(types.ExchangeMax, orderForm, originalErr, finalErr)
	}

	// order == nil and finalErr == nil means the order is not found
	return nil, types.NewRecoverOrderError(types.ExchangeMax, orderForm, originalErr, types.ErrOrderNotFound)
}

// PlatformFeeCurrency
func (e *Exchange) PlatformFeeCurrency() string {
	return toGlobalCurrency("max")
}

func (e *Exchange) getLaunchDate() (time.Time, error) {
	// MAX launch date June 21th, 2018
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		return time.Time{}, err
	}

	return time.Date(2018, time.June, 21, 0, 0, 0, 0, loc), nil
}

func (e *Exchange) QuerySpotAccount(ctx context.Context) (*types.Account, error) {
	if err := e.accountQueryLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	userInfo, err := e.v3client.NewGetUserInfoRequest().Do(ctx)
	if err != nil {
		return nil, err
	}

	// MAX returns the fee rate in the following format:
	//  "maker_fee": 0.0005 -> 0.05%
	//  "taker_fee": 0.0015 -> 0.15%
	a := &types.Account{
		AccountType:  types.AccountTypeSpot,
		MarginLevel:  fixedpoint.Zero,
		MakerFeeRate: fixedpoint.NewFromFloat(userInfo.Current.MakerFee), // 0.15% = 0.0015
		TakerFeeRate: fixedpoint.NewFromFloat(userInfo.Current.TakerFee), // 0.15% = 0.0015
	}

	balances, err := e.queryBalances(ctx, maxapi.WalletTypeSpot)
	if err != nil {
		return nil, err
	}
	a.UpdateBalances(balances)
	return a, nil
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	if err := e.accountQueryLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	userInfo, err := e.v3client.NewGetUserInfoRequest().Do(ctx)
	if err != nil {
		return nil, err
	}

	// MAX returns the fee rate in the following format:
	//  "maker_fee": 0.0005 -> 0.05%
	//  "taker_fee": 0.0015 -> 0.15%

	a := &types.Account{
		MarginLevel:  fixedpoint.Zero,
		MakerFeeRate: fixedpoint.NewFromFloat(userInfo.Current.MakerFee), // 0.15% = 0.0015
		TakerFeeRate: fixedpoint.NewFromFloat(userInfo.Current.TakerFee), // 0.15% = 0.0015
	}

	if e.MarginSettings.IsMargin {
		a.AccountType = types.AccountTypeMargin
	} else {
		a.AccountType = types.AccountTypeSpot
	}

	balances, err := e.QueryAccountBalances(ctx)
	if err != nil {
		return nil, err
	}
	a.UpdateBalances(balances)

	if e.MarginSettings.IsMargin {
		req := e.v3client.NewGetMarginADRatioRequest()
		adRatio, err := req.Do(ctx)
		if err != nil {
			return a, err
		}

		a.MarginLevel = adRatio.AdRatio
		a.TotalAccountValue = adRatio.AssetInUsdt
	}

	return a, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	walletType := maxapi.WalletTypeSpot
	if e.MarginSettings.IsMargin {
		walletType = maxapi.WalletTypeMargin
	}

	return e.queryBalances(ctx, walletType)
}

func (e *Exchange) queryBalances(ctx context.Context, walletType maxapi.WalletType) (types.BalanceMap, error) {
	if err := e.accountQueryLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	req := e.v3client.NewGetWalletAccountsRequest(walletType)

	accounts, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var balances = make(types.BalanceMap)
	for _, b := range accounts {
		cur := toGlobalCurrency(b.Currency)
		balances[cur] = types.Balance{
			Currency:  cur,
			Available: b.Balance,
			Locked:    b.Locked,
			NetAsset:  b.Balance.Add(b.Locked).Sub(b.Principal).Sub(b.Interest),
			Borrowed:  b.Principal,
			Interest:  b.Interest,
		}
	}

	return balances, nil
}

func (e *Exchange) QueryWithdrawHistory(
	ctx context.Context, asset string, since, until time.Time,
) (allWithdraws []types.Withdraw, err error) {
	startTime := since
	limit := 1000
	txIDs := map[string]struct{}{}

	if startTime.IsZero() {
		startTime, err = e.getLaunchDate()
		if err != nil {
			return nil, err
		}
	}

	for startTime.Before(until) {
		// startTime ~ endTime must be in 60 days
		endTime := startTime.AddDate(0, 0, 60)
		if endTime.After(until) {
			endTime = until
		}

		log.Infof("querying withdraw %s: %s <=> %s", asset, startTime, endTime)
		req := e.client.NewGetWithdrawalHistoryRequest()
		if len(asset) > 0 {
			req.Currency(toLocalCurrency(asset))
		}

		withdraws, err := req.
			Timestamp(startTime).
			Order("asc").
			Limit(limit).
			Do(ctx)

		if err != nil {
			return allWithdraws, err
		}

		if len(withdraws) == 0 {
			startTime = endTime
			continue
		}

		for i := len(withdraws) - 1; i >= 0; i-- {
			d := withdraws[i]
			if _, ok := txIDs[d.TxID]; ok {
				continue
			}

			status := convertWithdrawStatusV2(d.State)

			txIDs[d.TxID] = struct{}{}
			withdraw := types.Withdraw{
				Exchange:               types.ExchangeMax,
				ApplyTime:              types.Time(d.CreatedAt),
				Asset:                  toGlobalCurrency(d.Currency),
				Amount:                 d.Amount,
				Address:                d.Address,
				AddressTag:             "",
				TransactionID:          d.TxID,
				TransactionFee:         d.Fee,
				TransactionFeeCurrency: d.FeeCurrency,
				Network:                d.NetworkProtocol,
				Status:                 status,
				OriginalStatus:         string(d.State),
			}

			allWithdraws = append(allWithdraws, withdraw)
		}

		// go next time frame
		if len(withdraws) < limit {
			startTime = endTime
		} else {
			// its in descending order, so we get the first record
			startTime = withdraws[0].CreatedAt.Time()
		}
	}

	return allWithdraws, nil
}

func (e *Exchange) QueryDepositHistory(
	ctx context.Context, asset string, since, until time.Time,
) (allDeposits []types.Deposit, err error) {
	startTime := since
	limit := 1000
	txIDs := map[string]struct{}{}

	emptyTime := time.Time{}
	if startTime == emptyTime {
		startTime, err = e.getLaunchDate()
		if err != nil {
			return nil, err
		}
	}

	toRawStatusStr := func(d maxapi.Deposit) string {
		if len(d.StateReason) > 0 {
			return fmt.Sprintf("%s (%s: %s)", d.Status, d.State, d.StateReason)
		}

		return fmt.Sprintf("%s (%s)", d.Status, d.State)
	}

	for startTime.Before(until) {
		// startTime ~ endTime must be in 90 days
		endTime := startTime.AddDate(0, 0, 60)
		if endTime.After(until) {
			endTime = until
		}

		log.Infof("querying deposit history %s: %s <=> %s", asset, startTime, endTime)

		req := e.client.NewGetDepositHistoryRequest()
		if len(asset) > 0 {
			req.Currency(toLocalCurrency(asset))
		}

		deposits, err := req.
			Timestamp(startTime).
			Order("asc").
			Limit(limit).
			Do(ctx)

		if err != nil {
			return nil, err
		}

		for i := len(deposits) - 1; i >= 0; i-- {
			d := deposits[i]
			if _, ok := txIDs[d.TxID]; ok {
				continue
			}

			allDeposits = append(allDeposits, types.Deposit{
				Exchange:      types.ExchangeMax,
				Time:          types.Time(d.CreatedAt),
				Amount:        d.Amount,
				Asset:         toGlobalCurrency(d.Currency),
				Address:       d.Address,
				AddressTag:    "", // not supported
				TransactionID: d.TxID,
				Status:        toGlobalDepositStatus(d.State),
				Confirmation:  strconv.FormatInt(d.Confirmations, 10),
				Network:       toGlobalNetwork(d.NetworkProtocol),
				RawStatus:     toRawStatusStr(d),
			})
		}

		if len(deposits) < limit {
			startTime = endTime
		} else {
			startTime = time.Time(deposits[0].CreatedAt)
		}
	}

	return allDeposits, err
}

// QueryTrades
// For MAX API spec
// give from_id -> query trades from this id and order by asc
// give timestamp and order is asc -> query trades after timestamp and order by asc
// give timestamp and order is desc -> query trades before timestamp and order by desc
// limit should b1 1~1000
func (e *Exchange) QueryTrades(
	ctx context.Context, symbol string, options *types.TradeQueryOptions,
) (trades []types.Trade, err error) {
	if err := e.queryTradeLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	market := toLocalSymbol(symbol)
	walletType := maxapi.WalletTypeSpot
	if e.MarginSettings.IsMargin {
		walletType = maxapi.WalletTypeMargin
	}

	req := e.v3client.NewGetWalletTradesRequest(walletType)
	req.Market(market)

	if options.Limit > 0 {
		req.Limit(uint64(options.Limit))
	} else {
		req.Limit(1000)
	}

	// If we use start_time as parameter, MAX will ignore from_id.
	// However, we want to use from_id as main parameter for batch.TradeBatchQuery
	if options.LastTradeID > 0 {
		// MAX uses inclusive last trade ID
		req.FromID(options.LastTradeID)
		req.Order("asc")
	} else {
		if options.StartTime != nil {
			req.Timestamp(*options.StartTime)
			req.Order("asc")
		} else if options.EndTime != nil {
			req.Timestamp(*options.EndTime)
			req.Order("desc")
		}
	}

	maxTrades, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	for _, t := range maxTrades {
		if options.StartTime != nil && options.StartTime.After(t.CreatedAt.Time()) {
			continue
		}

		if options.EndTime != nil && options.EndTime.Before(t.CreatedAt.Time()) {
			continue
		}

		localTrades, err := toGlobalTradeV3(t)
		if err != nil {
			log.WithError(err).Errorf("can not convert trade: %+v", t)
			continue
		}

		trades = append(trades, localTrades...)
	}

	// ensure everything is sorted ascending
	trades = types.SortTradesAscending(trades)

	return trades, nil
}

func (e *Exchange) QueryRewards(ctx context.Context, startTime time.Time) ([]types.Reward, error) {
	var from = startTime
	var emptyTime = time.Time{}

	if from == emptyTime {
		from = time.Unix(maxapi.TimestampSince, 0)
	}

	var now = time.Now()
	for {
		if from.After(now) {
			return nil, nil
		}

		// scan by 30 days
		// an user might get most 14 commission records by currency per day
		// limit 1000 / 14 = 71 days
		to := from.Add(time.Hour * 24 * 30)
		req := e.client.RewardService.NewGetRewardsRequest()
		req.From(from.Unix())
		req.To(to.Unix())
		req.Limit(1000)

		maxRewards, err := req.Do(ctx)
		if err != nil {
			return nil, err
		}

		if len(maxRewards) == 0 {
			// next page
			from = to
			continue
		}

		rewards, err := toGlobalRewards(maxRewards)
		if err != nil {
			return nil, err
		}

		// sort them in the ascending order
		sort.Sort(types.RewardSliceByCreationTime(rewards))
		return rewards, nil
	}

	return nil, errors.New("unknown error")
}

// QueryKLines returns the klines from the MAX exchange API.
// The KLine API of the MAX exchange uses inclusive time range
//
// https://max-api.maicoin.com/api/v2/k?market=btctwd&limit=10&period=1&timestamp=1620202440
// The above query will return a kline that starts with 1620202440 (unix timestamp) without endTime.
// We need to calculate the endTime by ourself.
func (e *Exchange) QueryKLines(
	ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions,
) ([]types.KLine, error) {
	if err := e.marketDataLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	var limit = 5000
	if options.Limit > 0 {
		// default limit == 500
		limit = options.Limit
	}

	// workaround for the kline query, because MAX does not support query by end time
	// so we need to use the given end time and the limit number to calculate the start time
	if options.EndTime != nil && options.StartTime == nil {
		startTime := options.EndTime.Add(-time.Duration(limit) * interval.Duration())
		options.StartTime = &startTime
	}

	if options.StartTime == nil {
		return nil, errors.New("start time can not be empty")
	}

	log.Infof("querying kline %s %s %+v", symbol, interval, options)

	period, err := v3.IntervalToPeriod(string(interval))
	if err != nil {
		return nil, err
	}

	localKLines, err := e.v3client.NewGetKLinesRequest().
		Market(toLocalSymbol(symbol)).
		Period(int(period)).Timestamp(*options.StartTime).Limit(limit).
		Do(ctx)

	return toGlobalKLines(localKLines, symbol, period, interval, options.EndTime)
}

func (e *Exchange) QueryDepth(
	ctx context.Context, symbol string, limit int,
) (snapshot types.SliceOrderBook, finalUpdateID int64, err error) {
	req := e.v3client.NewGetDepthRequest()
	localSymbol := toLocalSymbol(symbol)
	req.Market(localSymbol)

	// maximum limit is 300
	if limit > 300 || limit <= 0 {
		limit = 300
	}

	if limit > 0 {
		// default limit is 300
		req.Limit(limit)
	}

	depth, err := req.Do(ctx)
	if err != nil {
		return snapshot, finalUpdateID, err
	}

	return convertDepth(localSymbol, depth)
}

var Two = fixedpoint.NewFromInt(2)

func (e *Exchange) QueryAveragePrice(ctx context.Context, symbol string) (fixedpoint.Value, error) {
	ticker, err := e.client.PublicService.Ticker(toLocalSymbol(symbol))
	if err != nil {
		return fixedpoint.Zero, err
	}

	return ticker.Sell.Add(ticker.Buy).Div(Two), nil
}

func (e *Exchange) RepayMarginAsset(ctx context.Context, asset string, amount fixedpoint.Value) error {
	req := e.v3client.NewMarginRepayRequest()
	req.Currency(toLocalCurrency(asset))
	req.Amount(amount.String())
	resp, err := req.Do(ctx)
	if err != nil {
		return err
	}

	log.Infof("margin repay: %v", resp)
	return nil
}

func (e *Exchange) BorrowMarginAsset(ctx context.Context, asset string, amount fixedpoint.Value) error {
	req := e.v3client.NewMarginLoanRequest()
	req.Currency(toLocalCurrency(asset))
	req.Amount(amount.String())
	resp, err := req.Do(ctx)
	if err != nil {
		return err
	}

	log.Infof("margin borrow: %v", resp)
	return nil
}

func (e *Exchange) QueryMarginAssetMaxBorrowable(
	ctx context.Context, asset string,
) (amount fixedpoint.Value, err error) {
	req := e.v3client.NewGetMarginBorrowingLimitsRequest()
	resp, err := req.Do(ctx)
	if err != nil {
		return fixedpoint.Zero, err
	}

	if resp == nil {
		return fixedpoint.Zero, errors.New("borrowing limits response is nil")
	}

	limits := resp
	if limit, ok := limits[toLocalCurrency(asset)]; ok {
		return limit, nil
	}

	return amount, fmt.Errorf("borrowing limit of %s not found", asset)
}

func (e *Exchange) getWalletType() maxapi.WalletType {
	if e.MarginSettings.IsMargin {
		return maxapi.WalletTypeMargin
	}

	return maxapi.WalletTypeSpot
}

// DefaultFeeRates returns the MAX VIP 0 fee schedule
// See also https://max-vip-zh.maicoin.com/
func (e *Exchange) DefaultFeeRates() types.ExchangeFee {
	return types.ExchangeFee{
		MakerFeeRate: fixedpoint.NewFromFloat(0.01 * 0.045), // 0.045%
		TakerFeeRate: fixedpoint.NewFromFloat(0.01 * 0.150), // 0.15%
	}
}

var SupportedIntervals = map[types.Interval]int{
	types.Interval1m:  1 * 60,
	types.Interval5m:  5 * 60,
	types.Interval15m: 15 * 60,
	types.Interval30m: 30 * 60,
	types.Interval1h:  60 * 60,
	types.Interval2h:  60 * 60 * 2,
	types.Interval4h:  60 * 60 * 4,
	types.Interval6h:  60 * 60 * 6,
	types.Interval12h: 60 * 60 * 12,
	types.Interval1d:  60 * 60 * 24,
	types.Interval3d:  60 * 60 * 24 * 3,
}

func (e *Exchange) SupportedInterval() map[types.Interval]int {
	return SupportedIntervals
}

func (e *Exchange) IsSupportedInterval(interval types.Interval) bool {
	_, ok := SupportedIntervals[interval]
	return ok
}

func logResponse(resp interface{}, err error, req interface{}) error {
	if err != nil {
		log.WithError(err).Errorf("%T: error %+v", req, resp)
		return err
	}

	log.Infof("%T: response: %+v", req, resp)
	return nil
}

func convertCancelOrderResponses(resps []v3.OrderCancelResponse, err error) ([]types.Order, error) {
	if err != nil {
		return nil, fmt.Errorf("cancel orders error: %w", err)
	}

	var maxOrders []maxapi.Order
	for _, resp := range resps {
		if resp.Error != nil {
			log.Warnf("cancel order error: %s", *resp.Error)
		} else {
			maxOrders = append(maxOrders, resp.Order)
		}
	}

	return toGlobalOrders(maxOrders)
}
