package binance

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func (e *Exchange) queryFuturesClosedOrders(
	ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64,
) (orders []types.Order, err error) {
	// Query regular orders
	req := e.futuresClient.NewListOrdersService().Symbol(symbol)

	if lastOrderID > 0 {
		req.OrderID(int64(lastOrderID))
	} else {
		req.StartTime(since.UnixMilli() / int64(time.Millisecond))
		if until.Sub(since) < 24*time.Hour {
			req.EndTime(until.UnixMilli() / int64(time.Millisecond))
		}
	}

	binanceOrders, err := req.Do(ctx)
	if err != nil {
		return orders, err
	}

	regularOrders, err := toGlobalFuturesOrders(binanceOrders, false)
	if err != nil {
		return orders, err
	}
	orders = append(orders, regularOrders...)

	// Query algo orders
	reqAlgo := e.futuresClient.NewListAllAlgoOrdersService().Symbol(symbol)
	if lastOrderID == 0 {
		reqAlgo.StartTime(since.UnixMilli() / int64(time.Millisecond))
		if until.Sub(since) < 24*time.Hour {
			reqAlgo.EndTime(until.UnixMilli() / int64(time.Millisecond))
		}
	} else {
		// For algo orders, we can use AlgoID to filter
		reqAlgo.AlgoID(int64(lastOrderID))
	}

	binanceAlgoOrders, err := reqAlgo.Do(ctx)
	if err != nil {
		// If algo orders query fails, still return regular orders
		return orders, nil
	}

	algoOrders, err := toGlobalFuturesOrders(binanceAlgoOrders, false)
	if err != nil {
		// If conversion fails, still return regular orders
		return orders, nil
	}
	orders = append(orders, algoOrders...)

	return orders, nil
}

func (e *Exchange) TransferFuturesAccountAsset(
	ctx context.Context, asset string, amount fixedpoint.Value, io types.TransferDirection,
) error {
	req := e.client2.NewFuturesTransferRequest()
	req.Asset(asset)
	req.Amount(amount.String())

	switch io {
	case types.TransferIn:
		req.TransferType(binanceapi.FuturesTransferSpotToUsdtFutures)
	case types.TransferOut:
		req.TransferType(binanceapi.FuturesTransferUsdtFuturesToSpot)
	default:
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

	reqRisks := e.futuresClient2.NewFuturesGetPositionRisksRequest()
	risks, err := reqRisks.Do(ctx)
	if err != nil {
		return nil, err
	}

	// out, _ := json.MarshalIndent(accountBalances, "", "  ")
	// fmt.Println(string(out))

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
		FuturesInfo: toGlobalFuturesAccountInfo(account, risks), // In binance GO api, Account define account info which maintain []*AccountAsset and []*AccountPosition.
		CanDeposit:  account.CanDeposit,                         // if can transfer in asset
		CanTrade:    account.CanTrade,                           // if can trade
		CanWithdraw: account.CanWithdraw,                        // if can transfer out asset
	}
	a.UpdateBalances(balances)
	return a, nil
}

