package xfundingv2

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util/tradingutil"
)

type TWAPOrderType string

const (
	TWAPOrderTypeMaker TWAPOrderType = "maker"
	TWAPOrderTypeTaker TWAPOrderType = "taker"
)

//go:generate stringer -type=TWAPWorkerState
type TWAPWorkerState int

const (
	TWAPWorkerStatePending TWAPWorkerState = iota
	TWAPWorkerStateRunning
	TWAPWorkerStateDone
)

type TWAPWorkerConfig struct {
	// Duration is the total time duration for the TWAP execution
	Duration types.Duration `json:"duration"`
	// ClosingDuration is the expected time duration for the closing phase of the TWAP execution.
	ClosingDuration types.Duration `json:"closingDuration"`

	// NumSlices is how many slices to divide the total time duration into.
	NumSlices int `json:"numSlices"`
	// OrderType specifies whether to use maker or taker orders for execution.
	OrderType TWAPOrderType `json:"orderType"`
	// CheckInterval is how often to check for price improvement for the active order.
	CheckInterval types.Duration `json:"checkInterval,omitempty"`

	MinSliceNotional fixedpoint.Value `json:"minSliceNotional,omitempty"`

	// optional configs
	// MaxSlippage is the maximum slippage ratio for taker orders (e.g. 0.001 = 0.1%)
	MaxSlippage fixedpoint.Value `json:"maxSlippage,omitempty"`
	// MaxSliceSize is the maximum quantity for each slice order
	MaxSliceSize fixedpoint.Value `json:"maxSliceSize,omitempty"`
	// MinSliceSize is the minimum quantity for each slice order
	MinSliceSize fixedpoint.Value `json:"minSliceSize,omitempty"`
	// NumOfTicks is for orders: number of ticks to improve price, maker only
	NumOfTicks int `json:"numOfTicks,omitempty"`
}

func (c *TWAPWorkerConfig) Defaults() {
	if c.Duration == 0 {
		c.Duration = types.Duration(2 * time.Hour)
	}
	if c.ClosingDuration == 0 {
		c.ClosingDuration = types.Duration(time.Hour)
	}
	if c.NumSlices == 0 {
		c.NumSlices = 4
	}
	if c.OrderType == "" {
		c.OrderType = TWAPOrderTypeMaker
	}
	if c.CheckInterval == 0 {
		c.CheckInterval = types.Duration(10 * time.Minute)
	}
	if c.MinSliceNotional.IsZero() {
		c.MinSliceNotional = fixedpoint.NewFromFloat(500.0) // $500
	}
}

type TWAPWorker struct {
	syncState   TWAPWorkerSyncState
	activeOrder *types.Order

	account *types.Account
	ctx     context.Context
	logger  logrus.FieldLogger
}

func NewTWAPWorker(
	ctx context.Context,
	symbol string,
	session *bbgo.ExchangeSession,
	generalExecutor *bbgo.GeneralOrderExecutor,
	config TWAPWorkerConfig,
) (*TWAPWorker, error) {
	market, found := session.Market(symbol)
	if !found {
		return nil, fmt.Errorf("market not found for symbol: %s", symbol)
	}
	service, ok := session.Exchange.(types.ExchangeOrderQueryService)
	if !ok {
		return nil, fmt.Errorf("exchange does not support OrderQueryService: %s", session.Exchange.Name())
	}
	w := &TWAPWorker{
		syncState: TWAPWorkerSyncState{
			Symbol:         symbol,
			Config:         config,
			State:          TWAPWorkerStatePending,
			TargetPosition: fixedpoint.Zero,
		},
	}
	w.ctx = ctx
	w.account = session.Account
	w.syncState.TWAPExecutor = NewTWAPExecutor(
		w.ctx,
		service,
		session.Futures,
		market,
		generalExecutor,
		config,
	)
	return w, nil
}

// SetTargetPosition sets the target position for the TWAP worker.
func (w *TWAPWorker) SetTargetPosition(targetPosition fixedpoint.Value) {
	w.syncState.TargetPosition = targetPosition
}

