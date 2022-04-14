package binance

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/spf13/viper"

	"go.uber.org/multierr"

	"golang.org/x/time/rate"

	"github.com/adshao/go-binance/v2"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

const BNB = "BNB"

const BinanceUSBaseURL = "https://api.binance.us"
const BinanceTestBaseURL = "https://testnet.binance.vision"
const BinanceUSWebSocketURL = "wss://stream.binance.us:9443"
const WebSocketURL = "wss://stream.binance.com:9443"
const WebSocketTestURL = "wss://testnet.binance.vision"
const FutureTestBaseURL = "https://testnet.binancefuture.com"
const FuturesWebSocketURL = "wss://fstream.binance.com"
const FuturesWebSocketTestURL = "wss://stream.binancefuture.com"

// 5 per second and a 2 initial bucket
var orderLimiter = rate.NewLimiter(5, 2)

var log = logrus.WithFields(logrus.Fields{
	"exchange": "binance",
})

func init() {
	_ = types.Exchange(&Exchange{})
	_ = types.MarginExchange(&Exchange{})
	_ = types.FuturesExchange(&Exchange{})

	// FIXME: this is not effected since dotenv is loaded in the rootCmd, not in the init function
	if ok, _ := strconv.ParseBool(os.Getenv("DEBUG_BINANCE_STREAM")); ok {
		log.Level = logrus.DebugLevel
	}
}

func isBinanceUs() bool {
	v, err := strconv.ParseBool(os.Getenv("BINANCE_US"))
	return err == nil && v
}

func paperTrade() bool {
	v, ok := util.GetEnvVarBool("PAPER_TRADE")
	return ok && v
}

type Exchange struct {
	types.MarginSettings
	types.FuturesSettings

	key, secret   string
	Client        *binance.Client // Spot & Margin
	futuresClient *futures.Client // USDT-M Futures
	// deliveryClient	*delivery.Client // Coin-M Futures
}

var timeSetter sync.Once

func New(key, secret string) *Exchange {
	var client = binance.NewClient(key, secret)
	client.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	client.Debug = viper.GetBool("debug-binance-client")

	var futuresClient = binance.NewFuturesClient(key, secret)
	futuresClient.HTTPClient = &http.Client{Timeout: 15 * time.Second}

	if isBinanceUs() {
		client.BaseURL = BinanceUSBaseURL
	}

	if paperTrade() {
		client.BaseURL = BinanceTestBaseURL
		futuresClient.BaseURL = FutureTestBaseURL
	}

	var err error
	if len(key) > 0 && len(secret) > 0 {
		timeSetter.Do(func() {
			_, err = client.NewSetServerTimeService().Do(context.Background())
			if err != nil {
				log.WithError(err).Error("can not set server time")
			}

			_, err = futuresClient.NewSetServerTimeService().Do(context.Background())
			if err != nil {
				log.WithError(err).Error("can not set server time")
			}
		})
	}

	return &Exchange{
		key:           key,
		secret:        secret,
		Client:        client,
		futuresClient: futuresClient,
		// deliveryClient: deliveryClient,
	}
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeBinance
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	req := e.Client.NewListPriceChangeStatsService()
	req.Symbol(strings.ToUpper(symbol))
	stats, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalTicker(stats[0])
}