func (e *Exchange) cancelFuturesOrders(ctx context.Context, orders ...types.Order) (err error) {
	for _, o := range orders {
		if isAlgoOrderType(o.Type) {
			err = multierr.Append(err, e.cancelFuturesAlgoOrder(ctx, o))
			continue
		}

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

func (e *Exchange) cancelFuturesAlgoOrder(ctx context.Context, order types.Order) error {
	// Cancel specific algo order by OrderID
	if order.OrderID > 0 {
		req := e.futuresClient.NewCancelAlgoOrderService().AlgoID(int64(order.OrderID))
		if order.ClientOrderID != "" {
			req.ClientAlgoID(order.ClientOrderID)
		}

		_, err := req.Do(ctx)
		return err
	}

	// Cancel all algo open orders for the symbol
	if order.Symbol != "" {
		req := e.futuresClient.NewCancelAllAlgoOpenOrdersService().
			Symbol(order.Symbol)
		return req.Do(ctx)
	}

	return fmt.Errorf("can not cancel algo order, order does not contain orderID or symbol")
}

func (e *Exchange) submitFuturesOrder(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
	// Check if this is an Algo order type
	if isAlgoOrderType(order.Type) {
		return e.submitFuturesAlgoOrder(ctx, order)
	}

	orderType, err := toLocalFuturesOrderType(order.Type)
	if err != nil {
		return nil, err
	}

	clientOrderID := newFuturesClientOrderID(order.ClientOrderID)

	// Select proper service: Algo for STOP_MARKET/TAKE_PROFIT_MARKET, regular otherwise
	useAlgo := order.Type == types.OrderTypeStopMarket || order.Type == types.OrderTypeTakeProfitMarket

	var (
		r       FuturesOrderRequest
		baseSvc *futures.CreateOrderService
		algoSvc *futures.CreateAlgoOrderService
	)

	if useAlgo {
		algoSvc = e.futuresClient.NewCreateAlgoOrderService()
		r = &FuturesCreateAlgoOrderPolyFill{CreateAlgoOrderService: algoSvc}
	} else {
		baseSvc = e.futuresClient.NewCreateOrderService()
		r = &FuturesCreateOrderPolyFill{CreateOrderService: baseSvc}
	}

	// Basic identifiers
	r.SetSymbol(order.Symbol)
	r.SetSide(futures.SideType(order.Side))
	r.SetTypeString(string(orderType))

	// Position flags
	if dualSidePosition {
		setDualSidePositionUnified(r, order)
	} else if order.ReduceOnly {
		r.SetReduceOnly(order.ReduceOnly)
	} else if order.ClosePosition {
		r.SetClosePosition(order.ClosePosition)
	}

	if len(clientOrderID) > 0 {
		r.SetNewClientOrderID(clientOrderID)
	}

	// use response result format (no-op for algo)
	r.SetNewOrderResponseType(futures.NewOrderRespTypeRESULT)

	if !order.ClosePosition {
		if order.Market.Symbol != "" {
			r.SetQuantity(order.Market.FormatQuantity(order.Quantity))
		} else {
			// TODO report error
			r.SetQuantity(order.Quantity.FormatString(8))
		}
	}

	// set price field for limit orders
	switch order.Type {
	case types.OrderTypeStopLimit, types.OrderTypeLimit, types.OrderTypeLimitMaker:
		if order.Market.Symbol != "" {
			r.SetPrice(order.Market.FormatPrice(order.Price))
		} else {
			// TODO report error
			r.SetPrice(order.Price.FormatString(8))
		}
	}

	// set stop/trigger price
	switch order.Type {
	case types.OrderTypeStopLimit, types.OrderTypeStopMarket, types.OrderTypeTakeProfitMarket:
		if order.Market.Symbol != "" {
			r.SetStopPrice(order.Market.FormatPrice(order.StopPrice))
		} else {
			// TODO report error
			r.SetStopPrice(order.StopPrice.FormatString(8))
		}
	}

	// could be IOC or FOK
	if len(order.TimeInForce) > 0 {
		// TODO: check the TimeInForce value
		r.SetTimeInForce(futures.TimeInForceType(order.TimeInForce))
	} else {
		switch order.Type {
		case types.OrderTypeLimit, types.OrderTypeLimitMaker, types.OrderTypeStopLimit:
			r.SetTimeInForce(futures.TimeInForceTypeGTC)
		}
	}

	if useAlgo {
		response, err := algoSvc.Do(ctx)
		if err != nil {
			return nil, err
		}
		log.Infof("futures algo order creation response: %+v", response)
		createdOrder, err := toGlobalFuturesAlgoOrder(response)
		return createdOrder, err
	}

	response, err := baseSvc.Do(ctx)
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
		ClosePosition:    response.ClosePosition,
	}, false)

	return createdOrder, err
}