func (w *TWAPWorker) SetLogger(logger logrus.FieldLogger) {
	accountType := "spot"
	if w.syncState.TWAPExecutor.IsFutures() {
		accountType = "futures"
	}
	w.logger = logger.WithFields(logrus.Fields{
		"component":   "TWAPWorker",
		"accountType": accountType,
		"symbol":      w.syncState.Symbol,
	})
}

func (w *TWAPWorker) Symbol() string {
	return w.syncState.Symbol
}

func (w *TWAPWorker) Market() types.Market {
	return w.syncState.TWAPExecutor.Market()
}

func (w *TWAPWorker) Executor() *TWAPExecutor {
	return w.syncState.TWAPExecutor
}

func (w *TWAPWorker) State() TWAPWorkerState {
	return w.syncState.State
}

func (w *TWAPWorker) Duration() types.Duration {
	return w.syncState.Config.Duration
}

func (w *TWAPWorker) IsDone() bool {
	return w.syncState.State == TWAPWorkerStateDone
}

func (w *TWAPWorker) AveragePrice() fixedpoint.Value {
	trades := w.syncState.TWAPExecutor.AllTrades()
	return tradingutil.AveragePriceFromTrades(trades)
}

func (w *TWAPWorker) FilledPosition() fixedpoint.Value {
	trades := w.syncState.TWAPExecutor.AllTrades()
	position := fixedpoint.Zero
	for _, t := range trades {
		if t.Side == types.SideTypeBuy {
			position = position.Add(t.Quantity)
		} else {
			position = position.Sub(t.Quantity)
		}
	}
	return position
}

func (w *TWAPWorker) TotalFee() map[string]fixedpoint.Value {
	trades := w.syncState.TWAPExecutor.AllTrades()
	feeMap := make(map[string]fixedpoint.Value)
	for _, t := range trades {
		if t.FeeCurrency == "" || t.Fee.IsZero() {
			continue
		}
		feeMap[t.FeeCurrency] = feeMap[t.FeeCurrency].Add(t.Fee)
	}
	return feeMap
}

func (w *TWAPWorker) ActiveOrder() *types.Order {
	return w.activeOrder
}

func (w *TWAPWorker) RemainingQuantity() fixedpoint.Value {
	// remaining = target - filled
	// NOTE: the remaining quantity can be positive or negative.
	return w.syncState.TargetPosition.Sub(w.FilledPosition())
}

func (w *TWAPWorker) TargetPosition() fixedpoint.Value {
	return w.syncState.TargetPosition
}

func (w *TWAPWorker) Start(ctx context.Context, currentTime time.Time) error {
	// worker should be able to start only once from pending state
	if w.syncState.State != TWAPWorkerStatePending {
		return fmt.Errorf("cannot start TWAPWorker: expected state Pending, got %s", w.syncState.State)
	}

	if w.logger == nil {
		w.logger = logrus.WithFields(logrus.Fields{
			"component": "twap",
			"symbol":    w.syncState.Symbol,
		})
	}

	// start the executor
	w.syncState.TWAPExecutor.SetLogger(w.logger)
	w.syncState.TWAPExecutor.Start()

	w.ResetTime(currentTime, w.syncState.Config.Duration)

	w.logger.Infof(
		"[TWAP Start] started: targetPosition=%s, duration=%s, interval=%s",
		w.syncState.TargetPosition,
		w.syncState.Config.Duration.Duration(),
		w.syncState.PlaceOrderInterval,
	)
	return nil
}

func (w *TWAPWorker) RemainingDuration(currentTime time.Time) time.Duration {
	if currentTime.After(w.syncState.EndAt) {
		return 0
	}
	return w.syncState.EndAt.Sub(currentTime)
}

