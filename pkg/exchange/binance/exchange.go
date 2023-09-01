package binance

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/spf13/viper"

	"go.uber.org/multierr"

	"golang.org/x/time/rate"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
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

// orderLimiter - the default order limiter apply 5 requests per second and a 2 initial bucket
// this includes SubmitOrder, CancelOrder and QueryClosedOrders
//
// Limit defines the maximum frequency of some events. Limit is represented as number of events per second. A zero Limit allows no events.
var orderLimiter = rate.NewLimiter(5, 2)
var queryTradeLimiter = rate.NewLimiter(1, 2)

var log = logrus.WithFields(logrus.Fields{
	"exchange": "binance",
})

func init() {
	_ = types.Exchange(&Exchange{})
	_ = types.MarginExchange(&Exchange{})
	_ = types.FuturesExchange(&Exchange{})

	if n, ok := util.GetEnvVarInt("BINANCE_ORDER_RATE_LIMITER"); ok {
		orderLimiter = rate.NewLimiter(rate.Every(time.Duration(n)*time.Minute), 2)
	}

	if n, ok := util.GetEnvVarInt("BINANCE_QUERY_TRADES_RATE_LIMITER"); ok {
		queryTradeLimiter = rate.NewLimiter(rate.Every(time.Duration(n)*time.Minute), 2)
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

	key, secret string
	// client is used for spot & margin
	client *binance.Client

	// futuresClient is used for usdt-m futures
	futuresClient *futures.Client // USDT-M Futures
	// deliveryClient	*delivery.Client // Coin-M Futures

	// client2 is a newer version of the binance api client implemented by ourselves.
	client2 *binanceapi.RestClient

	futuresClient2 *binanceapi.FuturesRestClient
}

var timeSetterOnce sync.Once

func New(key, secret string) *Exchange {
	var client = binance.NewClient(key, secret)
	client.HTTPClient = binanceapi.DefaultHttpClient
	client.Debug = viper.GetBool("debug-binance-client")

	var futuresClient = binance.NewFuturesClient(key, secret)
	futuresClient.HTTPClient = binanceapi.DefaultHttpClient
	futuresClient.Debug = viper.GetBool("debug-binance-futures-client")

	if isBinanceUs() {
		client.BaseURL = BinanceUSBaseURL
	}

	if paperTrade() {
		client.BaseURL = BinanceTestBaseURL
		futuresClient.BaseURL = FutureTestBaseURL
	}

	client2 := binanceapi.NewClient(client.BaseURL)
	futuresClient2 := binanceapi.NewFuturesRestClient(futuresClient.BaseURL)

	ex := &Exchange{
		key:            key,
		secret:         secret,
		client:         client,
		futuresClient:  futuresClient,
		client2:        client2,
		futuresClient2: futuresClient2,
	}

	if len(key) > 0 && len(secret) > 0 {
		client2.Auth(key, secret)
		futuresClient2.Auth(key, secret)

		ctx := context.Background()
		go timeSetterOnce.Do(func() {
			ex.setServerTimeOffset(ctx)

			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return

				case <-ticker.C:
					ex.setServerTimeOffset(ctx)
				}
			}
		})
	}

	return ex
}

func (e *Exchange) setServerTimeOffset(ctx context.Context) {
	_, err := e.client.NewSetServerTimeService().Do(ctx)
	if err != nil {
		log.WithError(err).Error("can not set server time")
	}

	_, err = e.futuresClient.NewSetServerTimeService().Do(ctx)
	if err != nil {
		log.WithError(err).Error("can not set server time")
	}

	if err = e.client2.SetTimeOffsetFromServer(ctx); err != nil {
		log.WithError(err).Error("can not set server time")
	}
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeBinance
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	if e.IsFutures {
		req := e.futuresClient.NewListPriceChangeStatsService()
		req.Symbol(strings.ToUpper(symbol))
		stats, err := req.Do(ctx)
		if err != nil {
			return nil, err
		}

		return toGlobalFuturesTicker(stats[0])
	}
	req := e.client.NewListPriceChangeStatsService()
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

	m := make(map[string]struct{})
	exists := struct{}{}

	for _, s := range symbol {
		m[s] = exists
	}

	if e.IsFutures {
		var req = e.futuresClient.NewListPriceChangeStatsService()
		changeStats, err := req.Do(ctx)
		if err != nil {
			return nil, err
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
				Buy:    fixedpoint.MustNewFromString(stats.LastPrice),
				Sell:   fixedpoint.MustNewFromString(stats.LastPrice),
				Time:   time.Unix(0, stats.CloseTime*int64(time.Millisecond)),
			}

			tickers[stats.Symbol] = tick
		}

		return tickers, nil
	}

	var req = e.client.NewListPriceChangeStatsService()
	changeStats, err := req.Do(ctx)
	if err != nil {
		return nil, err
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

	exchangeInfo, err := e.client.NewExchangeInfoService().Do(ctx)
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
	resp, err := e.client.NewAveragePriceService().Symbol(symbol).Do(ctx)
	if err != nil {
		return fixedpoint.Zero, err
	}

	return fixedpoint.MustNewFromString(resp.Price), nil
}

