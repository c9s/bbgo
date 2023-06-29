package retry

import (
	"context"
	"errors"
	"strconv"

	backoff2 "github.com/cenkalti/backoff/v4"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util/backoff"
)

type advancedOrderCancelService interface {
	CancelAllOrders(ctx context.Context) ([]types.Order, error)
	CancelOrdersBySymbol(ctx context.Context, symbol string) ([]types.Order, error)
	CancelOrdersByGroupID(ctx context.Context, groupID uint32) ([]types.Order, error)
}

func QueryOrderUntilFilled(ctx context.Context, queryOrderService types.ExchangeOrderQueryService, symbol string, orderId uint64) (o *types.Order, err error) {
	err = backoff.RetryGeneral(ctx, func() (err2 error) {
		o, err2 = queryOrderService.QueryOrder(ctx, types.OrderQuery{
			Symbol:  symbol,
			OrderID: strconv.FormatUint(orderId, 10),
		})

		if err2 != nil || o == nil {
			return err2
		}

		if o.Status != types.OrderStatusFilled {
			return errors.New("order is not filled yet")
		}

		return err2
	})

	return o, err
}

func GeneralBackoff(ctx context.Context, op backoff2.Operation) (err error) {
	err = backoff2.Retry(op, backoff2.WithContext(
		backoff2.WithMaxRetries(
			backoff2.NewExponentialBackOff(),
			101),
		ctx))
	return err
}

func QueryOpenOrdersUntilSuccessful(ctx context.Context, ex types.Exchange, symbol string) (openOrders []types.Order, err error) {
	var op = func() (err2 error) {
		openOrders, err2 = ex.QueryOpenOrders(ctx, symbol)
		return err2
	}

	err = GeneralBackoff(ctx, op)
	return openOrders, err
}

func CancelAllOrdersUntilSuccessful(ctx context.Context, service advancedOrderCancelService) error {
	var op = func() (err2 error) {
		_, err2 = service.CancelAllOrders(ctx)
		return err2
	}

	return GeneralBackoff(ctx, op)
}

func CancelOrdersUntilSuccessful(ctx context.Context, ex types.Exchange, orders ...types.Order) error {
	var op = func() (err2 error) {
		err2 = ex.CancelOrders(ctx, orders...)
		return err2
	}

	return GeneralBackoff(ctx, op)
}