// ResetTime resets the start and end time of the TWAP execution.
// It can be used to extend the execution time by resetting the end time to a later time.
// ex: TWAP worker is opening a position and then switch to closing the position, we can reset the time for the closing.
func (w *TWAPWorker) ResetTime(currentTime time.Time, duration types.Duration) {
	w.syncState.State = TWAPWorkerStateRunning
	w.syncState.StartAt = currentTime
	w.syncState.Config.Duration = duration
	w.syncState.EndAt = currentTime.Add(w.syncState.Config.Duration.Duration())

	numSlices := w.syncState.Config.NumSlices
	if numSlices <= 0 {
		numSlices = 1
	}

	w.syncState.PlaceOrderInterval = w.syncState.Config.Duration.Duration() / time.Duration(numSlices)
	w.syncState.CurrentIntervalStart = currentTime
	w.syncState.CurrentIntervalEnd = w.syncState.CurrentIntervalStart.Add(w.syncState.PlaceOrderInterval)
	if w.syncState.CurrentIntervalEnd.After(w.syncState.EndAt) {
		w.syncState.CurrentIntervalEnd = w.syncState.EndAt
	}

	w.syncAndResetActiveOrder()
}

// Stop stops the TWAP worker and cancels any active order on the exchange.
func (w *TWAPWorker) Stop() {
	if w.syncState.State == TWAPWorkerStateRunning || w.syncState.State == TWAPWorkerStatePending {
		if w.activeOrder != nil {
			err := w.syncState.TWAPExecutor.CancelOrder(w.ctx, *w.activeOrder)
			if err != nil {
				w.logger.WithError(err).Warnf("[TWAP Stop] failed to cancel active order: %s", w.activeOrder)
			}
		}

		// stop executor
		if err := w.syncState.TWAPExecutor.Stop(); err != nil {
			w.logger.WithError(err).Warn("[TWAP Stop] failed to stop TWAP executor")
		}

		w.syncState.State = TWAPWorkerStateDone
		w.activeOrder = nil
		w.logger.Infof(
			"[TWAP Stop] stopped: filled=%s / target=%s",
			w.FilledPosition(), w.syncState.TargetPosition,
		)
	}
}

// syncAndResetActiveOrder queries the exchange for the latest order state and
// its trades via REST API, updates ordersMap and tradesMap accordingly, then
// resets activeOrder to nil. Must be called under lock.
func (w *TWAPWorker) syncAndResetActiveOrder() *types.Order {
	if w.activeOrder == nil {
		return nil
	}
	w.logger.Debugf("[TWAPWorker syncAndResetActiveOrder] sync and reset active order: %s", w.activeOrder)

	if err := w.syncState.TWAPExecutor.SyncOrder(*w.activeOrder); err != nil {
		w.logger.WithError(err).Warnf("[TWAP syncAndResetActiveOrder] fail to sync active order, resetting: %s", w.activeOrder.String())
		w.activeOrder = nil
		return nil
	}

	oriActiveOrder, _ := w.syncState.TWAPExecutor.GetOrder(w.activeOrder.OrderID)
	w.activeOrder = nil
	return &oriActiveOrder
}