func (e *Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	var tickers = make(map[string]types.Ticker)

	if len(symbol) == 1 {
		ticker, err := e.QueryTicker(ctx, symbol[0])
		if err != nil {
			return nil, err
		}

		tickers[strings.ToUpper(symbol[0])] = *ticker
		return tickers, nil
	}

	var req = e.Client.NewListPriceChangeStatsService()
	changeStats, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	m := make(map[string]struct{})
	exists := struct{}{}

	for _, s := range symbol {
		m[s] = exists
	}

	for _, stats := range changeStats {
		if _, ok := m[stats.Symbol]; len(symbol) != 0 && !ok {
			continue
		}

		tick := types.Ticker{
			Volume: fixedpoint.MustNewFromString(stats.Volume),
			Last:   fixedpoint.MustNewFromString(stats.LastPrice),
			Open:   fixedpoint.MustNewFromString(stats.OpenPrice),
			High:   fixedpoint.MustNewFromString(stats.HighPrice),
			Low:    fixedpoint.MustNewFromString(stats.LowPrice),
			Buy:    fixedpoint.MustNewFromString(stats.BidPrice),
			Sell:   fixedpoint.MustNewFromString(stats.AskPrice),
			Time:   time.Unix(0, stats.CloseTime*int64(time.Millisecond)),
		}

		tickers[stats.Symbol] = tick
	}

	return tickers, nil
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {

	if e.IsFutures {
		exchangeInfo, err := e.futuresClient.NewExchangeInfoService().Do(ctx)
		if err != nil {
			return nil, err
		}

		markets := types.MarketMap{}
		for _, symbol := range exchangeInfo.Symbols {
			markets[symbol.Symbol] = toGlobalFuturesMarket(symbol)
		}

		return markets, nil
	}

	exchangeInfo, err := e.Client.NewExchangeInfoService().Do(ctx)
	if err != nil {
		return nil, err
	}

	markets := types.MarketMap{}
	for _, symbol := range exchangeInfo.Symbols {
		markets[symbol.Symbol] = toGlobalMarket(symbol)
	}

	return markets, nil
}

func (e *Exchange) QueryAveragePrice(ctx context.Context, symbol string) (fixedpoint.Value, error) {
	resp, err := e.Client.NewAveragePriceService().Symbol(symbol).Do(ctx)
	if err != nil {
		return fixedpoint.Zero, err
	}

	return fixedpoint.MustNewFromString(resp.Price), nil
}

func (e *Exchange) NewStream() types.Stream {
	stream := NewStream(e, e.Client, e.futuresClient)
	stream.MarginSettings = e.MarginSettings
	stream.FuturesSettings = e.FuturesSettings
	return stream
}

func (e *Exchange) QueryMarginAccount(ctx context.Context) (*types.Account, error) {
	account, err := e.Client.NewGetMarginAccountService().Do(ctx)
	if err != nil {
		return nil, err
	}

	a := &types.Account{
		AccountType: types.AccountTypeMargin,
		MarginInfo:  toGlobalMarginAccountInfo(account), // In binance GO api, Account define account info which mantain []*AccountAsset and []*AccountPosition.
	}

	return a, nil
}

func (e *Exchange) QueryIsolatedMarginAccount(ctx context.Context, symbols ...string) (*types.Account, error) {
	req := e.Client.NewGetIsolatedMarginAccountService()
	if len(symbols) > 0 {
		req.Symbols(symbols...)
	}

	account, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	a := &types.Account{
		AccountType:        types.AccountTypeMargin,
		IsolatedMarginInfo: toGlobalIsolatedMarginAccountInfo(account), // In binance GO api, Account define account info which mantain []*AccountAsset and []*AccountPosition.
	}

	return a, nil
}

func (e *Exchange) Withdrawal(ctx context.Context, asset string, amount fixedpoint.Value, address string, options *types.WithdrawalOptions) error {
	req := e.Client.NewCreateWithdrawService()
	req.Coin(asset)
	req.Address(address)
	req.Amount(fmt.Sprintf("%f", amount.Float64()))

	if options != nil {
		if options.Network != "" {
			req.Network(options.Network)
		}
		if options.AddressTag != "" {
			req.Network(options.AddressTag)
		}
	}

	response, err := req.Do(ctx)
	if err != nil {
		return err
	}

	log.Infof("withdrawal request sent, response: %+v", response)
	return nil
}

func (e *Exchange) QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (allWithdraws []types.Withdraw, err error) {
	startTime := since

	var emptyTime = time.Time{}
	if startTime == emptyTime {
		startTime, err = getLaunchDate()
		if err != nil {
			return nil, err
		}
	}

	txIDs := map[string]struct{}{}

	for startTime.Before(until) {
		// startTime ~ endTime must be in 90 days
		endTime := startTime.AddDate(0, 0, 60)
		if endTime.After(until) {
			endTime = until
		}

		req := e.Client.NewListWithdrawsService()
		if len(asset) > 0 {
			req.Coin(asset)
		}

		withdraws, err := req.
			StartTime(startTime.UnixNano() / int64(time.Millisecond)).
			EndTime(endTime.UnixNano() / int64(time.Millisecond)).
			Do(ctx)

		if err != nil {
			return allWithdraws, err
		}

		for _, d := range withdraws {
			if _, ok := txIDs[d.TxID]; ok {
				continue
			}

			status := ""
			switch d.Status {
			case 0:
				status = "email_sent"
			case 1:
				status = "cancelled"
			case 2:
				status = "awaiting_approval"
			case 3:
				status = "rejected"
			case 4:
				status = "processing"
			case 5:
				status = "failure"
			case 6:
				status = "completed"

			default:
				status = fmt.Sprintf("unsupported code: %d", d.Status)
			}

			txIDs[d.TxID] = struct{}{}

			// 2006-01-02 15:04:05
			applyTime, err := time.Parse("2006-01-02 15:04:05", d.ApplyTime)
			if err != nil {
				return nil, err
			}

			allWithdraws = append(allWithdraws, types.Withdraw{
				Exchange:        types.ExchangeBinance,
				ApplyTime:       types.Time(applyTime),
				Asset:           d.Coin,
				Amount:          fixedpoint.MustNewFromString(d.Amount),
				Address:         d.Address,
				TransactionID:   d.TxID,
				TransactionFee:  fixedpoint.MustNewFromString(d.TransactionFee),
				WithdrawOrderID: d.WithdrawOrderID,
				Network:         d.Network,
				Status:          status,
			})
		}

		startTime = endTime
	}

	return allWithdraws, nil
}

func (e *Exchange) QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []types.Deposit, err error) {
	startTime := since

	var emptyTime = time.Time{}
	if startTime == emptyTime {
		startTime, err = getLaunchDate()
		if err != nil {
			return nil, err
		}
	}
	txIDs := map[string]struct{}{}
	for startTime.Before(until) {

		// startTime ~ endTime must be in 90 days
		endTime := startTime.AddDate(0, 0, 60)
		if endTime.After(until) {
			endTime = until
		}

		req := e.Client.NewListDepositsService()
		if len(asset) > 0 {
			req.Coin(asset)
		}

		deposits, err := req.
			StartTime(startTime.UnixNano() / int64(time.Millisecond)).
			EndTime(endTime.UnixNano() / int64(time.Millisecond)).
			Do(ctx)

		if err != nil {
			return nil, err
		}

		for _, d := range deposits {
			if _, ok := txIDs[d.TxID]; ok {
				continue
			}

			// 0(0:pending,6: credited but cannot withdraw, 1:success)
			status := types.DepositStatus(fmt.Sprintf("code: %d", d.Status))

			switch d.Status {
			case 0:
				status = types.DepositPending
			case 6:
				// https://www.binance.com/en/support/faq/115003736451
				status = types.DepositCredited
			case 1:
				status = types.DepositSuccess
			}

			txIDs[d.TxID] = struct{}{}
			allDeposits = append(allDeposits, types.Deposit{
				Exchange:      types.ExchangeBinance,
				Time:          types.Time(time.Unix(0, d.InsertTime*int64(time.Millisecond))),
				Asset:         d.Coin,
				Amount:        fixedpoint.MustNewFromString(d.Amount),
				Address:       d.Address,
				AddressTag:    d.AddressTag,
				TransactionID: d.TxID,
				Status:        status,
			})
		}

		startTime = endTime
	}

	return allDeposits, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	account, err := e.QueryAccount(ctx)
	if err != nil {
		return nil, err
	}

	return account.Balances(), nil
}