func (e *Exchange) NewStream() types.Stream {
	stream := NewStream(e, e.client, e.futuresClient)
	stream.MarginSettings = e.MarginSettings
	stream.FuturesSettings = e.FuturesSettings
	return stream
}

func (e *Exchange) QueryMarginAssetMaxBorrowable(ctx context.Context, asset string) (amount fixedpoint.Value, err error) {
	req := e.client2.NewGetMarginMaxBorrowableRequest()
	req.Asset(asset)
	if e.IsIsolatedMargin {
		req.IsolatedSymbol(e.IsolatedMarginSymbol)
	}

	resp, err := req.Do(ctx)
	if err != nil {
		return fixedpoint.Zero, err
	}

	return resp.Amount, nil
}

func (e *Exchange) RepayMarginAsset(ctx context.Context, asset string, amount fixedpoint.Value) error {
	req := e.client.NewMarginRepayService()
	req.Asset(asset)
	req.Amount(amount.String())
	if e.IsIsolatedMargin {
		req.Symbol(e.IsolatedMarginSymbol)
	}

	log.Infof("repaying margin asset %s amount %f", asset, amount.Float64())
	resp, err := req.Do(ctx)
	if err != nil {
		return err
	}

	log.Debugf("margin repayed %f %s, transaction id = %d", amount.Float64(), asset, resp.TranID)
	return err
}

func (e *Exchange) BorrowMarginAsset(ctx context.Context, asset string, amount fixedpoint.Value) error {
	req := e.client.NewMarginLoanService()
	req.Asset(asset)
	req.Amount(amount.String())
	if e.IsIsolatedMargin {
		req.Symbol(e.IsolatedMarginSymbol)
	}

	log.Infof("borrowing margin asset %s amount %f", asset, amount.Float64())
	resp, err := req.Do(ctx)
	if err != nil {
		return err
	}
	log.Debugf("margin borrowed %f %s, transaction id = %d", amount.Float64(), asset, resp.TranID)
	return err
}

func (e *Exchange) QueryMarginBorrowHistory(ctx context.Context, asset string) error {
	req := e.client.NewListMarginLoansService()
	req.Asset(asset)
	history, err := req.Do(ctx)
	if err != nil {
		return err
	}
	_ = history
	return nil
}

// TransferMarginAccountAsset transfers the asset into/out from the margin account
//
// types.TransferIn => Spot to Margin
// types.TransferOut => Margin to Spot
//
// to call this method, you must set the IsMargin = true
func (e *Exchange) TransferMarginAccountAsset(ctx context.Context, asset string, amount fixedpoint.Value, io types.TransferDirection) error {
	if e.IsIsolatedMargin {
		return e.transferIsolatedMarginAccountAsset(ctx, asset, amount, io)
	}

	return e.transferCrossMarginAccountAsset(ctx, asset, amount, io)
}

func (e *Exchange) transferIsolatedMarginAccountAsset(ctx context.Context, asset string, amount fixedpoint.Value, io types.TransferDirection) error {
	req := e.client2.NewTransferIsolatedMarginAccountRequest()
	req.Symbol(e.IsolatedMarginSymbol)

	switch io {
	case types.TransferIn:
		req.TransFrom(binanceapi.AccountTypeSpot)
		req.TransTo(binanceapi.AccountTypeIsolatedMargin)

	case types.TransferOut:
		req.TransFrom(binanceapi.AccountTypeIsolatedMargin)
		req.TransTo(binanceapi.AccountTypeSpot)
	}

	req.Asset(asset)
	req.Amount(amount.String())
	resp, err := req.Do(ctx)
	return logResponse(resp, err, req)
}