// Tick is the main driver. It should be called on each external tick with the
// current time and an orderbook snapshot. It handles cancel-and-replace for
// maker orders, scheduling, quantity/price calculation, and order submission.
func (w *TWAPWorker) Tick(currentTime time.Time, orderBook types.OrderBook) error {
	defer func() {
		if currentTime.After(w.syncState.CurrentIntervalEnd) {
			w.syncState.CurrentIntervalStart = w.syncState.CurrentIntervalEnd
			w.syncState.CurrentIntervalEnd = w.syncState.CurrentIntervalStart.Add(w.syncState.PlaceOrderInterval)
			if w.syncState.CurrentIntervalStart.After(w.syncState.EndAt) {
				w.syncState.CurrentIntervalStart = w.syncState.EndAt
			}
			if w.syncState.CurrentIntervalEnd.After(w.syncState.EndAt) {
				w.syncState.CurrentIntervalEnd = w.syncState.EndAt
			}
		}
		if currentTime.After(w.syncState.EndAt) {
			w.syncState.State = TWAPWorkerStateDone
		}
	}()

	if w.syncState.State != TWAPWorkerStateRunning {
		// the worker is not running
		return nil
	}

	if currentTime.Before(w.syncState.CurrentIntervalStart) {
		// not time for the next order yet
		return nil
	}

	// it's running and currentTime is after the current interval start
	// time to check if we need to place/cancel/replace orders
	// NOTE: remaining can be positive or negative
	remaining := w.RemainingQuantity()

	// target reached, do nothing
	if remaining.IsZero() {
		return nil
	}

	// the existing active order is in the opposite direction of the remaining quantity
	if w.activeOrder != nil && remaining.Sign()*orderSide(remaining).Int() < 0 {
		if !w.activeOrder.GetRemainingQuantity().IsZero() {
			// cancel the active order
			w.logger.Infof("[TWAP tick] active order of opposite direction detected, canceling: %s", w.activeOrder)
			if err := w.syncState.TWAPExecutor.CancelOrder(w.ctx, *w.activeOrder); err != nil {
				return fmt.Errorf("[TWAP tick] failed to cancel partially filled active order in opposite direction: %w", err)
			}
		}
		w.syncAndResetActiveOrder()
	}

	market := w.Market()
	midPrice := getMidPrice(orderBook)

	// check if deadline exceeded
	deadlineExceeded := !currentTime.Before(w.syncState.EndAt)
	orderOptions := TWAPExecuteOrderOptions{
		DeadlineExceeded: deadlineExceeded,
		// use reduce-only if we are closing the position
		ReduceOnly: w.syncState.TargetPosition.Abs().Compare(w.FilledPosition().Abs()) < 0,
	}
	// if deadline exceeded, we want to place a final order for the remaining quantity
	if deadlineExceeded {
		if w.activeOrder != nil {
			if err := w.syncState.TWAPExecutor.CancelOrder(w.ctx, *w.activeOrder); err != nil {
				w.logger.WithError(err).Warn("[TWAP tick] failed to cancel active order when deadline exceeded")
				return nil
			}
			w.logger.Debug("deadline exceeded, reseting active order")
			w.syncAndResetActiveOrder()
		}
		// the remaining quantity is dust, do nothing
		if !midPrice.IsZero() && market.IsDustQuantity(remaining, midPrice) {
			return nil
		}
		createdOrder, err := w.syncState.TWAPExecutor.PlaceOrder(
			remaining.Abs(),
			orderSide(remaining),
			orderBook,
			orderOptions,
		)
		if err != nil || createdOrder == nil {
			return fmt.Errorf("failed to place final order when deadline exceeded: %w", err)
		}
		w.activeOrder = createdOrder
		return nil
	}
	// from here, deadline not exceeded

	// we don't have an active order, place a new one
	if w.activeOrder == nil {
		w.logger.Debugf("no active order, placing new order: %s", w.Symbol())
		sliceQty := w.calculateSliceQuantity(currentTime, remaining, false, market, midPrice)
		// the slice quantity is dust, do nothing
		if !midPrice.IsZero() && market.IsDustQuantity(sliceQty, midPrice) {
			w.logger.Infof("[TWAP tick] slice quantity is dust, skip creating active order: %s@%s", sliceQty, midPrice)
			return nil
		}
		createdOrder, err := w.syncState.TWAPExecutor.PlaceOrder(
			sliceQty,
			orderSide(remaining),
			orderBook,
			orderOptions,
		)
		if err != nil || createdOrder == nil {
			return fmt.Errorf("failed to place order: %w", err)
		}
		w.activeOrder = createdOrder
		w.logger.Infof("[TWAP tick] new active order created: %s", createdOrder)
		return nil
	}
	// from here, active order is not nil

	// we are within current interval and we have a better price
	if w.shouldUpdateActiveOrder(orderBook) && currentTime.Before(w.syncState.CurrentIntervalEnd) {
		// throttle order updates to avoid excessive cancel-and-replace
		if !w.syncState.LastCheckTime.IsZero() && currentTime.Sub(w.syncState.LastCheckTime) < w.syncState.Config.CheckInterval.Duration() {
			return nil
		}
		w.syncState.LastCheckTime = currentTime
		w.logger.Debugf("try to update active order: %s", w.activeOrder)

		// the remaining quantity of the active order is dust, no need to update
		remaining := w.activeOrder.GetRemainingQuantity()
		w.logger.Debugf("active order remaining quantity: %s", remaining)

		if err := w.syncState.TWAPExecutor.CancelOrder(w.ctx, *w.activeOrder); err != nil {
			w.logger.WithError(err).Warnf("[TWAP tick] failed to cancel active order: %s", w.activeOrder)
			return nil
		}
		// find the better price and submit new order
		createdOrder, err := w.syncState.TWAPExecutor.PlaceOrder(
			remaining,
			w.activeOrder.Side,
			orderBook,
			orderOptions,
		)
		if err != nil || createdOrder == nil {
			return fmt.Errorf("failed to place replacement order: %w", err)
		}
		oriActiveOrder := w.syncAndResetActiveOrder()
		w.activeOrder = createdOrder
		w.logger.Infof("[TWAP tick] active order updated: %s %s qty=%s(executed: %s)->%s price=%s->%s",
			createdOrder.Side,
			createdOrder.Type,
			oriActiveOrder.Quantity,
			oriActiveOrder.ExecutedQuantity,
			createdOrder.Quantity,
			oriActiveOrder.Price,
			createdOrder.Price,
		)
		return nil
	}

	// we are within the current interval, just wait for the next tick
	if currentTime.Before(w.syncState.CurrentIntervalEnd) {
		return nil
	}

	w.logger.Debugf("current interval ended, placing next slice order: %s > %s", currentTime, w.syncState.CurrentIntervalEnd)
	// currentTime is after current interval end, time to place the next slice order
	if oldActiveOrder := w.syncAndResetActiveOrder(); oldActiveOrder != nil && !oldActiveOrder.GetRemainingQuantity().IsZero() {
		// cancel the old active order before placing the next slice order
		if err := w.syncState.TWAPExecutor.CancelOrder(w.ctx, *oldActiveOrder); err != nil {
			return fmt.Errorf("[TWAP tick] failed to cancel old active order for next slice: %w", err)
		}
	}

	// calculate slice quantity
	sliceQty := w.calculateSliceQuantity(currentTime, remaining, deadlineExceeded, market, midPrice)
	// the slice quantity is dust, do nothing
	if !midPrice.IsZero() && market.IsDustQuantity(sliceQty, midPrice) {
		w.logger.Infof("slice quantity is dust, skip creating new active order: %s@%s", sliceQty, midPrice)
		return nil
	}
	createdOrder, err := w.syncState.TWAPExecutor.PlaceOrder(
		sliceQty,
		orderSide(remaining),
		orderBook,
		orderOptions,
	)
	if err != nil || createdOrder == nil {
		return fmt.Errorf("failed to place order for the next slice: %w", err)
	}
	w.logger.Infof("[TWAP tick] new active order created for next slice: %s", createdOrder)
	w.activeOrder = createdOrder

	return nil
}