func (e *Exchange) PlatformFeeCurrency() string {
	return BNB
}

func (e *Exchange) QuerySpotAccount(ctx context.Context) (*types.Account, error) {
	account, err := e.Client.NewGetAccountService().Do(ctx)
	if err != nil {
		return nil, err
	}

	var balances = map[string]types.Balance{}
	for _, b := range account.Balances {
		balances[b.Asset] = types.Balance{
			Currency:  b.Asset,
			Available: fixedpoint.MustNewFromString(b.Free),
			Locked:    fixedpoint.MustNewFromString(b.Locked),
		}
	}

	a := &types.Account{
		AccountType: types.AccountTypeSpot,
		CanDeposit:  account.CanDeposit,  // if can transfer in asset
		CanTrade:    account.CanTrade,    // if can trade
		CanWithdraw: account.CanWithdraw, // if can transfer out asset
	}
	a.UpdateBalances(balances)
	return a, nil
}

func (e *Exchange) QueryFuturesAccount(ctx context.Context) (*types.Account, error) {
	account, err := e.futuresClient.NewGetAccountService().Do(ctx)
	if err != nil {
		return nil, err
	}
	accountBalances, err := e.futuresClient.NewGetBalanceService().Do(ctx)
	if err != nil {
		return nil, err
	}

	var balances = map[string]types.Balance{}
	for _, b := range accountBalances {
		balances[b.Asset] = types.Balance{
			Currency:  b.Asset,
			Available: fixedpoint.Must(fixedpoint.NewFromString(b.AvailableBalance)),
		}
	}

	a := &types.Account{
		AccountType: types.AccountTypeFutures,
		FuturesInfo: toGlobalFuturesAccountInfo(account), // In binance GO api, Account define account info which mantain []*AccountAsset and []*AccountPosition.
		CanDeposit:  account.CanDeposit,                  // if can transfer in asset
		CanTrade:    account.CanTrade,                    // if can trade
		CanWithdraw: account.CanWithdraw,                 // if can transfer out asset
	}
	a.UpdateBalances(balances)
	return a, nil
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	var account *types.Account
	var err error
	if e.IsFutures {
		account, err = e.QueryFuturesAccount(ctx)
	} else if e.IsIsolatedMargin {
		account, err = e.QueryIsolatedMarginAccount(ctx)
	} else if e.IsMargin {
		account, err = e.QueryMarginAccount(ctx)
	} else {
		account, err = e.QuerySpotAccount(ctx)
	}

	return account, err
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	if e.IsMargin {
		req := e.Client.NewListMarginOpenOrdersService().Symbol(symbol)
		req.IsIsolated(e.IsIsolatedMargin)

		binanceOrders, err := req.Do(ctx)
		if err != nil {
			return orders, err
		}

		return toGlobalOrders(binanceOrders)
	}

	if e.IsFutures {
		req := e.futuresClient.NewListOpenOrdersService().Symbol(symbol)

		binanceOrders, err := req.Do(ctx)
		if err != nil {
			return orders, err
		}

		return toGlobalFuturesOrders(binanceOrders)
	}

	binanceOrders, err := e.Client.NewListOpenOrdersService().Symbol(symbol).Do(ctx)
	if err != nil {
		return orders, err
	}

	return toGlobalOrders(binanceOrders)
}