// submitFuturesAlgoOrder submits an Algo order (conditional order) to Binance Futures
func (e *Exchange) submitFuturesAlgoOrder(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
	orderType, err := toLocalFuturesAlgoOrderType(order.Type)
	if err != nil {
		return nil, err
	}

	req := e.futuresClient.NewCreateAlgoOrderService().
		Symbol(order.Symbol).
		Type(orderType).
		Side(futures.SideType(order.Side))

	// Set client order ID
	clientOrderID := newFuturesClientOrderID(order.ClientOrderID)
	if len(clientOrderID) > 0 {
		req.ClientAlgoId(clientOrderID)
	}

	// Set position side for hedge mode (dual side position mode)
	// According to Binance API: positionSide must be sent in Hedge Mode
	if dualSidePosition {
		setDualSidePosition(req, order)
	}

	// Set quantity (not supported when closePosition is true)
	if !order.ClosePosition {
		if order.Market.Symbol != "" {
			req.Quantity(order.Market.FormatQuantity(order.Quantity))
		} else {
			req.Quantity(order.Quantity.FormatString(8))
		}
	}

	// Set price for limit orders (STOP and TAKE_PROFIT)
	switch order.Type {
	case types.OrderTypeStopLimit, types.OrderTypeTakeProfit:
		if order.Market.Symbol != "" {
			req.Price(order.Market.FormatPrice(order.Price))
		} else {
			req.Price(order.Price.FormatString(8))
		}
	}

	// Set trigger price (stop price)
	if order.StopPrice.Sign() > 0 {
		if order.Market.Symbol != "" {
			req.TriggerPrice(order.Market.FormatPrice(order.StopPrice))
		} else {
			req.TriggerPrice(order.StopPrice.FormatString(8))
		}
	}

	// Set time in force
	if len(order.TimeInForce) > 0 {
		req.TimeInForce(futures.TimeInForceType(order.TimeInForce))
	} else {
		// Default to GTC for limit orders
		switch order.Type {
		case types.OrderTypeStopLimit, types.OrderTypeTakeProfit:
			req.TimeInForce(futures.TimeInForceTypeGTC)
		}
	}

	// Set reduce only and close position
	// According to Binance API:
	// - reduceOnly cannot be sent in Hedge Mode
	// - reduceOnly cannot be sent with closePosition=true
	if order.ReduceOnly && !dualSidePosition && !order.ClosePosition {
		req.ReduceOnly(order.ReduceOnly)
	}
	if order.ClosePosition {
		req.ClosePosition(order.ClosePosition)
	}

	response, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	log.Infof("futures algo order creation response: %+v", response)

	// Convert response to global order type
	createdOrder, err := toGlobalFuturesOrder(response, false)
	return createdOrder, err
}

