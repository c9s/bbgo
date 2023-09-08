package binance

import (
	"context"
	"fmt"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/google/uuid"
	"go.uber.org/multierr"

	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func (e *Exchange) queryFuturesClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) (orders []types.Order, err error) {
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
	return toGlobalFuturesOrders(binanceOrders, false)
}

func (e *Exchange) TransferFuturesAccountAsset(ctx context.Context, asset string, amount fixedpoint.Value, io types.TransferDirection) error {
	req := e.client2.NewFuturesTransferRequest()
	req.Asset(asset)
	req.Amount(amount.String())

	if io == types.TransferIn {
		req.TransferType(binanceapi.FuturesTransferSpotToUsdtFutures)
	} else if io == types.TransferOut {
		req.TransferType(binanceapi.FuturesTransferUsdtFuturesToSpot)
	} else {
		return fmt.Errorf("unexpected transfer direction: %d given", io)
	}

	resp, err := req.Do(ctx)

	switch io {
	case types.TransferIn:
		log.Infof("internal transfer (spot) => (futures) %s %s, transaction = %+v, err = %+v", amount.String(), asset, resp, err)
	case types.TransferOut:
		log.Infof("internal transfer (futures) => (spot) %s %s, transaction = %+v, err = %+v", amount.String(), asset, resp, err)
	}

	return err
}

// QueryFuturesAccount gets the futures account balances from Binance
// Balance.Available = Wallet Balance(in Binance UI) - Used Margin
// Balance.Locked = Used Margin
func (e *Exchange) QueryFuturesAccount(ctx context.Context) (*types.Account, error) {
	//account, err := e.futuresClient.NewGetAccountService().Do(ctx)
	reqAccount := e.futuresClient2.NewFuturesGetAccountRequest()
	account, err := reqAccount.Do(ctx)
	if err != nil {
		return nil, err
	}

	req := e.futuresClient2.NewFuturesGetAccountBalanceRequest()
	accountBalances, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var balances = map[string]types.Balance{}
	for _, b := range accountBalances {
		// The futures account balance is much different from the spot balance:
		// - Balance is the actual balance of the asset
		// - AvailableBalance is the available margin balance (can be used as notional)
		// - CrossWalletBalance (this will be meaningful when using isolated margin)
		balances[b.Asset] = types.Balance{
			Currency:          b.Asset,
			Available:         b.AvailableBalance,                                  // AvailableBalance here is the available margin, like how much quantity/notional you can SHORT/LONG, not what you can withdraw
			Locked:            b.Balance.Sub(b.AvailableBalance.Sub(b.CrossUnPnl)), // FIXME: AvailableBalance is the available margin balance, it could be re-calculated by the current formula.
			MaxWithdrawAmount: b.MaxWithdrawAmount,
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

func (e *Exchange) cancelFuturesOrders(ctx context.Context, orders ...types.Order) (err error) {
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

func (e *Exchange) submitFuturesOrder(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
	orderType, err := toLocalFuturesOrderType(order.Type)
	if err != nil {
		return nil, err
	}

	req := e.futuresClient.NewCreateOrderService().
		Symbol(order.Symbol).
		Type(orderType).
		Side(futures.SideType(order.Side))

	if order.ReduceOnly {
		req.ReduceOnly(order.ReduceOnly)
	} else if order.ClosePosition {
		req.ClosePosition(order.ClosePosition)
	}

	clientOrderID := newFuturesClientOrderID(order.ClientOrderID)
	if len(clientOrderID) > 0 {
		req.NewClientOrderID(clientOrderID)
	}

	// use response result format
	req.NewOrderResponseType(futures.NewOrderRespTypeRESULT)

	if !order.ClosePosition {
		if order.Market.Symbol != "" {
			req.Quantity(order.Market.FormatQuantity(order.Quantity))
		} else {
			// TODO report error
			req.Quantity(order.Quantity.FormatString(8))
		}
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
		case types.OrderTypeLimit, types.OrderTypeLimitMaker, types.OrderTypeStopLimit:
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
	}, false)

	return createdOrder, err
}

func (e *Exchange) QueryFuturesKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {

	var limit = 1000
	if options.Limit > 0 {
		// default limit == 1000
		limit = options.Limit
	}

	log.Infof("querying kline %s %s %v", symbol, interval, options)

	req := e.futuresClient.NewKlinesService().
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

func (e *Exchange) queryFuturesTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) (trades []types.Trade, err error) {

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

	// The parameter fromId cannot be sent with startTime or endTime.
	// Mentioned in binance futures docs
	if options.LastTradeID <= 0 {
		if options.StartTime != nil && options.EndTime != nil {
			if options.EndTime.Sub(*options.StartTime) < 24*time.Hour {
				req.StartTime(options.StartTime.UnixMilli())
				req.EndTime(options.EndTime.UnixMilli())
			} else {
				req.StartTime(options.StartTime.UnixMilli())
			}
		} else if options.EndTime != nil {
			req.EndTime(options.EndTime.UnixMilli())
		}
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

	trades = types.SortTradesAscending(trades)
	return trades, nil
}

func (e *Exchange) QueryFuturesPositionRisks(ctx context.Context, symbol string) error {
	req := e.futuresClient.NewGetPositionRiskService()
	req.Symbol(symbol)
	res, err := req.Do(ctx)
	if err != nil {
		return err
	}

	_ = res

	return nil
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

func (e *Exchange) queryFuturesDepth(ctx context.Context, symbol string) (snapshot types.SliceOrderBook, finalUpdateID int64, err error) {
	res, err := e.futuresClient.NewDepthService().Symbol(symbol).Do(ctx)
	if err != nil {
		return snapshot, finalUpdateID, err
	}

	response := &binance.DepthResponse{
		LastUpdateID: res.LastUpdateID,
		Bids:         res.Bids,
		Asks:         res.Asks,
	}

	return convertDepth(snapshot, symbol, finalUpdateID, response)
}

func (e *Exchange) GetFuturesClient() *binanceapi.FuturesRestClient {
	return e.futuresClient2
}

// QueryFuturesIncomeHistory queries the income history on the binance futures account
// This is more binance futures specific API, the convert function is not designed yet.
// TODO: consider other futures platforms and design the common data structure for this
func (e *Exchange) QueryFuturesIncomeHistory(ctx context.Context, symbol string, incomeType binanceapi.FuturesIncomeType, startTime, endTime *time.Time) ([]binanceapi.FuturesIncome, error) {
	req := e.futuresClient2.NewFuturesGetIncomeHistoryRequest()
	req.Symbol(symbol)
	req.IncomeType(incomeType)
	if startTime != nil {
		req.StartTime(*startTime)
	}

	if endTime != nil {
		req.EndTime(*endTime)
	}

	resp, err := req.Do(ctx)
	return resp, err
}