func (e *Exchange) QueryOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	orderID, err := strconv.ParseInt(q.OrderID, 10, 64)
	if err != nil {
		return nil, err
	}

	var order *binance.Order
	if e.IsMargin {
		order, err = e.Client.NewGetMarginOrderService().Symbol(q.Symbol).OrderID(orderID).Do(ctx)
	} else {
		order, err = e.Client.NewGetOrderService().Symbol(q.Symbol).OrderID(orderID).Do(ctx)
	}

	if err != nil {
		return nil, err
	}

	return toGlobalOrder(order, e.IsMargin)
}

func (e *Exchange) QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) (orders []types.Order, err error) {
	// we can only query orders within 24 hours
	// if the until-since is more than 24 hours, we should reset the until to:
	// new until = since + 24 hours - 1 millisecond
	/*
		if until.Sub(since) >= 24*time.Hour {
			until = since.Add(24*time.Hour - time.Millisecond)
		}
	*/

	if err := orderLimiter.Wait(ctx); err != nil {
		log.WithError(err).Errorf("order rate limiter wait error")
	}

	log.Infof("querying closed orders %s from %s <=> %s ...", symbol, since, until)

	if e.IsMargin {
		req := e.Client.NewListMarginOrdersService().Symbol(symbol)
		req.IsIsolated(e.IsIsolatedMargin)

		if lastOrderID > 0 {
			req.OrderID(int64(lastOrderID))
		} else {
			req.StartTime(since.UnixNano() / int64(time.Millisecond))
			if until.Sub(since) < 24*time.Hour {
				req.EndTime(until.UnixNano() / int64(time.Millisecond))
			}
		}

		binanceOrders, err := req.Do(ctx)
		if err != nil {
			return orders, err
		}

		return toGlobalOrders(binanceOrders)
	}

	if e.IsFutures {
		req := e.futuresClient.NewListOrdersService().Symbol(symbol)

		if lastOrderID > 0 {
			req.OrderID(int64(lastOrderID))
		} else {
			req.StartTime(since.UnixNano() / int64(time.Millisecond))
			if until.Sub(since) < 24*time.Hour {
				req.EndTime(until.UnixNano() / int64(time.Millisecond))
			}
		}

		binanceOrders, err := req.Do(ctx)
		if err != nil {
			return orders, err
		}
		return toGlobalFuturesOrders(binanceOrders)
	}

	// If orderId is set, it will get orders >= that orderId. Otherwise most recent orders are returned.
	// For some historical orders cummulativeQuoteQty will be < 0, meaning the data is not available at this time.
	// If startTime and/or endTime provided, orderId is not required.
	req := e.Client.NewListOrdersService().
		Symbol(symbol)

	if lastOrderID > 0 {
		req.OrderID(int64(lastOrderID))
	} else {
		req.StartTime(since.UnixNano() / int64(time.Millisecond))
		if until.Sub(since) < 24*time.Hour {
			req.EndTime(until.UnixNano() / int64(time.Millisecond))
		}
	}

	// default 500, max 1000
	req.Limit(1000)

	binanceOrders, err := req.Do(ctx)
	if err != nil {
		return orders, err
	}

	return toGlobalOrders(binanceOrders)
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) (err error) {
	if err := orderLimiter.Wait(ctx); err != nil {
		log.WithError(err).Errorf("order rate limiter wait error")
	}

	if e.IsFutures {
		for _, o := range orders {
			var req = e.futuresClient.NewCancelOrderService()

			// Mandatory
			req.Symbol(o.Symbol)

			if o.OrderID > 0 {
				req.OrderID(int64(o.OrderID))
			} else {
				err = multierr.Append(err, types.NewOrderError(
					fmt.Errorf("can not cancel %s order, order does not contain orderID or clientOrderID", o.Symbol),
					o))
				continue
			}

			_, err2 := req.Do(ctx)
			if err2 != nil {
				err = multierr.Append(err, types.NewOrderError(err2, o))
			}
		}

		return err
	}

	for _, o := range orders {
		if e.IsMargin {
			var req = e.Client.NewCancelMarginOrderService()
			req.IsIsolated(e.IsIsolatedMargin)
			req.Symbol(o.Symbol)

			if o.OrderID > 0 {
				req.OrderID(int64(o.OrderID))
			} else if len(o.ClientOrderID) > 0 {
				req.OrigClientOrderID(o.ClientOrderID)
			} else {
				err = multierr.Append(err, types.NewOrderError(
					fmt.Errorf("can not cancel %s order, order does not contain orderID or clientOrderID", o.Symbol),
					o))
				continue
			}

			_, err2 := req.Do(ctx)
			if err2 != nil {
				err = multierr.Append(err, types.NewOrderError(err2, o))
			}
		} else {
			// SPOT
			var req = e.Client.NewCancelOrderService()
			req.Symbol(o.Symbol)

			if o.OrderID > 0 {
				req.OrderID(int64(o.OrderID))
			} else if len(o.ClientOrderID) > 0 {
				req.OrigClientOrderID(o.ClientOrderID)
			} else {
				err = multierr.Append(err, types.NewOrderError(
					fmt.Errorf("can not cancel %s order, order does not contain orderID or clientOrderID", o.Symbol),
					o))
				continue
			}

			_, err2 := req.Do(ctx)
			if err2 != nil {
				err = multierr.Append(err, types.NewOrderError(err2, o))
			}
		}
	}

	return err
}

