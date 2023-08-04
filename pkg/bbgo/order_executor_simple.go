package bbgo

import (
	"context"

	log "github.com/sirupsen/logrus"
	"go.uber.org/multierr"

	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/types"
)

// SimpleOrderExecutor implements the minimal order executor
// This order executor does not handle position and profit stats update
type SimpleOrderExecutor struct {
	BaseOrderExecutor

	logger log.FieldLogger
}

func NewSimpleOrderExecutor(session *ExchangeSession) *SimpleOrderExecutor {
	return &SimpleOrderExecutor{
		BaseOrderExecutor: BaseOrderExecutor{
			session:           session,
			activeMakerOrders: NewActiveOrderBook(""),
			orderStore:        core.NewOrderStore(""),
		},
	}
}

func (e *SimpleOrderExecutor) SubmitOrders(ctx context.Context, submitOrders ...types.SubmitOrder) (types.OrderSlice, error) {
	formattedOrders, err := e.session.FormatOrders(submitOrders)
	if err != nil {
		return nil, err
	}

	orderCreateCallback := func(createdOrder types.Order) {
		e.orderStore.Add(createdOrder)
		e.activeMakerOrders.Add(createdOrder)
	}

	createdOrders, _, err := BatchPlaceOrder(ctx, e.session.Exchange, orderCreateCallback, formattedOrders...)
	return createdOrders, err
}

// CancelOrders cancels the given order objects directly
func (e *SimpleOrderExecutor) CancelOrders(ctx context.Context, orders ...types.Order) error {
	if len(orders) == 0 {
		orders = e.activeMakerOrders.Orders()
	}

	if len(orders) == 0 {
		return nil
	}

	err := e.session.Exchange.CancelOrders(ctx, orders...)
	if err != nil { // Retry once
		err2 := e.session.Exchange.CancelOrders(ctx, orders...)
		if err2 != nil {
			return multierr.Append(err, err2)
		}
	}

	return err
}

func (e *SimpleOrderExecutor) Bind() {
	e.activeMakerOrders.BindStream(e.session.UserDataStream)
	e.orderStore.BindStream(e.session.UserDataStream)
}
