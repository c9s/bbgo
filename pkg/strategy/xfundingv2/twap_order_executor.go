package xfundingv2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

type TWAPExecutor struct {
	ctx      context.Context
	exchange types.ExchangeOrderQueryService
	executor *bbgo.GeneralOrderExecutor

	syncState TWAPExecutorSyncState
	dryRun    bool

	logger logrus.FieldLogger
}

type TWAPExecuteOrderOptions struct {
	DeadlineExceeded bool
	ReduceOnly       bool
}

func NewTWAPExecutor(
	ctx context.Context,
	exchange types.ExchangeOrderQueryService,
	isFutures bool,
	market types.Market,
	executor *bbgo.GeneralOrderExecutor,
	config TWAPWorkerConfig,
) *TWAPExecutor {
	return &TWAPExecutor{
		ctx:      ctx,
		exchange: exchange,
		executor: executor,

		syncState: TWAPExecutorSyncState{
			Config:    config,
			Market:    market,
			IsFutures: isFutures,
			Orders:    make(map[uint64]types.OrderQuery),
			Trades:    make(map[uint64]types.Trade),
		},
	}
}

func (o *TWAPExecutor) SetLogger(logger logrus.FieldLogger) {
	accountType := "spot"
	if o.syncState.IsFutures {
		accountType = "futures"
	}
	o.logger = logger.WithFields(logrus.Fields{
		"component":   "TWAPExecutor",
		"accountType": accountType,
	})
}

func (o *TWAPExecutor) SetDryRun(dryRun bool) {
	o.dryRun = dryRun
}

func (o *TWAPExecutor) Market() types.Market {
	return o.syncState.Market
}

func (o *TWAPExecutor) IsFutures() bool {
	return o.syncState.IsFutures
}

func (o *TWAPExecutor) Start() {
	if o.logger == nil {
		o.logger = logrus.WithFields(
			logrus.Fields{
				"component": "TWAPOrderExecutor",
				"symbol":    o.syncState.Market.Symbol,
			},
		)
	}
}

// AddTrade adds a trade to the executor's internal trade list if it belongs to
// an order managed by this executor.
func (o *TWAPExecutor) AddTrade(trade types.Trade) bool {
	_, exists := o.syncState.Orders[trade.OrderID]
	if exists {
		o.syncState.Trades[trade.ID] = trade
	}
	return exists
}

func (o *TWAPExecutor) Stop() error {
	// adding Stop method for future use
	return nil
}

// SyncOrder queries the latest order status and updates the internal store and trades.
// it's not thread-safe
func (o *TWAPExecutor) SyncOrder(order types.Order) error {
	storeOrder, exists := o.executor.OrderStore().Get(order.OrderID)
	if !exists {
		o.logger.Warnf("[SyncOrder] order not found in the order store, skipping sync: %s", order)
		return nil
	}

	if storeOrder.GetRemainingQuantity().IsZero() {
		return nil
	}

	timedCtx, cancel := context.WithTimeout(o.ctx, 500*time.Millisecond)
	defer cancel()

	query := order.AsQuery()
	updatedOrder, err := o.exchange.QueryOrder(
		timedCtx,
		query,
	)
	if err != nil {
		return fmt.Errorf("failed to query order: %v", query)
	}
	orderStore := o.executor.OrderStore()
	orderStore.Add(*updatedOrder)

	trades, err := o.exchange.QueryOrderTrades(timedCtx, query)
	if err != nil {
		return fmt.Errorf("failed to query order trades: %v, %v", query, err)
	}

	tradeCollector := o.executor.TradeCollector()
	for _, trade := range trades {
		tradeCollector.ProcessTrade(trade)
	}
	tradeCollector.Process()

	return nil
}

func (o *TWAPExecutor) GetOrder(orderID uint64) (types.Order, bool) {
	if _, exists := o.syncState.Orders[orderID]; !exists {
		o.logger.Debugf("[GetOrder] order not exists: %d", orderID)
		return types.Order{}, false
	}
	orderStore := o.executor.OrderStore()
	order, found := orderStore.Get(orderID)
	if found {
		return order, true
	}

	query := o.syncState.Orders[orderID]
	updatedOrder, err := o.exchange.QueryOrder(o.ctx, query)
	if err != nil {
		o.logger.Debugf("[GetOrder] failed to query order: %+v", query)
		return types.Order{}, false
	}
	orderStore.Add(*updatedOrder)
	return *updatedOrder, true
}