func (e *Exchange) submitMarginOrder(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
	orderType, err := toLocalOrderType(order.Type)
	if err != nil {
		return nil, err
	}

	req := e.Client.NewCreateMarginOrderService().
		Symbol(order.Symbol).
		Type(orderType).
		Side(binance.SideType(order.Side))

	clientOrderID := newSpotClientOrderID(order.ClientOrderID)
	if len(clientOrderID) > 0 {
		req.NewClientOrderID(clientOrderID)
	}

	// use response result format
	req.NewOrderRespType(binance.NewOrderRespTypeRESULT)

	if e.IsIsolatedMargin {
		req.IsIsolated(e.IsIsolatedMargin)
	}

	if len(order.MarginSideEffect) > 0 {
		req.SideEffectType(binance.SideEffectType(order.MarginSideEffect))
	}

	if order.Market.Symbol != "" {
		req.Quantity(order.Market.FormatQuantity(order.Quantity))
	} else {
		// TODO report error
		req.Quantity(order.Quantity.FormatString(8))
	}

	// set price field for limit orders
	switch order.Type {
	case types.OrderTypeStopLimit, types.OrderTypeLimit, types.OrderTypeLimitMaker:
		if order.Market.Symbol != "" {
			req.Price(order.Market.FormatPrice(order.Price))
		} else {
			// TODO report error
			req.Price(order.Price.FormatString(8))
		}
	}

	// set stop price
	switch order.Type {

	case types.OrderTypeStopLimit, types.OrderTypeStopMarket:
		if order.Market.Symbol != "" {
			req.StopPrice(order.Market.FormatPrice(order.StopPrice))
		} else {
			// TODO report error
			req.StopPrice(order.StopPrice.FormatString(8))
		}
	}

	// could be IOC or FOK
	if len(order.TimeInForce) > 0 {
		// TODO: check the TimeInForce value
		req.TimeInForce(binance.TimeInForceType(order.TimeInForce))
	} else {
		switch order.Type {
		case types.OrderTypeLimit, types.OrderTypeStopLimit:
			req.TimeInForce(binance.TimeInForceTypeGTC)
		}
	}

	response, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	log.Infof("margin order creation response: %+v", response)

	createdOrder, err := toGlobalOrder(&binance.Order{
		Symbol:                   response.Symbol,
		OrderID:                  response.OrderID,
		ClientOrderID:            response.ClientOrderID,
		Price:                    response.Price,
		OrigQuantity:             response.OrigQuantity,
		ExecutedQuantity:         response.ExecutedQuantity,
		CummulativeQuoteQuantity: response.CummulativeQuoteQuantity,
		Status:                   response.Status,
		TimeInForce:              response.TimeInForce,
		Type:                     response.Type,
		Side:                     response.Side,
		UpdateTime:               response.TransactTime,
		Time:                     response.TransactTime,
		IsIsolated:               response.IsIsolated,
	}, true)

	return createdOrder, err
}

func (e *Exchange) submitFuturesOrder(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
	orderType, err := toLocalFuturesOrderType(order.Type)
	if err != nil {
		return nil, err
	}

	req := e.futuresClient.NewCreateOrderService().
		Symbol(order.Symbol).
		Type(orderType).
		Side(futures.SideType(order.Side)).
		ReduceOnly(order.ReduceOnly)

	clientOrderID := newFuturesClientOrderID(order.ClientOrderID)
	if len(clientOrderID) > 0 {
		req.NewClientOrderID(clientOrderID)
	}

	// use response result format
	req.NewOrderResponseType(futures.NewOrderRespTypeRESULT)

	if order.Market.Symbol != "" {
		req.Quantity(order.Market.FormatQuantity(order.Quantity))
	} else {
		// TODO report error
		req.Quantity(order.Quantity.FormatString(8))
	}

	// set price field for limit orders
	switch order.Type {
	case types.OrderTypeStopLimit, types.OrderTypeLimit, types.OrderTypeLimitMaker:
		if order.Market.Symbol != "" {
			req.Price(order.Market.FormatPrice(order.Price))
		} else {
			// TODO report error
			req.Price(order.Price.FormatString(8))
		}
	}

	// set stop price
	switch order.Type {

	case types.OrderTypeStopLimit, types.OrderTypeStopMarket:
		if order.Market.Symbol != "" {
			req.StopPrice(order.Market.FormatPrice(order.StopPrice))
		} else {
			// TODO report error
			req.StopPrice(order.StopPrice.FormatString(8))
		}
	}

	// could be IOC or FOK
	if len(order.TimeInForce) > 0 {
		// TODO: check the TimeInForce value
		req.TimeInForce(futures.TimeInForceType(order.TimeInForce))
	} else {
		switch order.Type {
		case types.OrderTypeLimit, types.OrderTypeStopLimit:
			req.TimeInForce(futures.TimeInForceTypeGTC)
		}
	}

	response, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	log.Infof("futures order creation response: %+v", response)

	createdOrder, err := toGlobalFuturesOrder(&futures.Order{
		Symbol:           response.Symbol,
		OrderID:          response.OrderID,
		ClientOrderID:    response.ClientOrderID,
		Price:            response.Price,
		OrigQuantity:     response.OrigQuantity,
		ExecutedQuantity: response.ExecutedQuantity,
		Status:           response.Status,
		TimeInForce:      response.TimeInForce,
		Type:             response.Type,
		Side:             response.Side,
		ReduceOnly:       response.ReduceOnly,
	}, true)

	return createdOrder, err
}

