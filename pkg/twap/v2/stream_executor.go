package twap

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var defaultUpdateInterval = time.Minute

type DoneSignal struct {
	doneC chan struct{}
	mu    sync.Mutex
}

func NewDoneSignal() *DoneSignal {
	return &DoneSignal{
		doneC: make(chan struct{}),
	}
}

func (e *DoneSignal) Emit() {
	e.mu.Lock()
	if e.doneC == nil {
		e.doneC = make(chan struct{})
	}

	close(e.doneC)
	e.mu.Unlock()
}

// Chan returns a channel that emits a signal when the execution is done.
func (e *DoneSignal) Chan() (c <-chan struct{}) {
	// if the channel is not allocated, it means it's not started yet, we need to return a closed channel
	e.mu.Lock()
	if e.doneC == nil {
		e.doneC = make(chan struct{})
		c = e.doneC
	} else {
		c = e.doneC
	}
	e.mu.Unlock()

	return c
}

// FixedQuantityExecutor is a TWAP executor that places orders on the exchange using the exchange's stream API.
// It uses a fixed target quantity to place orders.
type FixedQuantityExecutor struct {
	exchange types.Exchange

	// configuration fields

	symbol         string
	side           types.SideType
	targetQuantity fixedpoint.Value

	// updateInterval is a fixed update interval for placing new order
	updateInterval time.Duration

	// delayInterval is the delay interval between each order placement
	delayInterval time.Duration

	// priceLimit is the price limit for the order
	// for buy-orders, the price limit is the maximum price
	// for sell-orders, the price limit is the minimum price
	priceLimit fixedpoint.Value

	// deadlineTime is the deadline time for the order execution
	deadlineTime *time.Time

	executionCtx    context.Context
	cancelExecution context.CancelFunc

	userDataStreamCtx    context.Context
	cancelUserDataStream context.CancelFunc

	market           types.Market
	marketDataStream types.Stream
	orderBook        *types.StreamOrderBook

	userDataStream    types.Stream
	activeMakerOrders *bbgo.ActiveOrderBook
	orderStore        *core.OrderStore
	position          *types.Position

	logger logrus.FieldLogger

	mu sync.Mutex

	done *DoneSignal
}

func NewStreamExecutor(
	exchange types.Exchange,
	symbol string,
	market types.Market,
	side types.SideType,
	targetQuantity fixedpoint.Value,
) *FixedQuantityExecutor {
	return &FixedQuantityExecutor{
		exchange:       exchange,
		symbol:         symbol,
		side:           side,
		market:         market,
		position:       types.NewPositionFromMarket(market),
		targetQuantity: targetQuantity,
		updateInterval: defaultUpdateInterval,
		logger: logrus.WithFields(logrus.Fields{
			"executor": "twapStream",
			"symbol":   symbol,
		}),

		done: NewDoneSignal(),
	}
}

func (e *FixedQuantityExecutor) SetDeadlineTime(t time.Time) {
	e.deadlineTime = &t
}

func (e *FixedQuantityExecutor) SetDelayInterval(delayInterval time.Duration) {
	e.delayInterval = delayInterval
}

func (e *FixedQuantityExecutor) SetUpdateInterval(updateInterval time.Duration) {
	e.updateInterval = updateInterval
}

func (e *FixedQuantityExecutor) connectMarketData(ctx context.Context) {
	e.logger.Infof("connecting market data stream...")
	if err := e.marketDataStream.Connect(ctx); err != nil {
		e.logger.WithError(err).Errorf("market data stream connect error")
	}
}

func (e *FixedQuantityExecutor) connectUserData(ctx context.Context) {
	e.logger.Infof("connecting user data stream...")
	if err := e.userDataStream.Connect(ctx); err != nil {
		e.logger.WithError(err).Errorf("user data stream connect error")
	}
}