func (o *TWAPExecutor) UpdateOrder(update types.Order) {
	if _, exists := o.syncState.Orders[update.OrderID]; !exists {
		o.logger.Debugf("[UpdateOrder] order not exists: %d", update.OrderID)
		return
	}
	orderStore := o.executor.OrderStore()
	orderStore.Update(update)
}

func (o *TWAPExecutor) CancelOpenOrders(ctx context.Context) error {
	activeOrderBook := o.executor.ActiveMakerOrders()
	return o.executor.GracefulCancelActiveOrderBook(ctx, activeOrderBook)
}

func (o *TWAPExecutor) AllOrders() []types.Order {
	var orders []types.Order

	for orderID := range o.syncState.Orders {
		if order, exists := o.executor.OrderStore().Get(orderID); exists {
			orders = append(orders, order)
		}
	}
	return orders
}

func (o *TWAPExecutor) AllTrades() []types.Trade {
	var trades []types.Trade
	for _, trade := range o.syncState.Trades {
		trades = append(trades, trade)
	}
	return trades
}

// place order
func (o *TWAPExecutor) PlaceOrder(quantity fixedpoint.Value, side types.SideType, orderBook types.OrderBook, options TWAPExecuteOrderOptions) (*types.Order, error) {
	// find the better price and submit new order
	quantity = o.syncState.Market.TruncateQuantity(quantity)
	price, err := o.GetPrice(side, orderBook)
	o.logger.Debugf("quantity: %s, price: %s, side: %s", quantity, price, side)
	if err != nil {
		o.logger.WithError(err).Warn("[TWAP tick] failed to get price for active order update")
		return nil, err
	}
	price = o.syncState.Market.TruncatePrice(price)
	order := o.buildSubmitOrder(quantity, price, side, options)
	if order.Type != types.OrderTypeMarket && o.syncState.Market.IsDustQuantity(order.Quantity, order.Price) {
		return nil, fmt.Errorf("order is of dust quantity: %s", quantity)
	}

	if o.dryRun {
		msg := fmt.Sprintf("[TWAPExecutor] dry run mode, would have submitted order: %+v", order)
		o.logger.Warn(msg)
		return nil, errors.New(msg)
	}

	timedCtx, cancel := context.WithTimeout(o.ctx, 500*time.Millisecond)
	defer cancel()

	createdOrders, err := o.executor.SubmitOrders(timedCtx, order)
	if err != nil || len(createdOrders) == 0 {
		return nil, fmt.Errorf("failed to submit order: %+v, %v", order, err)
	}
	o.syncState.Orders[createdOrders[0].OrderID] = createdOrders[0].AsQuery()
	return &createdOrders[0], nil
}

func (o *TWAPExecutor) buildSubmitOrder(quantity, price fixedpoint.Value, side types.SideType, options TWAPExecuteOrderOptions) types.SubmitOrder {
	if options.DeadlineExceeded {
		return types.SubmitOrder{
			Symbol:     o.syncState.Market.Symbol,
			Market:     o.syncState.Market,
			Side:       side,
			Type:       types.OrderTypeMarket,
			Quantity:   quantity,
			ReduceOnly: options.ReduceOnly,
		}
	}
	orderType := types.OrderTypeLimitMaker
	var timeInForce types.TimeInForce

	if o.syncState.Config.OrderType == TWAPOrderTypeTaker {
		orderType = types.OrderTypeLimit
		timeInForce = types.TimeInForceIOC
	}

	orderForm := types.SubmitOrder{
		Symbol:      o.syncState.Market.Symbol,
		Market:      o.syncState.Market,
		Side:        side,
		Type:        orderType,
		Quantity:    quantity,
		Price:       price,
		TimeInForce: timeInForce,
		ReduceOnly:  options.ReduceOnly,
	}
	return orderForm
}