// BBGO is a broker on Binance
const spotBrokerID = "NSUYEBKM"

func newSpotClientOrderID(originalID string) (clientOrderID string) {
	if originalID == types.NoClientOrderID {
		return ""
	}

	prefix := "x-" + spotBrokerID
	prefixLen := len(prefix)

	if originalID != "" {
		// try to keep the whole original client order ID if user specifies it.
		if prefixLen+len(originalID) > 32 {
			return originalID
		}

		clientOrderID = prefix + originalID
		return clientOrderID
	}

	clientOrderID = uuid.New().String()
	clientOrderID = prefix + clientOrderID
	if len(clientOrderID) > 32 {
		return clientOrderID[0:32]
	}

	return clientOrderID
}

// BBGO is a futures broker on Binance
const futuresBrokerID = "gBhMvywy"

func newFuturesClientOrderID(originalID string) (clientOrderID string) {
	if originalID == types.NoClientOrderID {
		return ""
	}

	prefix := "x-" + futuresBrokerID
	prefixLen := len(prefix)

	if originalID != "" {
		// try to keep the whole original client order ID if user specifies it.
		if prefixLen+len(originalID) > 32 {
			return originalID
		}

		clientOrderID = prefix + originalID
		return clientOrderID
	}

	clientOrderID = uuid.New().String()
	clientOrderID = prefix + clientOrderID
	if len(clientOrderID) > 32 {
		return clientOrderID[0:32]
	}

	return clientOrderID
}

func (e *Exchange) submitSpotOrder(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
	orderType, err := toLocalOrderType(order.Type)
	if err != nil {
		return nil, err
	}

	req := e.Client.NewCreateOrderService().
		Symbol(order.Symbol).
		Side(binance.SideType(order.Side)).
		Type(orderType)

	clientOrderID := newSpotClientOrderID(order.ClientOrderID)
	if len(clientOrderID) > 0 {
		req.NewClientOrderID(clientOrderID)
	}

	if order.Market.Symbol != "" {
		req.Quantity(order.Market.FormatQuantity(order.Quantity))
	} else {
		// TODO: report error
		req.Quantity(order.Quantity.FormatString(8))
	}

	// set price field for limit orders
	switch order.Type {
	case types.OrderTypeStopLimit, types.OrderTypeLimit, types.OrderTypeLimitMaker:
		if order.Market.Symbol != "" {
			req.Price(order.Market.FormatPrice(order.Price))
		} else {
			// TODO: report error
			req.Price(order.Price.FormatString(8))
		}
	}

	switch order.Type {
	case types.OrderTypeStopLimit, types.OrderTypeStopMarket:
		if order.Market.Symbol != "" {
			req.StopPrice(order.Market.FormatPrice(order.StopPrice))
		} else {
			// TODO: report error
			req.StopPrice(order.StopPrice.FormatString(8))
		}
	}

	if len(order.TimeInForce) > 0 {
		// TODO: check the TimeInForce value
		req.TimeInForce(binance.TimeInForceType(order.TimeInForce))
	} else {
		switch order.Type {
		case types.OrderTypeLimit, types.OrderTypeStopLimit:
			req.TimeInForce(binance.TimeInForceTypeGTC)
		}
	}

	req.NewOrderRespType(binance.NewOrderRespTypeRESULT)

	response, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	log.Infof("spot order creation response: %+v", response)

	createdOrder, err := toGlobalOrder(&binance.Order{
		Symbol:                   response.Symbol,
		OrderID:                  response.OrderID,
		ClientOrderID:            response.ClientOrderID,
		Price:                    response.Price,
		OrigQuantity:             response.OrigQuantity,
		ExecutedQuantity:         response.ExecutedQuantity,
		CummulativeQuoteQuantity: response.CummulativeQuoteQuantity,
		Status:                   response.Status,
		TimeInForce:              response.TimeInForce,
		Type:                     response.Type,
		Side:                     response.Side,
		UpdateTime:               response.TransactTime,
		Time:                     response.TransactTime,
		IsIsolated:               response.IsIsolated,
	}, false)

	return createdOrder, err
}

