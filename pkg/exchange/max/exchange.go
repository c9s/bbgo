package max

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
	"golang.org/x/time/rate"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	v3 "github.com/c9s/bbgo/pkg/exchange/max/maxapi/v3"
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
	client      *maxapi.RestClient

	v3client *v3.Client
	v3margin *v3.MarginService

	submitOrderLimiter, queryTradeLimiter, accountQueryLimiter, closedOrderQueryLimiter, marketDataLimiter *rate.Limiter
}

func New(key, secret string) *Exchange {
	baseURL := maxapi.ProductionAPIURL
	if override := os.Getenv("MAX_API_BASE_URL"); len(override) > 0 {
		baseURL = override
	}

	client := maxapi.NewRestClient(baseURL)
	client.Auth(key, secret)
	return &Exchange{
		client: client,
		key:    key,
		// pragma: allowlist nextline secret
		secret:   secret,
		v3client: &v3.Client{Client: client},
		v3margin: &v3.MarginService{Client: client},

		queryTradeLimiter:  rate.NewLimiter(rate.Every(1*time.Second), 2),
		submitOrderLimiter: rate.NewLimiter(rate.Every(100*time.Millisecond), 10),

		// closedOrderQueryLimiter is used for the closed orders query rate limit, 1 request per second
		closedOrderQueryLimiter: rate.NewLimiter(rate.Every(1*time.Second), 1),
		accountQueryLimiter:     rate.NewLimiter(rate.Every(1*time.Second), 1),
		marketDataLimiter:       rate.NewLimiter(rate.Every(2*time.Second), 10),
	}
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeMax
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	ticker, err := e.client.PublicService.Ticker(toLocalSymbol(symbol))
	if err != nil {
		return nil, err
	}

	return &types.Ticker{
		Time:   ticker.Time,
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
		req := e.client.NewGetTickersRequest()
		maxTickers, err := req.Do(ctx)
		if err != nil {
			return nil, err
		}

		m := make(map[string]struct{})
		exists := struct{}{}
		for _, s := range symbol {
			m[toGlobalSymbol(s)] = exists
		}

		for k, v := range maxTickers {
			if _, ok := m[toGlobalSymbol(k)]; len(symbol) != 0 && !ok {
				continue
			}

			tickers[toGlobalSymbol(k)] = types.Ticker{
				Time:   v.Time,
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
	req := e.client.NewGetMarketsRequest()
	remoteMarkets, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	markets := types.MarketMap{}
	for _, m := range remoteMarkets {
		symbol := toGlobalSymbol(m.ID)

		market := types.Market{
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
	stream := NewStream(e.key, e.secret)
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

	if len(q.OrderID) != 0 && len(q.ClientOrderID) != 0 {
		return nil, errors.New("max.QueryOrder: only accept one parameter of OrderID/ClientOrderID")
	}

	request := e.v3client.NewGetOrderRequest()

	if len(q.OrderID) != 0 {
		orderID, err := strconv.ParseInt(q.OrderID, 10, 64)
		if err != nil {
			return nil, err
		}

		request.Id(uint64(orderID))
	}

	if len(q.ClientOrderID) != 0 {
		request.ClientOrderID(q.ClientOrderID)
	}

	maxOrder, err := request.Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalOrder(*maxOrder)
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) ([]types.Order, error) {
	market := toLocalSymbol(symbol)
	walletType := maxapi.WalletTypeSpot
	if e.MarginSettings.IsMargin {
		walletType = maxapi.WalletTypeMargin
	}

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
	walletType := maxapi.WalletTypeSpot
	if e.MarginSettings.IsMargin {
		walletType = maxapi.WalletTypeMargin
	}

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

func (e *Exchange) queryClosedOrdersByTime(ctx context.Context, symbol string, since, until time.Time, orderByType maxapi.OrderByType) (orders []types.Order, err error) {
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
	walletType := maxapi.WalletTypeSpot
	if e.MarginSettings.IsMargin {
		walletType = maxapi.WalletTypeMargin
	}

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
	walletType := maxapi.WalletTypeSpot
	if e.MarginSettings.IsMargin {
		walletType = maxapi.WalletTypeMargin
	}

	req := e.v3client.NewCancelWalletOrderAllRequest(walletType)
	var orderResponses, err = req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var maxOrders []maxapi.Order
	for _, resp := range orderResponses {
		if resp.Error == nil {
			maxOrders = append(maxOrders, resp.Order)
		}
	}

	return toGlobalOrders(maxOrders)
}

func (e *Exchange) CancelOrdersBySymbol(ctx context.Context, symbol string) ([]types.Order, error) {
	market := toLocalSymbol(symbol)
	walletType := maxapi.WalletTypeSpot
	if e.MarginSettings.IsMargin {
		walletType = maxapi.WalletTypeMargin
	}

	req := e.v3client.NewCancelWalletOrderAllRequest(walletType)
	req.Market(market)

	var orderResponses, err = req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var maxOrders []maxapi.Order
	for _, resp := range orderResponses {
		if resp.Error == nil {
			maxOrders = append(maxOrders, resp.Order)
		}
	}

	return toGlobalOrders(maxOrders)
}

func (e *Exchange) CancelOrdersByGroupID(ctx context.Context, groupID uint32) ([]types.Order, error) {
	walletType := maxapi.WalletTypeSpot
	if e.MarginSettings.IsMargin {
		walletType = maxapi.WalletTypeMargin
	}

	req := e.v3client.NewCancelWalletOrderAllRequest(walletType)
	req.GroupID(groupID)

	var orderResponses, err = req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var maxOrders []maxapi.Order
	for _, resp := range orderResponses {
		if resp.Error == nil {
			maxOrders = append(maxOrders, resp.Order)
		}
	}

	return toGlobalOrders(maxOrders)
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) (err2 error) {
	walletType := maxapi.WalletTypeSpot
	if e.MarginSettings.IsMargin {
		walletType = maxapi.WalletTypeMargin
	}

	var groupIDs = make(map[uint32]struct{})
	var orphanOrders []types.Order
	for _, o := range orders {
		if o.GroupID > 0 {
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
				log.WithError(err).Errorf("group id order cancel error")
				err2 = err
			}
		}
	}

	for _, o := range orphanOrders {
		req := e.v3client.NewCancelOrderRequest()
		if o.OrderID > 0 {
			req.Id(o.OrderID)
		} else if len(o.ClientOrderID) > 0 && o.ClientOrderID != types.NoClientOrderID {
			req.ClientOrderID(o.ClientOrderID)
		} else {
			return fmt.Errorf("order id or client order id is not defined, order=%+v", o)
		}

		if _, err := req.Do(ctx); err != nil {
			log.WithError(err).Errorf("order cancel error")
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

	clientOrderID := NewClientOrderID(o.ClientOrderID)

	req := e.v3client.NewCreateWalletOrderRequest(walletType)
	req.Market(toLocalSymbol(o.Symbol)).
		Side(toLocalSideType(o.Side)).
		Volume(quantityString).
		OrderType(orderType).
		ClientOrderID(clientOrderID)

	if o.GroupID > 0 {
		req.GroupID(strconv.FormatUint(uint64(o.GroupID%math.MaxInt32), 10))
	}

	switch o.Type {
	case types.OrderTypeStopLimit, types.OrderTypeLimit, types.OrderTypeLimitMaker:
		var priceInString string
		if o.Market.Symbol != "" {
			priceInString = o.Market.FormatPrice(o.Price)
		} else {
			priceInString = o.Price.String()
		}
		req.Price(priceInString)
	}

	// set stop price field for limit orders
	switch o.Type {
	case types.OrderTypeStopLimit, types.OrderTypeStopMarket:
		var priceInString string
		if o.Market.Symbol != "" {
			priceInString = o.Market.FormatPrice(o.StopPrice)
		} else {
			priceInString = o.StopPrice.String()
		}
		req.StopPrice(priceInString)
	}

	retOrder, err := req.Do(ctx)
	if err != nil {
		return createdOrder, err
	}

	if retOrder == nil {
		return createdOrder, errors.New("returned nil order")
	}

	createdOrder, err = toGlobalOrder(*retOrder)
	return createdOrder, err
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

	vipLevel, err := e.client.NewGetVipLevelRequest().Do(ctx)
	if err != nil {
		return nil, err
	}

	// MAX returns the fee rate in the following format:
	//  "maker_fee": 0.0005 -> 0.05%
	//  "taker_fee": 0.0015 -> 0.15%
	a := &types.Account{
		AccountType:  types.AccountTypeSpot,
		MarginLevel:  fixedpoint.Zero,
		MakerFeeRate: fixedpoint.NewFromFloat(vipLevel.Current.MakerFee), // 0.15% = 0.0015
		TakerFeeRate: fixedpoint.NewFromFloat(vipLevel.Current.TakerFee), // 0.15% = 0.0015
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

	vipLevel, err := e.client.NewGetVipLevelRequest().Do(ctx)
	if err != nil {
		return nil, err
	}

	// MAX returns the fee rate in the following format:
	//  "maker_fee": 0.0005 -> 0.05%
	//  "taker_fee": 0.0015 -> 0.15%

	a := &types.Account{
		MarginLevel:  fixedpoint.Zero,
		MakerFeeRate: fixedpoint.NewFromFloat(vipLevel.Current.MakerFee), // 0.15% = 0.0015
		TakerFeeRate: fixedpoint.NewFromFloat(vipLevel.Current.TakerFee), // 0.15% = 0.0015
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

	emptyTime := time.Time{}
	if startTime == emptyTime {
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
			From(startTime).
			To(endTime).
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

			// we can convert this later
			status := d.State
			switch d.State {

			case "confirmed":
				status = "completed" // make it compatible with binance

			case "submitting", "submitted", "accepted",
				"rejected", "suspect", "approved", "delisted_processing",
				"processing", "retryable", "sent", "canceled",
				"failed", "pending",
				"kgi_manually_processing", "kgi_manually_confirmed", "kgi_possible_failed",
				"sygna_verifying":

			default:
				status = d.State

			}

			txIDs[d.TxID] = struct{}{}
			withdraw := types.Withdraw{
				Exchange:               types.ExchangeMax,
				ApplyTime:              types.Time(d.CreatedAt),
				Asset:                  toGlobalCurrency(d.Currency),
				Amount:                 d.Amount,
				Address:                "",
				AddressTag:             "",
				TransactionID:          d.TxID,
				TransactionFee:         d.Fee,
				TransactionFeeCurrency: d.FeeCurrency,
				// WithdrawOrderID: d.WithdrawOrderID,
				// Network:         d.Network,
				Status: status,
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
			From(startTime).
			To(endTime).
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
				Address:       d.Address, // not supported
				AddressTag:    "",        // not supported
				TransactionID: d.TxID,
				Status:        toGlobalDepositStatus(d.State),
				Confirmation:  "",
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
// start_time and end_time need to be within 3 days
// without any parameters      -> return trades within 24 hours
// give start_time or end_time -> ignore parameter from_id
// give start_time or from_id  -> order by time asc
// give end_time               -> order by time desc
// limit should b1 1~1000
// For this QueryTrades spec (to be compatible with batch.TradeBatchQuery)
// give LastTradeID       -> ignore start_time (but still can filter the end_time)
// without any parameters -> return trades within 24 hours
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
		req.From(options.LastTradeID)
	} else {
		// option's start_time and end_time need to be within 3 days
		// so if the start_time and end_time is over 3 days, we make end_time down to start_time + 3 days
		if options.StartTime != nil && options.EndTime != nil {
			endTime := *options.EndTime
			startTime := *options.StartTime
			if endTime.Sub(startTime) > 72*time.Hour {
				startTime := *options.StartTime
				endTime = startTime.Add(72 * time.Hour)
			}
			req.StartTime(startTime)
			req.EndTime(endTime)
		} else if options.StartTime != nil {
			req.StartTime(*options.StartTime)
		} else if options.EndTime != nil {
			req.EndTime(*options.EndTime)
		}
	}

	maxTrades, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	for _, t := range maxTrades {
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
	localKLines, err := e.client.PublicService.KLines(toLocalSymbol(symbol), string(interval), *options.StartTime, limit)
	if err != nil {
		return nil, err
	}

	var kLines []types.KLine
	for _, k := range localKLines {
		if options.EndTime != nil && k.StartTime.After(*options.EndTime) {
			break
		}

		kLines = append(kLines, k.KLine())
	}

	return kLines, nil
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

	limits := *resp
	if limit, ok := limits[toLocalCurrency(asset)]; ok {
		return limit, nil
	}

	err = fmt.Errorf("borrowing limit of %s not found", asset)
	return amount, err
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
