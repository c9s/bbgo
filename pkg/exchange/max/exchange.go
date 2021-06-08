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
	"golang.org/x/time/rate"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

var closedOrderQueryLimiter = rate.NewLimiter(rate.Every(5*time.Second), 1)
var tradeQueryLimiter = rate.NewLimiter(rate.Every(3*time.Second), 1)
var accountQueryLimiter = rate.NewLimiter(rate.Every(3*time.Second), 1)
var marketDataLimiter = rate.NewLimiter(rate.Every(2*time.Second), 10)

var log = logrus.WithField("exchange", "max")

type Exchange struct {
	client      *maxapi.RestClient
	key, secret string
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
		secret: secret,
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
		Volume: util.MustParseFloat(ticker.Volume),
		Last:   util.MustParseFloat(ticker.Last),
		Open:   util.MustParseFloat(ticker.Open),
		High:   util.MustParseFloat(ticker.High),
		Low:    util.MustParseFloat(ticker.Low),
		Buy:    util.MustParseFloat(ticker.Buy),
		Sell:   util.MustParseFloat(ticker.Sell),
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
				Volume: util.MustParseFloat(v.Volume),
				Last:   util.MustParseFloat(v.Last),
				Open:   util.MustParseFloat(v.Open),
				High:   util.MustParseFloat(v.High),
				Low:    util.MustParseFloat(v.Low),
				Buy:    util.MustParseFloat(v.Buy),
				Sell:   util.MustParseFloat(v.Sell),
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
			MaxQuantity: 10000.0,
			StepSize:    1.0 / math.Pow10(m.BaseUnitPrecision), // make it like 0.0001

			MinPrice: 1.0 / math.Pow10(m.QuoteUnitPrecision), // used in the price formatter
			MaxPrice: 10000.0,
			TickSize: 1.0 / math.Pow10(m.QuoteUnitPrecision),
		}

		markets[symbol] = market
	}

	return markets, nil
}

func (e *Exchange) NewStream() types.Stream {
	return NewStream(e.key, e.secret)
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	maxOrders, err := e.client.OrderService.Open(toLocalSymbol(symbol), maxapi.QueryOrderOptions{})
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
func (e *Exchange) QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) (orders []types.Order, err error) {
	if err := closedOrderQueryLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	numBatches := 3
	limit := 1000 // max limit = 1000
	offset := limit * numBatches
	orderIDs := make(map[uint64]struct{}, limit*2)

	for ; offset > 0; offset -= limit {
		log.Infof("querying %s closed orders offset %d ~ ", symbol, offset)

		maxOrders, err := e.client.OrderService.Closed(toLocalSymbol(symbol), maxapi.QueryOrderOptions{
			Offset: offset,
			Limit:  limit,
		})
		if err != nil {
			return orders, err
		}

		if len(maxOrders) == 0 {
			break
		}

		for _, maxOrder := range maxOrders {
			if maxOrder.CreatedAt.Before(since) {
				continue
			}

			if maxOrder.CreatedAt.After(until) {
				return orders, err
			}

			order, err := toGlobalOrder(maxOrder)
			if err != nil {
				return orders, err
			}

			if _, ok := orderIDs[order.OrderID]; ok {
				log.Infof("skipping duplicated order: %d", order.OrderID)
			}

			orderIDs[order.OrderID] = struct{}{}
			orders = append(orders, *order)
		}
	}

	return orders, err
}

func (e *Exchange) CancelAllOrders(ctx context.Context) ([]types.Order, error) {
	var req = e.client.OrderService.NewOrderCancelAllRequest()
	var maxOrders, err = req.Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalOrders(maxOrders)
}

func (e *Exchange) CancelOrdersBySymbol(ctx context.Context, symbol string) ([]types.Order, error) {
	var req = e.client.OrderService.NewOrderCancelAllRequest()
	req.Market(toLocalSymbol(symbol))

	var maxOrders, err = req.Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalOrders(maxOrders)
}

