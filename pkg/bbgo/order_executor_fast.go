package bbgo

import (
	"context"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/types"
)

// FastOrderExecutor provides shorter submit order / cancel order round-trip time
// for strategies that need to response more faster, e.g. 1s kline or market trades related strategies.
type FastOrderExecutor struct {
	*GeneralOrderExecutor
}

func NewFastOrderExecutor(session *ExchangeSession, symbol, strategy, strategyInstanceID string, position *types.Position) *FastOrderExecutor {
	oe := NewGeneralOrderExecutor(session, symbol, strategy, strategyInstanceID, position)
	return &FastOrderExecutor{
		GeneralOrderExecutor: oe,
	}
}

// SubmitOrders sends []types.SubmitOrder directly to the exchange without blocking wait on the status update.
// This is a faster version of GeneralOrderExecutor.SubmitOrders(). Created orders will be consumed in newly created goroutine (in non-backteset session).
// @param ctx: golang context type.
// @param submitOrders: Lists of types.SubmitOrder to be sent to the exchange.
// @return *types.SubmitOrder: SubmitOrder with calculated quantity and price.
// @return error: Error message.
func (e *FastOrderExecutor) SubmitOrders(ctx context.Context, submitOrders ...types.SubmitOrder) (types.OrderSlice, error) {
	formattedOrders, err := e.session.FormatOrders(submitOrders)
	if err != nil {
		return nil, err
	}

	createdOrders, errIdx, err := BatchPlaceOrder(ctx, e.session.Exchange, nil, formattedOrders...)
	if len(errIdx) > 0 {
		return nil, err
	}

	if IsBackTesting {
		e.orderStore.Add(createdOrders...)
		e.activeMakerOrders.Add(createdOrders...)
		e.tradeCollector.Process()
	} else {
		go func() {
			e.orderStore.Add(createdOrders...)
			e.activeMakerOrders.Add(createdOrders...)
			e.tradeCollector.Process()
		}()
	}
	return createdOrders, err

}

// Cancel cancels all active maker orders if orders is not given, otherwise cancel the given orders
func (e *FastOrderExecutor) Cancel(ctx context.Context, orders ...types.Order) error {
	if e.activeMakerOrders.NumOfOrders() == 0 {
		return nil
	}

	if err := e.activeMakerOrders.FastCancel(ctx, e.session.Exchange, orders...); err != nil {
		return errors.Wrap(err, "fast cancel order error")
	}

	return nil
}
