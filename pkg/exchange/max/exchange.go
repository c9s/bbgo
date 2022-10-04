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

// closedOrderQueryLimiter is used for the closed orders query rate limit, 1 request per second
var closedOrderQueryLimiter = rate.NewLimiter(rate.Every(1*time.Second), 1)
var tradeQueryLimiter = rate.NewLimiter(rate.Every(3*time.Second), 1)
var accountQueryLimiter = rate.NewLimiter(rate.Every(3*time.Second), 1)
var marketDataLimiter = rate.NewLimiter(rate.Every(2*time.Second), 10)

var log = logrus.WithField("exchange", "max")

type Exchange struct {
	types.MarginSettings

	key, secret string
	client      *maxapi.RestClient

	v3order  *v3.OrderService
	v3margin *v3.MarginService
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
		v3order:  &v3.OrderService{Client: client},
		v3margin: &v3.MarginService{Client: client},
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
		Volume: fixedpoint.MustNewFromString(ticker.Volume),
		Last:   fixedpoint.MustNewFromString(ticker.Last),
		Open:   fixedpoint.MustNewFromString(ticker.Open),
		High:   fixedpoint.MustNewFromString(ticker.High),
		Low:    fixedpoint.MustNewFromString(ticker.Low),
		Buy:    fixedpoint.MustNewFromString(ticker.Buy),
		Sell:   fixedpoint.MustNewFromString(ticker.Sell),
	}, nil
}

func (e *Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	if err := marketDataLimiter.Wait(ctx); err != nil {
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

		maxTickers, err := e.client.PublicService.Tickers()
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
				Volume: fixedpoint.MustNewFromString(v.Volume),
				Last:   fixedpoint.MustNewFromString(v.Last),
				Open:   fixedpoint.MustNewFromString(v.Open),
				High:   fixedpoint.MustNewFromString(v.High),
				Low:    fixedpoint.MustNewFromString(v.Low),
				Buy:    fixedpoint.MustNewFromString(v.Buy),
				Sell:   fixedpoint.MustNewFromString(v.Sell),
			}
		}
	}

	return tickers, nil
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	log.Info("querying market info...")

	remoteMarkets, err := e.client.PublicService.Markets()
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

	maxTrades, err := e.v3order.NewGetOrderTradesRequest().OrderID(uint64(orderID)).Do(ctx)
	if err != nil {
		return nil, err
	}

	var trades []types.Trade
	for _, t := range maxTrades {
		localTrade, err := toGlobalTrade(t)
		if err != nil {
			log.WithError(err).Errorf("can not convert trade: %+v", t)
			continue
		}

		trades = append(trades, *localTrade)
	}

	// ensure everything is sorted ascending
	trades = types.SortTradesAscending(trades)
	return trades, nil
}

func (e *Exchange) QueryOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	if q.OrderID == "" {
		return nil, errors.New("max.QueryOrder: OrderID is required parameter")
	}

	orderID, err := strconv.ParseInt(q.OrderID, 10, 64)
	if err != nil {
		return nil, err
	}

	maxOrder, err := e.v3order.NewGetOrderRequest().Id(uint64(orderID)).Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalOrder(*maxOrder)
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	market := toLocalSymbol(symbol)
	walletType := maxapi.WalletTypeSpot
	if e.MarginSettings.IsMargin {
		walletType = maxapi.WalletTypeMargin
	}

	maxOrders, err := e.v3order.NewGetWalletOpenOrdersRequest(walletType).Market(market).Do(ctx)
	if err != nil {
		return orders, err
	}

	for _, maxOrder := range maxOrders {
		order, err := toGlobalOrder(maxOrder)
		if err != nil {
			return orders, err
		}

		orders = append(orders, *order)
	}

	return orders, err
}

// lastOrderID is not supported on MAX
func (e *Exchange) QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) ([]types.Order, error) {
	log.Warn("!!!MAX EXCHANGE API NOTICE!!!")
	log.Warn("the since/until conditions will not be effected on closed orders query, max exchange does not support time-range-based query")
	return e.queryClosedOrdersByLastOrderID(ctx, symbol, lastOrderID)
}