func (e *Exchange) QueryFuturesKLines(
	ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions,
) ([]types.KLine, error) {

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

func (e *Exchange) queryFuturesTrades(
	ctx context.Context, symbol string, options *types.TradeQueryOptions,
) (trades []types.Trade, err error) {

	var remoteTrades []*futures.AccountTrade
	req := e.futuresClient.NewListAccountTradeService().
		Symbol(symbol)
	if options.Limit > 0 {
		req.Limit(int(options.Limit))
	} else {
		req.Limit(1000)
	}

	// The parameter fromId cannot be sent with startTime or endTime.
	// Mentioned in binance futures docs
	// According to the guideline of types.TradeQueryOptions, we prioritize startTime/endTime over LastTradeID
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
	} else if options.LastTradeID > 0 {
		// BINANCE uses inclusive last trade ID
		req.FromID(int64(options.LastTradeID))
	}
	if (options.StartTime != nil || options.EndTime != nil) && options.LastTradeID > 0 {
		log.Debugf("both startTime/endTime and lastTradeID are set in TradeQueryOptions, lastTradeID will be ignored")
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

func (e *Exchange) queryFuturesDepth(
	ctx context.Context, symbol string,
) (snapshot types.SliceOrderBook, finalUpdateID int64, err error) {
	res, err := e.futuresClient.NewDepthService().Symbol(symbol).Limit(DefaultFuturesDepthLimit).Do(ctx)
	if err != nil {
		return snapshot, finalUpdateID, err
	}

	response := &binance.DepthResponse{
		LastUpdateID: res.LastUpdateID,
		Bids:         res.Bids,
		Asks:         res.Asks,
	}

	return convertDepthLegacy(snapshot, symbol, finalUpdateID, response)
}

func (e *Exchange) GetFuturesClient() *binanceapi.FuturesRestClient {
	return e.futuresClient2
}

// QueryFuturesIncomeHistory queries the income history on the binance futures account
// This is more binance futures specific API, the convert function is not designed yet.
// TODO: consider other futures platforms and design the common data structure for this
func (e *Exchange) QueryFuturesIncomeHistory(
	ctx context.Context, symbol string, incomeType binanceapi.FuturesIncomeType, startTime, endTime *time.Time,
) ([]binanceapi.FuturesIncome, error) {
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

func (e *Exchange) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	if e.IsFutures {
		_, err := e.futuresClient2.NewFuturesChangeInitialLeverageRequest().
			Symbol(symbol).
			Leverage(leverage).
			Do(ctx)
		return err
	}

	return fmt.Errorf("not supported set leverage")
}

func (e *Exchange) QueryPositionRisk(ctx context.Context, symbol ...string) ([]types.PositionRisk, error) {
	if !e.IsFutures {
		return nil, fmt.Errorf("not supported for non-futures exchange")
	}

	req := e.futuresClient2.NewFuturesGetPositionRisksRequest()
	if len(symbol) == 1 {
		req.Symbol(symbol[0])
		positions, err := req.Do(ctx)
		if err != nil {
			return nil, err
		}
		return toGlobalPositionRisk(positions), nil
	}

	positions, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	if len(symbol) == 0 {
		return toGlobalPositionRisk(positions), nil
	}

	symbolSet := make(map[string]struct{}, len(symbol))
	for _, s := range symbol {
		symbolSet[s] = struct{}{}
	}

	filteredPositions := make([]binanceapi.FuturesPositionRisk, 0, len(symbol))
	for _, pos := range positions {
		if _, ok := symbolSet[pos.Symbol]; ok {
			filteredPositions = append(filteredPositions, pos)
		}
	}

	return toGlobalPositionRisk(filteredPositions), nil
}

func (e *Exchange) queryFuturesOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	req := e.futuresClient.NewListOpenOrdersService().Symbol(symbol)
	binanceOrders, err := req.Do(ctx)
	if err != nil {
		return orders, err
	}

	globalOrders, err := toGlobalFuturesOrders(binanceOrders, false)
	if err != nil {
		return orders, err
	}
	orders = append(orders, globalOrders...)

	reqAlgo := e.futuresClient.NewListAllAlgoOrdersService().Symbol(symbol)
	binanceAlgoOrders, err := reqAlgo.Do(ctx)
	if err != nil {
		return orders, err
	}

	globalOrders, err = toGlobalFuturesOrders(binanceAlgoOrders, false)
	if err != nil {
		return orders, err
	}

	orders = append(orders, globalOrders...)
	return
}

func (e *Exchange) queryFuturesOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	if q.Symbol == "" {
		return nil, errors.New("symbol is required")
	}

	// Try to query regular order first
	if order, err := e.queryRegularFuturesOrder(ctx, q); err == nil {
		return order, nil
	}

	// If regular order query fails, try algo order
	return e.queryAlgoFuturesOrder(ctx, q)
}