func (e *Exchange) CancelOrdersByGroupID(ctx context.Context, groupID uint32) ([]types.Order, error) {
	var req = e.client.OrderService.NewOrderCancelAllRequest()
	req.GroupID(groupID)

	var maxOrders, err = req.Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalOrders(maxOrders)
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) (err2 error) {
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
			var req = e.client.OrderService.NewOrderCancelAllRequest()
			req.GroupID(groupID)

			if _, err := req.Do(ctx); err != nil {
				log.WithError(err).Errorf("group id order cancel error")
				err2 = err
			}
		}
	}

	for _, o := range orphanOrders {
		var req = e.client.OrderService.NewOrderCancelRequest()
		if o.OrderID > 0 {
			req.ID(o.OrderID)
		} else if len(o.ClientOrderID) > 0 && o.ClientOrderID != types.NoClientOrderID {
			req.ClientOrderID(o.ClientOrderID)
		} else {
			return fmt.Errorf("order id or client order id is not defined, order=%+v", o)
		}

		if err := req.Do(ctx); err != nil {
			log.WithError(err).Errorf("order cancel error")
			err2 = err
		}
	}

	return err2
}

func toMaxSubmitOrder(o types.SubmitOrder) (*maxapi.Order, error) {
	symbol := toLocalSymbol(o.Symbol)
	orderType, err := toLocalOrderType(o.Type)
	if err != nil {
		return nil, err
	}

	volumeInString := o.QuantityString
	if len(volumeInString) == 0 {
		if o.Market.Symbol != "" {
			volumeInString = o.Market.FormatQuantity(o.Quantity)
		} else {
			volumeInString = strconv.FormatFloat(o.Quantity, 'f', -1, 64)
		}
	}

	maxOrder := maxapi.Order{
		Market:    symbol,
		Side:      toLocalSideType(o.Side),
		OrderType: orderType,
		// Price:     priceInString,
		Volume: volumeInString,
	}

	if o.GroupID > 0 {
		maxOrder.GroupID = o.GroupID
	}

	clientOrderID := NewClientOrderID(o.ClientOrderID)
	if len(clientOrderID) > 0 {
		maxOrder.ClientOID = clientOrderID
	}

	switch o.Type {
	case types.OrderTypeStopLimit, types.OrderTypeLimit, types.OrderTypeLimitMaker, types.OrderTypeIOCLimit:
		priceInString := o.PriceString
		if len(priceInString) == 0 {
			if o.Market.Symbol != "" {
				priceInString = o.Market.FormatPrice(o.Price)
			} else {
				priceInString = strconv.FormatFloat(o.Price, 'f', -1, 64)
			}
		}
		maxOrder.Price = priceInString
	}

	// set stop price field for limit orders
	switch o.Type {
	case types.OrderTypeStopLimit, types.OrderTypeStopMarket:
		if len(o.StopPriceString) == 0 {
			return nil, fmt.Errorf("stop price string can not be empty")
		}

		priceInString := o.StopPriceString
		if len(priceInString) == 0 {
			if o.Market.Symbol != "" {
				priceInString = o.Market.FormatPrice(o.StopPrice)
			} else {
				priceInString = strconv.FormatFloat(o.StopPrice, 'f', -1, 64)
			}
		}

		maxOrder.StopPrice = priceInString
	}

	return &maxOrder, nil
}

