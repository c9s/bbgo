package max

import (
	"context"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

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
			PricePrecision:  m.QuoteUnitPrecision,
			VolumePrecision: m.BaseUnitPrecision,
			QuoteCurrency:   toGlobalCurrency(m.QuoteUnit),
			BaseCurrency:    toGlobalCurrency(m.BaseUnit),
			MinNotional:     m.MinQuoteAmount,
			MinAmount:       m.MinQuoteAmount,
			MinLot:          1.0 / math.Pow10(m.BaseUnitPrecision), // make it like 0.0001
			MinQuantity:     m.MinBaseAmount,
			MaxQuantity:     10000.0,
			MinPrice:        1.0 / math.Pow10(m.QuoteUnitPrecision), // used in the price formatter
			MaxPrice:        10000.0,
			TickSize:        1.0 / math.Pow10(m.QuoteUnitPrecision),
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
	numBatches := 5
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

func (e *Exchange) CancelOrdersByGroupID(ctx context.Context, groupID string) ([]types.Order, error) {
	var req = e.client.OrderService.NewOrderCancelAllRequest()
	req.GroupID(groupID)

	var maxOrders, err = req.Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalOrders(maxOrders)
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) (err2 error) {
	var groupIDs = make(map[string]struct{})
	var orphanOrders []types.Order
	for _, o := range orders {
		if len(o.GroupID) > 0 {
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
		} else if len(o.ClientOrderID) > 0 {
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

func (e *Exchange) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error) {
	for _, order := range orders {
		orderType, err := toLocalOrderType(order.Type)
		if err != nil {
			return createdOrders, err
		}

		req := e.client.OrderService.NewCreateOrderRequest().
			Market(toLocalSymbol(order.Symbol)).
			OrderType(string(orderType)).
			Side(toLocalSideType(order.Side)).
			Volume(order.QuantityString)

		if len(order.ClientOrderID) > 0 {
			req.ClientOrderID(order.ClientOrderID)
		} else {
			clientOrderID := uuid.New().String()
			req.ClientOrderID(clientOrderID)
		}

		switch order.Type {
		case types.OrderTypeStopLimit, types.OrderTypeStopMarket:
			if len(order.StopPriceString) == 0 {
				return createdOrders, fmt.Errorf("stop price string can not be empty")
			}

			req.StopPrice(order.StopPriceString)
		}

		if len(order.PriceString) > 0 {
			req.Price(order.PriceString)
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

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
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

	a := &types.Account{
		MakerCommission: 15, // 0.15%
		TakerCommission: 15, // 0.15%
	}

	a.UpdateBalances(balances)
	return a, nil
}

func (e *Exchange) QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []types.Withdraw, err error) {
	startTime := since
	txIDs := map[string]struct{}{}

	for startTime.Before(until) {
		// startTime ~ endTime must be in 90 days
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
			Do(ctx)

		if err != nil {
			return allWithdraws, err
		}

		for _, d := range withdraws {
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
			allWithdraws = append(allWithdraws, types.Withdraw{
				ApplyTime:      time.Unix(d.CreatedAt, 0),
				Asset:          toGlobalCurrency(d.Currency),
				Amount:         util.MustParseFloat(d.Amount),
				Address:        "",
				AddressTag:     "",
				TransactionID:  d.TxID,
				TransactionFee: util.MustParseFloat(d.Fee),
				// WithdrawOrderID: d.WithdrawOrderID,
				// Network:         d.Network,
				Status: status,
			})
		}

		startTime = endTime
	}

	return allWithdraws, nil
}

func (e *Exchange) QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []types.Deposit, err error) {
	startTime := since
	txIDs := map[string]struct{}{}
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
			To(endTime.Unix()).Do(ctx)

		if err != nil {
			return nil, err
		}

		for _, d := range deposits {
			if _, ok := txIDs[d.TxID]; ok {
				continue
			}

			allDeposits = append(allDeposits, types.Deposit{
				Time:          time.Unix(d.CreatedAt, 0),
				Amount:        util.MustParseFloat(d.Amount),
				Asset:         toGlobalCurrency(d.Currency),
				Address:       "", // not supported
				AddressTag:    "", // not supported
				TransactionID: d.TxID,
				Status:        toGlobalDepositStatus(d.State),
			})
		}

		startTime = endTime
	}

	return allDeposits, err
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
			Available: fixedpoint.Must(fixedpoint.NewFromString(a.Balance)),
			Locked:    fixedpoint.Must(fixedpoint.NewFromString(a.Locked)),
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
		localTrade, err := toGlobalTrade(t)
		if err != nil {
			logger.WithError(err).Errorf("can not convert trade: %+v", t)
			continue
		}

		logger.Infof("T: id=%d % 4s %s P=%f Q=%f %s", localTrade.ID, localTrade.Symbol, localTrade.Side, localTrade.Price, localTrade.Quantity, localTrade.Time)

		trades = append(trades, *localTrade)
	}

	return trades, nil
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	var limit = 5000
	if options.Limit > 0 {
		// default limit == 500
		limit = options.Limit
	}

	// workaround for the kline query, because MAX does not support query by end time
	// so we need to use the given end time and the limit number to calculate the start time
	if options.EndTime != nil && options.StartTime == nil {
		startTime := options.EndTime.Add(- time.Duration(limit) * interval.Duration())
		options.StartTime = &startTime
	}

	if options.StartTime == nil {
		return nil, errors.New("start time can not be empty")
	}

	log.Infof("querying kline %s %s %+v", symbol, interval, options)

	// avoid rate limit
	time.Sleep(100 * time.Millisecond)

	localKLines, err := e.client.PublicService.KLines(toLocalSymbol(symbol), string(interval), *options.StartTime, limit)
	if err != nil {
		return nil, err
	}

	var kLines []types.KLine
	for _, k := range localKLines {
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