// queryRegularFuturesOrder queries a regular futures order
func (e *Exchange) queryRegularFuturesOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	req := e.futuresClient.NewGetOrderService().Symbol(q.Symbol)

	if len(q.OrderID) > 0 {
		orderID, err := strconv.ParseInt(q.OrderID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid order id: %w", err)
		}
		req.OrderID(orderID)
	} else if len(q.ClientOrderID) > 0 {
		req.OrigClientOrderID(q.ClientOrderID)
	} else {
		return nil, errors.New("order id or client order id is required")
	}

	order, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalFuturesOrder(order, false)
}

// queryAlgoFuturesOrder queries an algo futures order
func (e *Exchange) queryAlgoFuturesOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	req := e.futuresClient.NewGetAlgoOrderService()

	if len(q.OrderID) > 0 {
		orderID, err := strconv.ParseInt(q.OrderID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid algo order id: %w", err)
		}
		req.AlgoID(orderID)
	} else if len(q.ClientOrderID) > 0 {
		req.ClientAlgoID(q.ClientOrderID)
	} else {
		return nil, errors.New("algo order id or client algo order id is required")
	}

	algoOrder, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalFuturesOrder(algoOrder, false)
}

// OrderServiceConstraint is a type constraint for order services that support setting position side
type OrderServiceConstraint interface {
	*futures.CreateOrderService | *futures.CreateAlgoOrderService
}

func setDualSidePosition[T OrderServiceConstraint](req T, order types.SubmitOrder) {
	var positionSide futures.PositionSideType

	switch order.Side {
	case types.SideTypeBuy:
		if order.ReduceOnly {
			positionSide = futures.PositionSideTypeShort
		} else {
			positionSide = futures.PositionSideTypeLong
		}
	case types.SideTypeSell:
		if order.ReduceOnly {
			positionSide = futures.PositionSideTypeLong
		} else {
			positionSide = futures.PositionSideTypeShort
		}
	}

	// Use type assertion to call PositionSide method
	switch v := any(req).(type) {
	case *futures.CreateOrderService:
		v.PositionSide(positionSide)
	case *futures.CreateAlgoOrderService:
		v.PositionSide(positionSide)
	}
}

// setDualSidePositionUnified sets the PositionSide field through the unified
// FuturesOrderRequest interface, mirroring the logic of setDualSidePosition.
func setDualSidePositionUnified(req FuturesOrderRequest, order types.SubmitOrder) {
	switch order.Side {
	case types.SideTypeBuy:
		if order.ReduceOnly {
			req.SetPositionSide(futures.PositionSideTypeShort)
		} else {
			req.SetPositionSide(futures.PositionSideTypeLong)
		}
	case types.SideTypeSell:
		if order.ReduceOnly {
			req.SetPositionSide(futures.PositionSideTypeLong)
		} else {
			req.SetPositionSide(futures.PositionSideTypeShort)
		}
	}
}

type FuturesCreateOrderPolyFill struct {
	*futures.CreateOrderService
}

// FuturesOrderRequest is a unified interface for configuring both standard
// futures orders and algo futures orders. It exposes a common set of setters
// so callers can treat both services uniformly.
type FuturesOrderRequest interface {
	// Core order flags
	SetReduceOnly(bool)
	SetSide(futures.SideType)
	SetPositionSide(futures.PositionSideType)
	SetClosePosition(bool)

	// Identification and response
	SetNewOrderResponseType(futures.NewOrderRespType)
	SetNewClientOrderID(string)

	// Common order fields
	SetSymbol(string)
	// Use string to be compatible with both regular OrderType and AlgoOrderType
	SetTypeString(string)
	SetQuantity(string)
	SetPrice(string)
	// Stop/trigger price: maps to StopPrice for regular orders and TriggerPrice for algo orders
	SetStopPrice(string)

	// Optional common extras
	SetTimeInForce(futures.TimeInForceType)
}

func (f *FuturesCreateOrderPolyFill) SetSide(side futures.SideType) {
	f.CreateOrderService.Side(side)
}

func (f *FuturesCreateOrderPolyFill) SetPositionSide(side futures.PositionSideType) {
	f.CreateOrderService.PositionSide(side)
}