func (o *TWAPExecutor) GetPrice(side types.SideType, orderBook types.OrderBook) (price fixedpoint.Value, err error) {
	defer func() {
		if err == nil {
			price = o.syncState.Market.TruncatePrice(price)
		}
	}()

	switch o.syncState.Config.OrderType {
	case TWAPOrderTypeTaker:
		return o.getTakerPrice(side, orderBook)
	case TWAPOrderTypeMaker:
		return o.getMakerPrice(side, orderBook)
	default:
		return o.getTakerPrice(side, orderBook)
	}
}

func (o *TWAPExecutor) getTakerPrice(side types.SideType, orderBook types.OrderBook) (fixedpoint.Value, error) {
	switch side {
	case types.SideTypeBuy:
		ask, ok := orderBook.BestAsk()
		if !ok {
			return fixedpoint.Zero, fmt.Errorf("no ask price available for %s", o.syncState.Market.Symbol)
		}
		price := ask.Price
		if o.syncState.Config.MaxSlippage.Sign() > 0 {
			price = price.Mul(fixedpoint.One.Add(o.syncState.Config.MaxSlippage))
		}
		return price, nil

	case types.SideTypeSell:
		bid, ok := orderBook.BestBid()
		if !ok {
			return fixedpoint.Zero, fmt.Errorf("no bid price available for %s", o.syncState.Market.Symbol)
		}
		price := bid.Price
		if o.syncState.Config.MaxSlippage.Sign() > 0 {
			price = price.Mul(fixedpoint.One.Sub(o.syncState.Config.MaxSlippage))
		}
		return price, nil

	default:
		return fixedpoint.Zero, fmt.Errorf("unknown side: %s", side)
	}
}

func (o *TWAPExecutor) getMakerPrice(side types.SideType, orderBook types.OrderBook) (fixedpoint.Value, error) {
	tickSize := o.syncState.Market.TickSize
	numOfTicks := fixedpoint.NewFromInt(int64(o.syncState.Config.NumOfTicks))
	tickImprovement := tickSize.Mul(numOfTicks)

	switch side {
	case types.SideTypeBuy:
		bid, ok := orderBook.BestBid()
		if !ok {
			return fixedpoint.Zero, fmt.Errorf("no bid price available for %s", o.syncState.Market.Symbol)
		}
		// improve price by moving closer to spread
		price := bid.Price.Add(tickImprovement)
		// but don't cross the spread
		ask, hasAsk := orderBook.BestAsk()
		if hasAsk && price.Compare(ask.Price) >= 0 {
			price = ask.Price.Sub(tickSize)
		}
		return price, nil

	case types.SideTypeSell:
		ask, ok := orderBook.BestAsk()
		if !ok {
			return fixedpoint.Zero, fmt.Errorf("no ask price available for %s", o.syncState.Market.Symbol)
		}
		price := ask.Price.Sub(tickImprovement)
		bid, hasBid := orderBook.BestBid()
		if hasBid && price.Compare(bid.Price) <= 0 {
			price = bid.Price.Add(tickSize)
		}
		return price, nil

	default:
		return fixedpoint.Zero, fmt.Errorf("unknown side: %s", side)
	}
}

// cancel order
func (o *TWAPExecutor) CancelOrder(ctx context.Context, order types.Order) error {
	if _, found := o.syncState.Orders[order.OrderID]; !found {
		o.logger.Warnf("[TWAPExecutor] order not found, skipping cancel: %s", order)
		return nil
	}

	o.logger.Debugf("[TWAPExecutor] canceling order: %s", order)
	// NOTE: GracefulCancel will ensure the order is canceled before returning. That is, it may keep trying forever.
	// Add a notification ticker to notify if it takes too long to cancel the order.
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// do GracefulCancel
	cancelErrC := make(chan error, 1)
	go func() {
		cancelErrC <- o.executor.GracefulCancel(ctx, order)
	}()

	for {
		select {
		case <-ticker.C:
			bbgo.Notify("[TWAPExecutor] taking too long to cancel order: %s", order)
		case err := <-cancelErrC:
			return err
		}
	}
}