func (w *TWAPWorker) calculateSliceQuantity(currentTime time.Time, remaining fixedpoint.Value, deadlineExceeded bool, market types.Market, price fixedpoint.Value) fixedpoint.Value {
	remaining = remaining.Abs()
	w.logger.Debugf("remaining quantity: %s@%s", remaining, price)

	if deadlineExceeded {
		w.logger.Debugf("deadline exceeded, placing final order: %s@%s", remaining, price)
		return remaining
	}

	// the remaining quantity is very small, just place the remaining quantity
	if !price.IsZero() && price.Mul(remaining).Compare(w.syncState.Config.MinSliceNotional) < 0 {
		return remaining
	}

	// dynamic slice: remaining / remaining_slices
	timeLeft := w.syncState.EndAt.Sub(currentTime)
	w.logger.Debugf("time left: %s", timeLeft)
	if timeLeft <= 0 {
		return remaining
	}

	remainingSlices := int(timeLeft / w.syncState.PlaceOrderInterval)
	if remainingSlices <= 0 {
		remainingSlices = 1
	}
	w.logger.Debugf("remaining slices: %d", remainingSlices)

	sliceQty := remaining.Div(fixedpoint.NewFromInt(int64(remainingSlices)))

	// apply min/max slice size constraints
	if w.syncState.Config.MaxSliceSize.Sign() > 0 && sliceQty.Compare(w.syncState.Config.MaxSliceSize) > 0 {
		sliceQty = w.syncState.Config.MaxSliceSize
	}
	if w.syncState.Config.MinSliceSize.Sign() > 0 && sliceQty.Compare(w.syncState.Config.MinSliceSize) < 0 {
		// if remaining is less than min, just use remaining
		if remaining.Compare(w.syncState.Config.MinSliceSize) <= 0 {
			sliceQty = remaining
		} else {
			sliceQty = w.syncState.Config.MinSliceSize
		}
	}

	if !price.IsZero() && market.IsDustQuantity(sliceQty, price) && remaining.Compare(sliceQty) > 0 {
		diff := remaining.Sub(sliceQty)
		n := diff.Div(market.MinQuantity).Round(0, fixedpoint.Up).Int64()
		for i := int64(0); i < n; i++ {
			if !market.IsDustQuantity(sliceQty, price) {
				break
			}
			sliceQty = sliceQty.Add(market.MinQuantity)
		}
	}

	// cap at remaining
	if sliceQty.Compare(remaining) > 0 {
		sliceQty = remaining
	}
	// cap at available balance for spot orders
	if !w.Executor().IsFutures() {
		switch orderSide(remaining) {
		case types.SideTypeSell:
			// check available base for sell
			base := w.Market().BaseCurrency
			if baseBalance, ok := w.account.Balance(base); ok {
				w.logger.Debugf("available balance on spot: %s %s", baseBalance.Available, base)
				sliceQty = fixedpoint.Min(sliceQty, baseBalance.Available)
			}
		case types.SideTypeBuy:
			// check available quote for buy
			quote := w.Market().QuoteCurrency
			if quoteBalance, ok := w.account.Balance(quote); !price.IsZero() && ok {
				w.logger.Debugf("available balance on spot: %s %s", quoteBalance.Available, quote)
				// calculate the max quantity we can buy with the available quote balance
				maxBuyQty := quoteBalance.Available.Div(price)
				sliceQty = fixedpoint.Min(sliceQty, maxBuyQty)
			}
		}
	}
	w.logger.Debugf("sliceQty: %s@%s", sliceQty, price)
	return sliceQty
}