func (e *FixedQuantityExecutor) handleFilledOrder(order types.Order) {
	e.logger.Info(order.String())

	// filled event triggers the order removal from the active order store
	// we need to ensure we received every order update event before the execution is done.
	e.cancelContextIfTargetQuantityFilled()
}

func (e *FixedQuantityExecutor) cancelContextIfTargetQuantityFilled() bool {
	base := e.position.GetBase()

	if base.Abs().Compare(e.targetQuantity) >= 0 {
		e.logger.Infof("filled target quantity, canceling the order execution context")
		e.cancelExecution()
		return true
	}
	return false
}

func (e *FixedQuantityExecutor) cancelActiveOrders(ctx context.Context) error {
	gracefulCtx, gracefulCancel := context.WithTimeout(ctx, 30*time.Second)
	defer gracefulCancel()
	return e.activeMakerOrders.GracefulCancel(gracefulCtx, e.exchange)
}

func (e *FixedQuantityExecutor) orderUpdater(ctx context.Context) {
	updateLimiter := rate.NewLimiter(rate.Every(3*time.Second), 1)
	_ = updateLimiter

	defer func() {
		if err := e.cancelActiveOrders(ctx); err != nil {
			e.logger.WithError(err).Error("cancel active orders error")
		}

		e.cancelUserDataStream()
		e.done.Emit()
	}()

	ticker := time.NewTimer(e.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-e.orderBook.C:
			if !updateLimiter.Allow() {
				break
			}

			if e.cancelContextIfTargetQuantityFilled() {
				return
			}

			e.logger.Infof("%s order book changed, checking order...", e.symbol)

			/*
				if err := e.updateOrder(ctx); err != nil {
					e.logger.WithError(err).Errorf("order update failed")
				}
			*/

		case <-ticker.C:
			if !updateLimiter.Allow() {
				break
			}

			if e.cancelContextIfTargetQuantityFilled() {
				return
			}

			/*
				if err := e.updateOrder(ctx); err != nil {
					e.logger.WithError(err).Errorf("order update failed")
				}
			*/
		}
	}
}

func (e *FixedQuantityExecutor) Start(ctx context.Context) error {
	if e.marketDataStream != nil {
		return errors.New("market data stream is not nil, you can't start the executor twice")
	}

	e.executionCtx, e.cancelExecution = context.WithCancel(ctx)
	e.userDataStreamCtx, e.cancelUserDataStream = context.WithCancel(ctx)

	e.marketDataStream = e.exchange.NewStream()
	e.marketDataStream.SetPublicOnly()
	e.marketDataStream.Subscribe(types.BookChannel, e.symbol, types.SubscribeOptions{
		Depth: types.DepthLevelMedium,
	})

	e.orderBook = types.NewStreamBook(e.symbol)
	e.orderBook.BindStream(e.marketDataStream)

	e.orderStore = core.NewOrderStore(e.symbol)
	e.orderStore.BindStream(e.userDataStream)
	e.activeMakerOrders = bbgo.NewActiveOrderBook(e.symbol)
	e.activeMakerOrders.OnFilled(e.handleFilledOrder)
	e.activeMakerOrders.BindStream(e.userDataStream)

	go e.connectMarketData(e.executionCtx)
	go e.connectUserData(e.userDataStreamCtx)
	go e.orderUpdater(e.executionCtx)
	return nil
}

// Done returns a channel that emits a signal when the execution is done.
func (e *FixedQuantityExecutor) Done() <-chan struct{} {
	return e.done.Chan()
}

// Shutdown stops the execution
// If we call this method, it means the execution is still running,
// We need it to:
// 1. Stop the order updater (by using the execution context)
// 2. The order updater cancels all open orders and closes the user data stream
func (e *FixedQuantityExecutor) Shutdown(shutdownCtx context.Context) {
	e.mu.Lock()
	if e.cancelExecution != nil {
		e.cancelExecution()
	}
	e.mu.Unlock()

	for {
		select {

		case <-shutdownCtx.Done():
			return

		case <-e.done.Chan():
			return

		}
	}
}