func (e *Exchange) queryClosedOrdersByLastOrderID(ctx context.Context, symbol string, lastOrderID uint64) (orders []types.Order, err error) {
	if err := closedOrderQueryLimiter.Wait(ctx); err != nil {
		return orders, err
	}

	market := toLocalSymbol(symbol)
	walletType := maxapi.WalletTypeSpot
	if e.MarginSettings.IsMargin {
		walletType = maxapi.WalletTypeMargin
	}

	req := e.v3order.NewGetWalletOrderHistoryRequest(walletType).Market(market)
	if lastOrderID == 0 {
		lastOrderID = 1
	}

	req.FromID(lastOrderID)
	req.Limit(1000)

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

	orders = types.SortOrdersAscending(orders)
	return orders, nil
}

func (e *Exchange) CancelAllOrders(ctx context.Context) ([]types.Order, error) {
	walletType := maxapi.WalletTypeSpot
	if e.MarginSettings.IsMargin {
		walletType = maxapi.WalletTypeMargin
	}

	req := e.v3order.NewCancelWalletOrderAllRequest(walletType)
	var maxOrders, err = req.Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalOrders(maxOrders)
}

func (e *Exchange) CancelOrdersBySymbol(ctx context.Context, symbol string) ([]types.Order, error) {
	market := toLocalSymbol(symbol)
	walletType := maxapi.WalletTypeSpot
	if e.MarginSettings.IsMargin {
		walletType = maxapi.WalletTypeMargin
	}

	req := e.v3order.NewCancelWalletOrderAllRequest(walletType)
	req.Market(market)

	maxOrders, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalOrders(maxOrders)
}

func (e *Exchange) CancelOrdersByGroupID(ctx context.Context, groupID uint32) ([]types.Order, error) {
	walletType := maxapi.WalletTypeSpot
	if e.MarginSettings.IsMargin {
		walletType = maxapi.WalletTypeMargin
	}

	req := e.v3order.NewCancelWalletOrderAllRequest(walletType)
	req.GroupID(groupID)

	maxOrders, err := req.Do(ctx)
	if err != nil {
		return nil, err
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
			req := e.v3order.NewCancelWalletOrderAllRequest(walletType)
			req.GroupID(groupID)

			if _, err := req.Do(ctx); err != nil {
				log.WithError(err).Errorf("group id order cancel error")
				err2 = err
			}
		}
	}

	for _, o := range orphanOrders {
		req := e.v3order.NewCancelOrderRequest()
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

func toMaxSubmitOrder(o types.SubmitOrder) (*maxapi.SubmitOrder, error) {
	symbol := toLocalSymbol(o.Symbol)
	orderType, err := toLocalOrderType(o.Type)
	if err != nil {
		return nil, err
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

	maxOrder := maxapi.SubmitOrder{
		Market:    symbol,
		Side:      toLocalSideType(o.Side),
		OrderType: orderType,
		Volume:    quantityString,
	}

	if o.GroupID > 0 {
		maxOrder.GroupID = o.GroupID
	}

	clientOrderID := NewClientOrderID(o.ClientOrderID)
	if len(clientOrderID) > 0 {
		maxOrder.ClientOID = clientOrderID
	}

	switch o.Type {
	case types.OrderTypeStopLimit, types.OrderTypeLimit, types.OrderTypeLimitMaker:
		var priceInString string
		if o.Market.Symbol != "" {
			priceInString = o.Market.FormatPrice(o.Price)
		} else {
			priceInString = o.Price.String()
		}
		maxOrder.Price = priceInString
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
		maxOrder.StopPrice = priceInString
	}

	return &maxOrder, nil
}

func (e *Exchange) Withdraw(ctx context.Context, asset string, amount fixedpoint.Value, address string, options *types.WithdrawalOptions) error {
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

	req := e.v3order.NewCreateWalletOrderRequest(walletType)
	req.Market(toLocalSymbol(o.Symbol)).
		Side(toLocalSideType(o.Side)).
		Volume(quantityString).
		OrderType(orderType).
		ClientOrderID(clientOrderID)

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

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	if err := accountQueryLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	vipLevel, err := e.client.AccountService.NewGetVipLevelRequest().Do(ctx)
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

	balances, err := e.QueryAccountBalances(ctx)
	if err != nil {
		return nil, err
	}
	a.UpdateBalances(balances)

	if e.MarginSettings.IsMargin {
		a.AccountType = types.AccountTypeMargin

		req := e.v3margin.NewGetMarginADRatioRequest()
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
	if err := accountQueryLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	walletType := maxapi.WalletTypeSpot
	if e.MarginSettings.IsMargin {
		walletType = maxapi.WalletTypeMargin
	}

	req := e.v3order.NewGetWalletAccountsRequest(walletType)
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
			NetAsset:  b.Balance.Add(b.Locked).Sub(b.Debt),
			Borrowed:  b.Borrowed,
			Interest:  b.Interest,
		}
	}

	return balances, nil
}

func (e *Exchange) QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []types.Withdraw, err error) {
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
		req := e.client.AccountService.NewGetWithdrawalHistoryRequest()
		if len(asset) > 0 {
			req.Currency(toLocalCurrency(asset))
		}

		withdraws, err := req.
			From(startTime.Unix()).
			To(endTime.Unix()).
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
				ApplyTime:              types.Time(time.Unix(d.CreatedAt, 0)),
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
			startTime = time.Unix(withdraws[0].CreatedAt, 0)
		}
	}

	return allWithdraws, nil
}