func (e *Exchange) Withdrawal(ctx context.Context, asset string, amount fixedpoint.Value, address string, options *types.WithdrawalOptions) error {
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

func (e *Exchange) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error) {
	if len(orders) > 1 && len(orders) < 15 {
		var ordersBySymbol = map[string][]maxapi.Order{}
		for _, o := range orders {
			maxOrder, err := toMaxSubmitOrder(o)
			if err != nil {
				return nil, err
			}

			ordersBySymbol[maxOrder.Market] = append(ordersBySymbol[maxOrder.Market], *maxOrder)
		}

		for symbol, orders := range ordersBySymbol {
			req := e.client.OrderService.NewCreateMultiOrderRequest()
			req.Market(symbol)
			req.AddOrders(orders...)

			orderResponses, err := req.Do(ctx)
			if err != nil {
				return createdOrders, err
			}

			for _, resp := range *orderResponses {
				if len(resp.Error) > 0 {
					log.Errorf("multi-order submit error: %s", resp.Error)
					continue
				}

				o, err := toGlobalOrder(resp.Order)
				if err != nil {
					return createdOrders, err
				}

				createdOrders = append(createdOrders, *o)
			}
		}

		return createdOrders, nil
	}

	for _, order := range orders {
		maxOrder, err := toMaxSubmitOrder(order)
		if err != nil {
			return createdOrders, err
		}

		req := e.client.OrderService.NewCreateOrderRequest().
			Market(maxOrder.Market).
			Side(maxOrder.Side).
			Volume(maxOrder.Volume).
			OrderType(string(maxOrder.OrderType))

		if len(maxOrder.ClientOID) > 0 {
			req.ClientOrderID(maxOrder.ClientOID)
		}

		if len(maxOrder.Price) > 0 {
			req.Price(maxOrder.Price)
		}

		if len(maxOrder.StopPrice) > 0 {
			req.StopPrice(maxOrder.StopPrice)
		}

		retOrder, err := req.Do(ctx)
		if err != nil {
			return createdOrders, err
		}
		if retOrder == nil {
			return createdOrders, errors.New("returned nil order")
		}

		createdOrder, err := toGlobalOrder(*retOrder)
		if err != nil {
			return createdOrders, err
		}

		createdOrders = append(createdOrders, *createdOrder)
	}

	return createdOrders, err
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

	userInfo, err := e.client.AccountService.Me()
	if err != nil {
		return nil, err
	}

	var balances = make(types.BalanceMap)
	for _, a := range userInfo.Accounts {
		balances[toGlobalCurrency(a.Currency)] = types.Balance{
			Currency:  toGlobalCurrency(a.Currency),
			Available: fixedpoint.Must(fixedpoint.NewFromString(a.Balance)),
			Locked:    fixedpoint.Must(fixedpoint.NewFromString(a.Locked)),
		}
	}

	vipLevel, err := e.client.AccountService.VipLevel()
	if err != nil {
		return nil, err
	}

	// MAX returns the fee rate in the following format:
	//  "maker_fee": 0.0005 -> 0.05%
	//  "taker_fee": 0.0015 -> 0.15%
	a := &types.Account{
		MakerFeeRate: fixedpoint.NewFromFloat(vipLevel.Current.MakerFee), // 0.15% = 0.0015
		TakerFeeRate: fixedpoint.NewFromFloat(vipLevel.Current.TakerFee), // 0.15% = 0.0015
	}

	a.UpdateBalances(balances)
	return a, nil
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
				Amount:                 util.MustParseFloat(d.Amount),
				Address:                "",
				AddressTag:             "",
				TransactionID:          d.TxID,
				TransactionFee:         util.MustParseFloat(d.Fee),
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
				Amount:        util.MustParseFloat(d.Amount),
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

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	if err := accountQueryLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	accounts, err := e.client.AccountService.Accounts()
	if err != nil {
		return nil, err
	}

	var balances = make(types.BalanceMap)

	for _, a := range accounts {
		balances[toGlobalCurrency(a.Currency)] = types.Balance{
			Currency:  toGlobalCurrency(a.Currency),
			Available: fixedpoint.Must(fixedpoint.NewFromString(a.Balance)),
			Locked:    fixedpoint.Must(fixedpoint.NewFromString(a.Locked)),
		}
	}

	return balances, nil
}

func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) (trades []types.Trade, err error) {
	if err := tradeQueryLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	req := e.client.TradeService.NewPrivateTradeRequest()
	req.Market(toLocalSymbol(symbol))

	if options.Limit > 0 {
		req.Limit(options.Limit)
	} else {
		req.Limit(1000)
	}

	// MAX uses exclusive last trade ID
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
		localTrade, err := toGlobalTrade(t)
		if err != nil {
			logger.WithError(err).Errorf("can not convert trade: %+v", t)
			continue
		}

		trades = append(trades, *localTrade)
	}

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
		req := e.client.RewardService.NewRewardsRequest()
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

func (e *Exchange) QueryAveragePrice(ctx context.Context, symbol string) (float64, error) {
	ticker, err := e.client.PublicService.Ticker(toLocalSymbol(symbol))
	if err != nil {
		return 0, err
	}

	return (util.MustParseFloat(ticker.Sell) + util.MustParseFloat(ticker.Buy)) / 2, nil
}