// transferCrossMarginAccountAsset transfer asset to the cross margin account or to the main account
func (e *Exchange) transferCrossMarginAccountAsset(ctx context.Context, asset string, amount fixedpoint.Value, io types.TransferDirection) error {
	req := e.client2.NewTransferCrossMarginAccountRequest()
	req.Asset(asset)
	req.Amount(amount.String())

	if io == types.TransferIn {
		req.TransferType(int(binance.MarginTransferTypeToMargin))
	} else if io == types.TransferOut {
		req.TransferType(int(binance.MarginTransferTypeToMain))
	} else {
		return fmt.Errorf("unexpected transfer direction: %d given", io)
	}

	resp, err := req.Do(ctx)
	return logResponse(resp, err, req)
}

func (e *Exchange) QueryCrossMarginAccount(ctx context.Context) (*types.Account, error) {
	marginAccount, err := e.client.NewGetMarginAccountService().Do(ctx)
	if err != nil {
		return nil, err
	}

	marginLevel := fixedpoint.MustNewFromString(marginAccount.MarginLevel)
	a := &types.Account{
		AccountType:     types.AccountTypeMargin,
		MarginInfo:      toGlobalMarginAccountInfo(marginAccount), // In binance GO api, Account define marginAccount info which mantain []*AccountAsset and []*AccountPosition.
		MarginLevel:     marginLevel,
		MarginTolerance: calculateMarginTolerance(marginLevel),
		BorrowEnabled:   marginAccount.BorrowEnabled,
		TransferEnabled: marginAccount.TransferEnabled,
	}

	// convert cross margin user assets into balances
	balances := types.BalanceMap{}
	for _, userAsset := range marginAccount.UserAssets {
		balances[userAsset.Asset] = types.Balance{
			Currency:  userAsset.Asset,
			Available: fixedpoint.MustNewFromString(userAsset.Free),
			Locked:    fixedpoint.MustNewFromString(userAsset.Locked),
			Interest:  fixedpoint.MustNewFromString(userAsset.Interest),
			Borrowed:  fixedpoint.MustNewFromString(userAsset.Borrowed),
			NetAsset:  fixedpoint.MustNewFromString(userAsset.NetAsset),
		}
	}
	a.UpdateBalances(balances)
	return a, nil
}

func (e *Exchange) QueryIsolatedMarginAccount(ctx context.Context) (*types.Account, error) {
	req := e.client.NewGetIsolatedMarginAccountService()
	req.Symbols(e.IsolatedMarginSymbol)

	marginAccount, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	a := &types.Account{
		AccountType:        types.AccountTypeIsolatedMargin,
		IsolatedMarginInfo: toGlobalIsolatedMarginAccountInfo(marginAccount), // In binance GO api, Account define marginAccount info which mantain []*AccountAsset and []*AccountPosition.
	}

	if len(marginAccount.Assets) == 0 {
		return nil, fmt.Errorf("empty margin account assets, please check your isolatedMarginSymbol is correctly set: %+v", marginAccount)
	}

	// for isolated margin account, we will only have one asset in the Assets array.
	if len(marginAccount.Assets) > 1 {
		return nil, fmt.Errorf("unexpected number of user assets returned, got %d user assets", len(marginAccount.Assets))
	}

	userAsset := marginAccount.Assets[0]
	marginLevel := fixedpoint.MustNewFromString(userAsset.MarginLevel)
	a.MarginLevel = marginLevel
	a.MarginTolerance = calculateMarginTolerance(marginLevel)
	a.MarginRatio = fixedpoint.MustNewFromString(userAsset.MarginRatio)
	a.BorrowEnabled = userAsset.BaseAsset.BorrowEnabled || userAsset.QuoteAsset.BorrowEnabled
	a.LiquidationPrice = fixedpoint.MustNewFromString(userAsset.LiquidatePrice)
	a.LiquidationRate = fixedpoint.MustNewFromString(userAsset.LiquidateRate)

	// Convert user assets into balances
	balances := types.BalanceMap{}
	balances[userAsset.BaseAsset.Asset] = types.Balance{
		Currency:  userAsset.BaseAsset.Asset,
		Available: fixedpoint.MustNewFromString(userAsset.BaseAsset.Free),
		Locked:    fixedpoint.MustNewFromString(userAsset.BaseAsset.Locked),
		Interest:  fixedpoint.MustNewFromString(userAsset.BaseAsset.Interest),
		Borrowed:  fixedpoint.MustNewFromString(userAsset.BaseAsset.Borrowed),
		NetAsset:  fixedpoint.MustNewFromString(userAsset.BaseAsset.NetAsset),
	}

	balances[userAsset.QuoteAsset.Asset] = types.Balance{
		Currency:  userAsset.QuoteAsset.Asset,
		Available: fixedpoint.MustNewFromString(userAsset.QuoteAsset.Free),
		Locked:    fixedpoint.MustNewFromString(userAsset.QuoteAsset.Locked),
		Interest:  fixedpoint.MustNewFromString(userAsset.QuoteAsset.Interest),
		Borrowed:  fixedpoint.MustNewFromString(userAsset.QuoteAsset.Borrowed),
		NetAsset:  fixedpoint.MustNewFromString(userAsset.QuoteAsset.NetAsset),
	}

	a.UpdateBalances(balances)
	return a, nil
}

