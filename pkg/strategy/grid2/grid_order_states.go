package grid2

import (
	"context"
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"go.uber.org/multierr"
)

type GridOrder struct {
	OrderID       uint64           `json:"orderID"`
	ClientOrderID string           `json:"clientOrderID"`
	Side          types.SideType   `json:"side"`
	Price         fixedpoint.Value `json:"price"`
	Quantity      fixedpoint.Value `json:"quantity"`
}

type GridOrderStates struct {
	Orders map[fixedpoint.Value]GridOrder `json:"orders"`
}

func newGridOrderStates() *GridOrderStates {
	return &GridOrderStates{
		Orders: make(map[fixedpoint.Value]GridOrder),
	}
}

func (s *GridOrderStates) AddCreatedOrders(createdOrders ...types.Order) {
	for _, createdOrder := range createdOrders {
		s.Orders[createdOrder.Price] = GridOrder{
			OrderID:       createdOrder.OrderID,
			ClientOrderID: createdOrder.ClientOrderID,
			Side:          createdOrder.Side,
			Price:         createdOrder.Price,
			Quantity:      createdOrder.Quantity,
		}
	}
}

func (s *GridOrderStates) AddSubmitOrders(submitOrders ...types.SubmitOrder) {
	for _, submitOrder := range submitOrders {
		s.Orders[submitOrder.Price] = GridOrder{
			ClientOrderID: submitOrder.ClientOrderID,
			Side:          submitOrder.Side,
			Price:         submitOrder.Price,
			Quantity:      submitOrder.Quantity,
		}
	}
}

func (s *GridOrderStates) GetFailedOrdersWhenGridOpening(ctx context.Context, orderQueryService types.ExchangeOrderQueryService) ([]GridOrder, error) {
	var failed []GridOrder
	var errs error

	for _, order := range s.Orders {
		if order.OrderID == 0 {
			_, err := orderQueryService.QueryOrder(ctx, types.OrderQuery{
				ClientOrderID: order.ClientOrderID,
			})

			if err != nil {
				// error handle
				if strings.Contains(err.Error(), "resource not found") {
					// not found error, need to re-place this order
					// if order not found: {"success":false,"error":{"code":404,"message":"resource not found"}}
					failed = append(failed, order)
				} else {
					// other error
					// need to log the error and stop
					errs = multierr.Append(errs, err)
				}

				continue
			}
		}
	}

	return failed, errs
}