// shouldUpdateActiveOrder checks whether the active order should be canceled and replaced
// with a better price. For taker orders (IOC), always update. For maker orders,
// compare the current order price against the best computed maker price.
func (w *TWAPWorker) shouldUpdateActiveOrder(orderBook types.OrderBook) bool {
	if w.activeOrder == nil {
		return false
	}

	// taker orders are IOC — always refresh
	if w.syncState.Config.OrderType == TWAPOrderTypeTaker {
		return true
	}

	newPrice, err := w.syncState.TWAPExecutor.GetPrice(w.activeOrder.Side, orderBook)
	if err != nil {
		w.logger.WithError(err).Warn("[TWAP shouldUpdateOrder] failed to get price for order update check")
		return false
	}

	remaining := w.activeOrder.GetRemainingQuantity()
	if w.Market().IsDustQuantity(remaining, newPrice) {
		w.logger.Debugf("[TWAP shouldUpdateOrder] remaining quantity of active order is dust, should not update order: %s@%s", remaining, newPrice)
		return false
	}

	// remaining is not dust but canceled
	if w.activeOrder.Status == types.OrderStatusCanceled {
		w.logger.Debugf("[TWAP shouldUpdateOrder] non-dust active order is canceled: %s", w.activeOrder)
		return true
	}

	newPriceBtter := false
	switch w.activeOrder.Side {
	case types.SideTypeBuy:
		newPriceBtter = newPrice.Compare(w.activeOrder.Price) < 0
	case types.SideTypeSell:
		newPriceBtter = newPrice.Compare(w.activeOrder.Price) > 0
	}
	w.logger.Debugf("[TWAP shouldUpdateOrder] order update check: current price=%s, new price=%s, better=%t",
		w.activeOrder.Price.String(), newPrice.String(), newPriceBtter)
	return newPriceBtter
}

func orderSide(remaining fixedpoint.Value) types.SideType {
	if remaining.Sign() > 0 {
		return types.SideTypeBuy
	}
	return types.SideTypeSell
}