func (e *Exchange) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error) {
	for _, order := range orders {
		if err := orderLimiter.Wait(ctx); err != nil {
			log.WithError(err).Errorf("order rate limiter wait error")
		}

		var createdOrder *types.Order
		if e.IsMargin {
			createdOrder, err = e.submitMarginOrder(ctx, order)
		} else if e.IsFutures {
			createdOrder, err = e.submitFuturesOrder(ctx, order)
		} else {
			createdOrder, err = e.submitSpotOrder(ctx, order)
		}

		if err != nil {
			return createdOrders, err
		}

		if createdOrder == nil {
			return createdOrders, errors.New("nil converted order")
		}

		createdOrders = append(createdOrders, *createdOrder)
	}

	return createdOrders, err
}

// QueryKLines queries the Kline/candlestick bars for a symbol. Klines are uniquely identified by their open time.
// Binance uses inclusive start time query range, eg:
// https://api.binance.com/api/v3/klines?symbol=BTCUSDT&interval=1m&startTime=1620172860000
// the above query will return a kline with startTime = 1620172860000
// and,
// https://api.binance.com/api/v3/klines?symbol=BTCUSDT&interval=1m&startTime=1620172860000&endTime=1620172920000
// the above query will return a kline with startTime = 1620172860000, and a kline with endTime = 1620172860000
//
// the endTime of a binance kline, is the (startTime + interval time - 1 millisecond), e.g.,
// millisecond unix timestamp: 1620172860000 and 1620172919999
func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {

	var limit = 1000
	if options.Limit > 0 {
		// default limit == 1000
		limit = options.Limit
	}

	log.Infof("querying kline %s %s %v", symbol, interval, options)

	req := e.Client.NewKlinesService().
		Symbol(symbol).
		Interval(string(interval)).
		Limit(limit)

	if options.StartTime != nil {
		req.StartTime(options.StartTime.UnixNano() / int64(time.Millisecond))
	}

	if options.EndTime != nil {
		req.EndTime(options.EndTime.UnixNano() / int64(time.Millisecond))
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var kLines []types.KLine
	for _, k := range resp {
		kLines = append(kLines, types.KLine{
			Exchange:                 types.ExchangeBinance,
			Symbol:                   symbol,
			Interval:                 interval,
			StartTime:                types.NewTimeFromUnix(0, k.OpenTime*int64(time.Millisecond)),
			EndTime:                  types.NewTimeFromUnix(0, k.CloseTime*int64(time.Millisecond)),
			Open:                     fixedpoint.MustNewFromString(k.Open),
			Close:                    fixedpoint.MustNewFromString(k.Close),
			High:                     fixedpoint.MustNewFromString(k.High),
			Low:                      fixedpoint.MustNewFromString(k.Low),
			Volume:                   fixedpoint.MustNewFromString(k.Volume),
			QuoteVolume:              fixedpoint.MustNewFromString(k.QuoteAssetVolume),
			TakerBuyBaseAssetVolume:  fixedpoint.MustNewFromString(k.TakerBuyBaseAssetVolume),
			TakerBuyQuoteAssetVolume: fixedpoint.MustNewFromString(k.TakerBuyQuoteAssetVolume),
			LastTradeID:              0,
			NumberOfTrades:           uint64(k.TradeNum),
			Closed:                   true,
		})
	}
	return kLines, nil
}

func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) (trades []types.Trade, err error) {

	if e.IsMargin {
		var remoteTrades []*binance.TradeV3
		req := e.Client.NewListMarginTradesService().
			IsIsolated(e.IsIsolatedMargin).
			Symbol(symbol)

		if options.Limit > 0 {
			req.Limit(int(options.Limit))
		} else {
			req.Limit(1000)
		}

		if options.StartTime != nil {
			req.StartTime(options.StartTime.UnixNano() / int64(time.Millisecond))
		}

		if options.EndTime != nil {
			req.EndTime(options.EndTime.UnixNano() / int64(time.Millisecond))
		}

		// BINANCE uses inclusive last trade ID
		if options.LastTradeID > 0 {
			req.FromID(int64(options.LastTradeID))
		}

		remoteTrades, err = req.Do(ctx)
		if err != nil {
			return nil, err
		}
		for _, t := range remoteTrades {
			localTrade, err := toGlobalTrade(*t, e.IsMargin)
			if err != nil {
				log.WithError(err).Errorf("can not convert binance trade: %+v", t)
				continue
			}

			trades = append(trades, *localTrade)
		}

		return trades, nil
	} else if e.IsFutures {
		var remoteTrades []*futures.AccountTrade
		req := e.futuresClient.NewListAccountTradeService().
			Symbol(symbol)
		if options.Limit > 0 {
			req.Limit(int(options.Limit))
		} else {
			req.Limit(1000)
		}

		// BINANCE uses inclusive last trade ID
		if options.LastTradeID > 0 {
			req.FromID(int64(options.LastTradeID))
		}

		remoteTrades, err = req.Do(ctx)
		if err != nil {
			return nil, err
		}
		for _, t := range remoteTrades {
			localTrade, err := toGlobalFuturesTrade(*t)
			if err != nil {
				log.WithError(err).Errorf("can not convert binance futures trade: %+v", t)
				continue
			}

			trades = append(trades, *localTrade)
		}

		return trades, nil
	} else {
		var remoteTrades []*binance.TradeV3
		req := e.Client.NewListTradesService().
			Symbol(symbol)

		if options.Limit > 0 {
			req.Limit(int(options.Limit))
		} else {
			req.Limit(1000)
		}

		if options.StartTime != nil {
			req.StartTime(options.StartTime.UnixNano() / int64(time.Millisecond))
		}
		if options.EndTime != nil {
			req.EndTime(options.EndTime.UnixNano() / int64(time.Millisecond))
		}

		// BINANCE uses inclusive last trade ID
		if options.LastTradeID > 0 {
			req.FromID(int64(options.LastTradeID))
		}

		remoteTrades, err = req.Do(ctx)
		if err != nil {
			return nil, err
		}
		for _, t := range remoteTrades {
			localTrade, err := toGlobalTrade(*t, e.IsMargin)
			if err != nil {
				log.WithError(err).Errorf("can not convert binance trade: %+v", t)
				continue
			}

			trades = append(trades, *localTrade)
		}

		return trades, nil
	}
}