func (e *Exchange) QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []types.Deposit, err error) {
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

		req := e.client.AccountService.NewGetDepositHistoryRequest()
		if len(asset) > 0 {
			req.Currency(toLocalCurrency(asset))
		}

		deposits, err := req.
			From(startTime.Unix()).
			To(endTime.Unix()).
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
				Time:          types.Time(time.Unix(d.CreatedAt, 0)),
				Amount:        d.Amount,
				Asset:         toGlobalCurrency(d.Currency),
				Address:       "", // not supported
				AddressTag:    "", // not supported
				TransactionID: d.TxID,
				Status:        toGlobalDepositStatus(d.State),
			})
		}

		if len(deposits) < limit {
			startTime = endTime
		} else {
			startTime = time.Unix(deposits[0].CreatedAt, 0)
		}
	}

	return allDeposits, err
}

func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) (trades []types.Trade, err error) {
	if err := tradeQueryLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	market := toLocalSymbol(symbol)
	walletType := maxapi.WalletTypeSpot
	if e.MarginSettings.IsMargin {
		walletType = maxapi.WalletTypeMargin
	}

	req := e.v3order.NewGetWalletTradesRequest(walletType)
	req.Market(market)

	if options.Limit > 0 {
		req.Limit(uint64(options.Limit))
	} else {
		req.Limit(1000)
	}

	// MAX uses exclusive last trade ID
	// the timestamp parameter is used for reverse order, we can't use it.
	if options.LastTradeID > 0 {
		req.From(options.LastTradeID)
	}

	maxTrades, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	for _, t := range maxTrades {
		localTrade, err := toGlobalTrade(t)
		if err != nil {
			log.WithError(err).Errorf("can not convert trade: %+v", t)
			continue
		}

		trades = append(trades, *localTrade)
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
func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	if err := marketDataLimiter.Wait(ctx); err != nil {
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

	return fixedpoint.MustNewFromString(ticker.Sell).
		Add(fixedpoint.MustNewFromString(ticker.Buy)).Div(Two), nil
}

func (e *Exchange) RepayMarginAsset(ctx context.Context, asset string, amount fixedpoint.Value) error {
	req := e.v3margin.NewMarginRepayRequest()
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
	req := e.v3margin.NewMarginLoanRequest()
	req.Currency(toLocalCurrency(asset))
	req.Amount(amount.String())
	resp, err := req.Do(ctx)
	if err != nil {
		return err
	}

	log.Infof("margin borrow: %v", resp)
	return nil
}

func (e *Exchange) QueryMarginAssetMaxBorrowable(ctx context.Context, asset string) (amount fixedpoint.Value, err error) {
	req := e.v3margin.NewGetMarginBorrowingLimitsRequest()
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