func (f *FuturesCreateOrderPolyFill) SetReduceOnly(reduceOnly bool) {
	f.CreateOrderService.ReduceOnly(reduceOnly)
}

func (f *FuturesCreateOrderPolyFill) SetClosePosition(closePosition bool) {
	f.CreateOrderService.ClosePosition(closePosition)
}

func (f *FuturesCreateOrderPolyFill) SetNewClientOrderID(clientOrderID string) {
	f.CreateOrderService.NewClientOrderID(clientOrderID)
}

func (f *FuturesCreateOrderPolyFill) SetNewOrderResponseType(respType futures.NewOrderRespType) {
	f.CreateOrderService.NewOrderResponseType(respType)
}

func (f *FuturesCreateOrderPolyFill) SetSymbol(symbol string) {
	f.CreateOrderService.Symbol(symbol)
}

func (f *FuturesCreateOrderPolyFill) SetTypeString(t string) {
	f.CreateOrderService.Type(futures.OrderType(t))
}

func (f *FuturesCreateOrderPolyFill) SetQuantity(q string) {
	f.CreateOrderService.Quantity(q)
}

func (f *FuturesCreateOrderPolyFill) SetPrice(p string) {
	f.CreateOrderService.Price(p)
}

func (f *FuturesCreateOrderPolyFill) SetStopPrice(sp string) {
	f.CreateOrderService.StopPrice(sp)
}

func (f *FuturesCreateOrderPolyFill) SetTimeInForce(tif futures.TimeInForceType) {
	f.CreateOrderService.TimeInForce(tif)
}

type FuturesCreateAlgoOrderPolyFill struct {
	*futures.CreateAlgoOrderService
}

func (f *FuturesCreateAlgoOrderPolyFill) SetSide(side futures.SideType) {
	f.CreateAlgoOrderService.Side(side)
}

func (f *FuturesCreateAlgoOrderPolyFill) SetReduceOnly(reduceOnly bool) {
	f.CreateAlgoOrderService.ReduceOnly(reduceOnly)
}

func (f *FuturesCreateAlgoOrderPolyFill) SetPositionSide(side futures.PositionSideType) {
	f.CreateAlgoOrderService.PositionSide(side)
}

func (f *FuturesCreateAlgoOrderPolyFill) SetClosePosition(closePosition bool) {
	f.CreateAlgoOrderService.ClosePosition(closePosition)
}

// For algo orders, the client identifier is clientAlgoId
func (f *FuturesCreateAlgoOrderPolyFill) SetNewClientOrderID(clientOrderID string) {
	f.CreateAlgoOrderService.ClientAlgoId(clientOrderID)
}

// Algo order API does not support NewOrderResponseType; keep as no-op to satisfy the interface
func (f *FuturesCreateAlgoOrderPolyFill) SetNewOrderResponseType(_ futures.NewOrderRespType) {
	// no-op
}

func (f *FuturesCreateAlgoOrderPolyFill) SetSymbol(symbol string) {
	f.CreateAlgoOrderService.Symbol(symbol)
}

func (f *FuturesCreateAlgoOrderPolyFill) SetTypeString(t string) {
	f.CreateAlgoOrderService.Type(futures.AlgoOrderType(t))
}

func (f *FuturesCreateAlgoOrderPolyFill) SetQuantity(q string) {
	f.CreateAlgoOrderService.Quantity(q)
}

func (f *FuturesCreateAlgoOrderPolyFill) SetPrice(p string) {
	f.CreateAlgoOrderService.Price(p)
}

// Maps common StopPrice to TriggerPrice used by algo orders
func (f *FuturesCreateAlgoOrderPolyFill) SetStopPrice(sp string) {
	f.CreateAlgoOrderService.TriggerPrice(sp)
}

func (f *FuturesCreateAlgoOrderPolyFill) SetTimeInForce(tif futures.TimeInForceType) {
	f.CreateAlgoOrderService.TimeInForce(tif)
}