func (e *Exchange) QueryDepth(ctx context.Context, symbol string) (snapshot types.SliceOrderBook, finalUpdateID int64, err error) {
	response, err := e.Client.NewDepthService().Symbol(symbol).Do(ctx)
	if err != nil {
		return snapshot, finalUpdateID, err
	}

	snapshot.Symbol = symbol
	finalUpdateID = response.LastUpdateID
	for _, entry := range response.Bids {
		// entry.Price, Quantity: entry.Quantity
		price, err := fixedpoint.NewFromString(entry.Price)
		if err != nil {
			return snapshot, finalUpdateID, err
		}

		quantity, err := fixedpoint.NewFromString(entry.Quantity)
		if err != nil {
			return snapshot, finalUpdateID, err
		}

		snapshot.Bids = append(snapshot.Bids, types.PriceVolume{Price: price, Volume: quantity})
	}

	for _, entry := range response.Asks {
		price, err := fixedpoint.NewFromString(entry.Price)
		if err != nil {
			return snapshot, finalUpdateID, err
		}

		quantity, err := fixedpoint.NewFromString(entry.Quantity)
		if err != nil {
			return snapshot, finalUpdateID, err
		}

		snapshot.Asks = append(snapshot.Asks, types.PriceVolume{Price: price, Volume: quantity})
	}

	return snapshot, finalUpdateID, nil
}

func (e *Exchange) BatchQueryKLines(ctx context.Context, symbol string, interval types.Interval, startTime, endTime time.Time) ([]types.KLine, error) {
	var allKLines []types.KLine

	for startTime.Before(endTime) {
		klines, err := e.QueryKLines(ctx, symbol, interval, types.KLineQueryOptions{
			StartTime: &startTime,
			Limit:     1000,
		})

		if err != nil {
			return nil, err
		}

		for _, kline := range klines {
			if kline.EndTime.After(endTime) {
				return allKLines, nil
			}

			allKLines = append(allKLines, kline)
			startTime = kline.EndTime.Time()
		}
	}

	return allKLines, nil
}

func (e *Exchange) QueryPremiumIndex(ctx context.Context, symbol string) (*types.PremiumIndex, error) {
	futuresClient := binance.NewFuturesClient(e.key, e.secret)

	// when symbol is set, only one index will be returned.
	indexes, err := futuresClient.NewPremiumIndexService().Symbol(symbol).Do(ctx)
	if err != nil {
		return nil, err
	}

	return convertPremiumIndex(indexes[0])
}

func (e *Exchange) QueryFundingRateHistory(ctx context.Context, symbol string) (*types.FundingRate, error) {
	futuresClient := binance.NewFuturesClient(e.key, e.secret)
	rates, err := futuresClient.NewFundingRateService().
		Symbol(symbol).
		Limit(1).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	if len(rates) == 0 {
		return nil, errors.New("empty funding rate data")
	}

	rate := rates[0]
	fundingRate, err := fixedpoint.NewFromString(rate.FundingRate)
	if err != nil {
		return nil, err
	}

	return &types.FundingRate{
		FundingRate: fundingRate,
		FundingTime: time.Unix(0, rate.FundingTime*int64(time.Millisecond)),
		Time:        time.Unix(0, rate.Time*int64(time.Millisecond)),
	}, nil
}

func (e *Exchange) QueryPositionRisk(ctx context.Context, symbol string) (*types.PositionRisk, error) {
	futuresClient := binance.NewFuturesClient(e.key, e.secret)

	// when symbol is set, only one position risk will be returned.
	risks, err := futuresClient.NewGetPositionRiskService().Symbol(symbol).Do(ctx)
	if err != nil {
		return nil, err
	}

	return convertPositionRisk(risks[0])
}

func getLaunchDate() (time.Time, error) {
	// binance launch date 12:00 July 14th, 2017
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Time{}, err
	}

	return time.Date(2017, time.July, 14, 0, 0, 0, 0, loc), nil
}