func (e *Exchange) Withdraw(ctx context.Context, asset string, amount fixedpoint.Value, address string, options *types.WithdrawalOptions) error {
	req := e.client2.NewWithdrawRequest()
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

func (e *Exchange) QueryWithdrawHistory(ctx context.Context, asset string, since, until time.Time) (withdraws []types.Withdraw, err error) {
	var emptyTime = time.Time{}
	if since == emptyTime {
		since, err = getLaunchDate()
		if err != nil {
			return withdraws, err
		}
	}

	// startTime ~ endTime must be in 90 days
	historyDayRangeLimit := time.Hour * 24 * 89
	if until.Sub(since) >= historyDayRangeLimit {
		until = since.Add(historyDayRangeLimit)
	}

	req := e.client2.NewGetWithdrawHistoryRequest()
	if len(asset) > 0 {
		req.Coin(asset)
	}

	records, err := req.
		StartTime(since).
		EndTime(until).
		Limit(1000).
		Do(ctx)

	if err != nil {
		return withdraws, err
	}

	for _, d := range records {
		// time format: 2006-01-02 15:04:05
		applyTime, err := time.Parse("2006-01-02 15:04:05", d.ApplyTime)
		if err != nil {
			return nil, err
		}

		withdraws = append(withdraws, types.Withdraw{
			Exchange:        types.ExchangeBinance,
			ApplyTime:       types.Time(applyTime),
			Asset:           d.Coin,
			Amount:          d.Amount,
			Address:         d.Address,
			TransactionID:   d.TxID,
			TransactionFee:  d.TransactionFee,
			WithdrawOrderID: d.WithdrawOrderID,
			Network:         d.Network,
			Status:          d.Status.String(),
		})
	}

	return withdraws, nil
}

func (e *Exchange) QueryDepositHistory(ctx context.Context, asset string, since, until time.Time) (allDeposits []types.Deposit, err error) {
	if since.IsZero() {
		since, err = getLaunchDate()
		if err != nil {
			return nil, err
		}
	}

	// startTime ~ endTime must be in 90 days
	historyDayRangeLimit := time.Hour * 24 * 89
	if until.Sub(since) >= historyDayRangeLimit {
		until = since.Add(historyDayRangeLimit)
	}

	req := e.client2.NewGetDepositHistoryRequest()
	if len(asset) > 0 {
		req.Coin(asset)
	}

	req.StartTime(since).
		EndTime(until)

	records, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	for _, d := range records {
		// 0(0:pending,6: credited but cannot withdraw, 1:success)
		// set the default status
		status := types.DepositStatus(fmt.Sprintf("code: %d", d.Status))

		// https://www.binance.com/en/support/faq/115003736451
		switch d.Status {
		case binanceapi.DepositStatusPending:
			status = types.DepositPending

		case binanceapi.DepositStatusCredited:
			status = types.DepositCredited

		case binanceapi.DepositStatusSuccess:
			status = types.DepositSuccess
		}

		allDeposits = append(allDeposits, types.Deposit{
			Exchange:      types.ExchangeBinance,
			Time:          types.Time(d.InsertTime.Time()),
			Asset:         d.Coin,
			Amount:        d.Amount,
			Address:       d.Address,
			AddressTag:    d.AddressTag,
			TransactionID: d.TxId,
			Status:        status,
			UnlockConfirm: d.UnlockConfirm,
			Confirmation:  d.ConfirmTimes,
		})
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
	account, err := e.client.NewGetAccountService().Do(ctx)
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

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	var account *types.Account
	var err error
	if e.IsFutures {
		account, err = e.QueryFuturesAccount(ctx)
	} else if e.IsIsolatedMargin {
		account, err = e.QueryIsolatedMarginAccount(ctx)
	} else if e.IsMargin {
		account, err = e.QueryCrossMarginAccount(ctx)
	} else {
		account, err = e.QuerySpotAccount(ctx)
	}

	return account, err
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	if e.IsMargin {
		req := e.client.NewListMarginOpenOrdersService().Symbol(symbol)
		req.IsIsolated(e.IsIsolatedMargin)

		binanceOrders, err := req.Do(ctx)
		if err != nil {
			return orders, err
		}

		return toGlobalOrders(binanceOrders, false)
	}

	if e.IsFutures {
		req := e.futuresClient.NewListOpenOrdersService().Symbol(symbol)

		binanceOrders, err := req.Do(ctx)
		if err != nil {
			return orders, err
		}

		return toGlobalFuturesOrders(binanceOrders, false)
	}

	binanceOrders, err := e.client.NewListOpenOrdersService().Symbol(symbol).Do(ctx)
	if err != nil {
		return orders, err
	}

	return toGlobalOrders(binanceOrders, false)
}

func (e *Exchange) QueryOrderTrades(ctx context.Context, q types.OrderQuery) ([]types.Trade, error) {
	orderID, err := strconv.ParseInt(q.OrderID, 10, 64)
	if err != nil {
		return nil, err
	}

	if len(q.Symbol) == 0 {
		return nil, errors.New("binance: symbol parameter is a mandatory parameter for querying order trades")
	}

	remoteTrades, err := e.client.NewListTradesService().Symbol(q.Symbol).OrderId(orderID).Do(ctx)
	if err != nil {
		return nil, err
	}

	var trades []types.Trade
	for _, t := range remoteTrades {
		localTrade, err := toGlobalTrade(*t, e.IsMargin)
		if err != nil {
			log.WithError(err).Errorf("binance: can not convert trade: %+v", t)
			continue
		}

		trades = append(trades, *localTrade)
	}

	trades = types.SortTradesAscending(trades)
	return trades, nil
}

func (e *Exchange) QueryOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	orderID, err := strconv.ParseInt(q.OrderID, 10, 64)
	if err != nil {
		return nil, err
	}

	var order *binance.Order
	if e.IsMargin {
		order, err = e.client.NewGetMarginOrderService().Symbol(q.Symbol).OrderID(orderID).Do(ctx)
	} else {
		order, err = e.client.NewGetOrderService().Symbol(q.Symbol).OrderID(orderID).Do(ctx)
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

	if err = orderLimiter.Wait(ctx); err != nil {
		log.WithError(err).Errorf("order rate limiter wait error")
		return nil, err
	}

	log.Infof("querying closed orders %s from %s <=> %s ...", symbol, since, until)

	if e.IsMargin {
		req := e.client.NewListMarginOrdersService().Symbol(symbol)
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

		return toGlobalOrders(binanceOrders, e.IsMargin)
	}

	if e.IsFutures {
		return e.queryFuturesClosedOrders(ctx, symbol, since, until, lastOrderID)
	}

	// If orderId is set, it will get orders >= that orderId. Otherwise most recent orders are returned.
	// For some historical orders cummulativeQuoteQty will be < 0, meaning the data is not available at this time.
	// If startTime and/or endTime provided, orderId is not required.
	req := e.client.NewListOrdersService().
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

	return toGlobalOrders(binanceOrders, e.IsMargin)
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) (err error) {
	if err = orderLimiter.Wait(ctx); err != nil {
		log.WithError(err).Errorf("order rate limiter wait error")
		return err
	}

	if e.IsFutures {
		return e.cancelFuturesOrders(ctx, orders...)
	}

	for _, o := range orders {
		if e.IsMargin {
			var req = e.client.NewCancelMarginOrderService()
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
			var req = e.client.NewCancelOrderService()
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

	req := e.client.NewCreateMarginOrderService().
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

func (e *Exchange) submitSpotOrder(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
	orderType, err := toLocalOrderType(order.Type)
	if err != nil {
		return nil, err
	}

	req := e.client.NewCreateOrderService().
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

func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (createdOrder *types.Order, err error) {
	if err = orderLimiter.Wait(ctx); err != nil {
		log.WithError(err).Errorf("order rate limiter wait error")
		return nil, err
	}

	if e.IsMargin {
		createdOrder, err = e.submitMarginOrder(ctx, order)
	} else if e.IsFutures {
		createdOrder, err = e.submitFuturesOrder(ctx, order)
	} else {
		createdOrder, err = e.submitSpotOrder(ctx, order)
	}

	return createdOrder, err
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
	if e.IsFutures {
		return e.QueryFuturesKLines(ctx, symbol, interval, options)
	}

	var limit = 1000
	if options.Limit > 0 {
		// default limit == 1000
		limit = options.Limit
	}

	log.Infof("querying kline %s %s %v", symbol, interval, options)

	req := e.client.NewKlinesService().
		Symbol(symbol).
		Interval(string(interval)).
		Limit(limit)

	if options.StartTime != nil {
		req.StartTime(options.StartTime.UnixMilli())
	}

	if options.EndTime != nil {
		req.EndTime(options.EndTime.UnixMilli())
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

	kLines = types.SortKLinesAscending(kLines)
	return kLines, nil
}

func (e *Exchange) queryMarginTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) (trades []types.Trade, err error) {
	var remoteTrades []*binance.TradeV3
	req := e.client.NewListMarginTradesService().
		IsIsolated(e.IsIsolatedMargin).
		Symbol(symbol)

	if options.Limit > 0 {
		req.Limit(int(options.Limit))
	} else {
		req.Limit(1000)
	}

	// BINANCE seems to have an API bug, we can't use both fromId and the start time/end time
	// BINANCE uses inclusive last trade ID
	if options.LastTradeID > 0 {
		req.FromID(int64(options.LastTradeID))
	} else {
		if options.StartTime != nil && options.EndTime != nil {
			if options.EndTime.Sub(*options.StartTime) < 24*time.Hour {
				req.StartTime(options.StartTime.UnixMilli())
				req.EndTime(options.EndTime.UnixMilli())
			} else {
				req.StartTime(options.StartTime.UnixMilli())
			}
		} else if options.StartTime != nil {
			req.StartTime(options.StartTime.UnixMilli())
		} else if options.EndTime != nil {
			req.EndTime(options.EndTime.UnixMilli())
		}
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

	trades = types.SortTradesAscending(trades)
	return trades, nil
}

func (e *Exchange) querySpotTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) (trades []types.Trade, err error) {
	req := e.client2.NewGetMyTradesRequest()
	req.Symbol(symbol)

	// BINANCE uses inclusive last trade ID
	if options.LastTradeID > 0 {
		req.FromID(options.LastTradeID)
	} else {
		if options.StartTime != nil && options.EndTime != nil {
			if options.EndTime.Sub(*options.StartTime) < 24*time.Hour {
				req.StartTime(*options.StartTime)
				req.EndTime(*options.EndTime)
			} else {
				req.StartTime(*options.StartTime)
			}
		} else if options.StartTime != nil {
			req.StartTime(*options.StartTime)
		} else if options.EndTime != nil {
			req.EndTime(*options.EndTime)
		}
	}

	if options.Limit > 0 {
		req.Limit(uint64(options.Limit))
	} else {
		req.Limit(1000)
	}

	remoteTrades, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	for _, t := range remoteTrades {
		localTrade, err := toGlobalTrade(t, e.IsMargin)
		if err != nil {
			log.WithError(err).Errorf("can not convert binance trade: %+v", t)
			continue
		}

		trades = append(trades, *localTrade)
	}

	trades = types.SortTradesAscending(trades)
	return trades, nil
}

func (e *Exchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
	if err := queryTradeLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	if e.IsMargin {
		return e.queryMarginTrades(ctx, symbol, options)
	} else if e.IsFutures {
		return e.queryFuturesTrades(ctx, symbol, options)
	}
	return e.querySpotTrades(ctx, symbol, options)
}

// DefaultFeeRates returns the Binance VIP 0 fee schedule
// See also https://www.binance.com/en/fee/schedule
// See futures fee at: https://www.binance.com/en/fee/futureFee
func (e *Exchange) DefaultFeeRates() types.ExchangeFee {
	if e.IsFutures {
		return types.ExchangeFee{
			MakerFeeRate: fixedpoint.NewFromFloat(0.01 * 0.0180), // 0.0180% -USDT with BNB
			TakerFeeRate: fixedpoint.NewFromFloat(0.01 * 0.0360), // 0.0360% -USDT with BNB
		}
	}

	return types.ExchangeFee{
		MakerFeeRate: fixedpoint.NewFromFloat(0.01 * 0.075), // 0.075% with BNB
		TakerFeeRate: fixedpoint.NewFromFloat(0.01 * 0.075), // 0.075% with BNB
	}
}

// QueryDepth query the order book depth of a symbol
func (e *Exchange) QueryDepth(ctx context.Context, symbol string) (snapshot types.SliceOrderBook, finalUpdateID int64, err error) {
	if e.IsFutures {
		return e.queryFuturesDepth(ctx, symbol)
	}

	response, err := e.client.NewDepthService().Symbol(symbol).Do(ctx)
	if err != nil {
		return snapshot, finalUpdateID, err
	}

	return convertDepth(snapshot, symbol, finalUpdateID, response)
}

func convertDepth(snapshot types.SliceOrderBook, symbol string, finalUpdateID int64, response *binance.DepthResponse) (types.SliceOrderBook, int64, error) {
	snapshot.Symbol = symbol
	// empty time since the API does not provide time information.
	snapshot.Time = time.Time{}
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

// QueryPremiumIndex is only for futures
func (e *Exchange) QueryPremiumIndex(ctx context.Context, symbol string) (*types.PremiumIndex, error) {
	// when symbol is set, only one index will be returned.
	indexes, err := e.futuresClient.NewPremiumIndexService().Symbol(symbol).Do(ctx)
	if err != nil {
		return nil, err
	}

	return convertPremiumIndex(indexes[0])
}

func (e *Exchange) QueryFundingRateHistory(ctx context.Context, symbol string) (*types.FundingRate, error) {
	rates, err := e.futuresClient.NewFundingRateService().
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
	// when symbol is set, only one position risk will be returned.
	risks, err := e.futuresClient.NewGetPositionRiskService().Symbol(symbol).Do(ctx)
	if err != nil {
		return nil, err
	}

	return convertPositionRisk(risks[0])
}

// in seconds
var SupportedIntervals = map[types.Interval]int{
	types.Interval1s:  1,
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
	types.Interval1w:  60 * 60 * 24 * 7,
}

func (e *Exchange) SupportedInterval() map[types.Interval]int {
	return SupportedIntervals
}

func (e *Exchange) IsSupportedInterval(interval types.Interval) bool {
	_, ok := SupportedIntervals[interval]
	return ok
}

func getLaunchDate() (time.Time, error) {
	// binance launch date 12:00 July 14th, 2017
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Time{}, err
	}

	return time.Date(2017, time.July, 14, 0, 0, 0, 0, loc), nil
}

// Margin tolerance ranges from 0.0 (liquidation) to 1.0 (safest level of margin).
func calculateMarginTolerance(marginLevel fixedpoint.Value) fixedpoint.Value {
	if marginLevel.IsZero() {
		// Although margin level shouldn't be zero, that would indicate a significant problem.
		// In that case, margin tolerance should return 0.0 to also reflect that problem.
		return fixedpoint.Zero
	}

	// Formula created by operations team for our binance code.  Liquidation occurs at 1.1,
	// so when marginLevel equals 1.1, the formula becomes 1.0 - 1.0, or zero.
	// = 1.0 - (1.1 / marginLevel)
	return fixedpoint.One.Sub(fixedpoint.NewFromFloat(1.1).Div(marginLevel))
}

func logResponse(resp interface{}, err error, req interface{}) error {
	if err != nil {
		log.WithError(err).Errorf("%T: error %+v", req, resp)
		return err
	}

	log.Infof("%T: response: %+v", req, resp)
	return nil
}
