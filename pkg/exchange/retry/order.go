package retry

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/cenkalti/backoff/v4"

	"github.com/c9s/bbgo/pkg/exchange/max"
	maxapi "github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/types"
)

var ErrOrderIsNil = errors.New("order object is nil")

type advancedOrderCancelService interface {
	CancelAllOrders(ctx context.Context) ([]types.Order, error)
	CancelOrdersBySymbol(ctx context.Context, symbol string) ([]types.Order, error)
	CancelOrdersByGroupID(ctx context.Context, groupID uint32) ([]types.Order, error)
}

func QueryOrderUntilCanceled(
	ctx context.Context, queryOrderService types.ExchangeOrderQueryService, symbol string, orderId uint64,
) (o *types.Order, err error) {
	var op = func() (err2 error) {
		o, err2 = queryOrderService.QueryOrder(ctx, types.OrderQuery{
			Symbol:  symbol,
			OrderID: strconv.FormatUint(orderId, 10),
		})

		if err2 != nil {
			return err2
		}

		if o == nil {
			return fmt.Errorf("order #%d response is nil", orderId)
		}

		if o.Status == types.OrderStatusCanceled || o.Status == types.OrderStatusFilled {
			return nil
		}

		return fmt.Errorf("order #%d is not canceled yet: %s", o.OrderID, o.Status)
	}

	err = GeneralBackoff(ctx, op)
	return o, err
}

func QueryOrderUntilFilled(
	ctx context.Context, queryOrderService types.ExchangeOrderQueryService, symbol string, orderId uint64,
) (o *types.Order, err error) {
	_, isMax := queryOrderService.(*max.Exchange)
	var op = func() (err2 error) {
		o, err2 = queryOrderService.QueryOrder(ctx, types.OrderQuery{
			Symbol:  symbol,
			OrderID: strconv.FormatUint(orderId, 10),
		})

		if err2 != nil {
			return err2
		}

		if o == nil {
			return ErrOrderIsNil
		}

		// for final status return nil error to stop the retry
		switch o.Status {
		case types.OrderStatusFilled:
			if isMax {
				// for MAX exchange, the order state done is filled but finalizing is not filled
				if o.OriginalStatus == string(maxapi.OrderStateDone) {
					return nil
				}
			} else {
				return nil
			}
		case types.OrderStatusCanceled:
			return nil
		}

		return fmt.Errorf("order is not filled yet: status=%s E/Q=%s/%s", o.Status, o.ExecutedQuantity.String(), o.Quantity.String())
	}

	err = GeneralBackoff(ctx, op)
	return o, err
}

func GeneralBackoff(ctx context.Context, op backoff.Operation) (err error) {
	err = backoff.Retry(op, backoff.WithContext(
		backoff.WithMaxRetries(
			backoff.NewExponentialBackOff(),
			101),
		ctx))
	return err
}

func GeneralLiteBackoff(ctx context.Context, op backoff.Operation) (err error) {
	err = backoff.Retry(op, backoff.WithContext(
		backoff.WithMaxRetries(
			backoff.NewExponentialBackOff(),
			5),
		ctx))
	return err
}

func QueryOpenOrdersUntilSuccessful(
	ctx context.Context, ex types.Exchange, symbol string,
) (openOrders []types.Order, err error) {
	var op = func() (err2 error) {
		openOrders, err2 = ex.QueryOpenOrders(ctx, symbol)
		return err2
	}

	err = GeneralBackoff(ctx, op)
	return openOrders, err
}

func QueryOpenOrdersUntilSuccessfulLite(
	ctx context.Context, ex types.Exchange, symbol string,
) (openOrders []types.Order, err error) {
	var op = func() (err2 error) {
		openOrders, err2 = ex.QueryOpenOrders(ctx, symbol)
		return err2
	}

	err = GeneralLiteBackoff(ctx, op)
	return openOrders, err
}

func QueryClosedOrdersUntilSuccessful(
	ctx context.Context, ex types.ExchangeTradeHistoryService, symbol string, since, until time.Time, lastOrderID uint64,
) (closedOrders []types.Order, err error) {
	var op = func() (err2 error) {
		closedOrders, err2 = ex.QueryClosedOrders(ctx, symbol, since, until, lastOrderID)
		return err2
	}

	err = GeneralBackoff(ctx, op)
	return closedOrders, err
}

func QueryClosedOrdersUntilSuccessfulLite(
	ctx context.Context, ex types.ExchangeTradeHistoryService, symbol string, since, until time.Time, lastOrderID uint64,
) (closedOrders []types.Order, err error) {
	var op = func() (err2 error) {
		closedOrders, err2 = ex.QueryClosedOrders(ctx, symbol, since, until, lastOrderID)
		return err2
	}

	err = GeneralLiteBackoff(ctx, op)
	return closedOrders, err
}

func QueryOrderTradesUntilSuccessful(
	ctx context.Context, ex types.ExchangeOrderQueryService, q types.OrderQuery,
) (trades []types.Trade, err error) {
	var op = func() (err2 error) {
		trades, err2 = ex.QueryOrderTrades(ctx, q)
		return err2
	}

	err = GeneralBackoff(ctx, op)
	return trades, err
}

func QueryOrderTradesUntilSuccessfulLite(
	ctx context.Context, ex types.ExchangeOrderQueryService, q types.OrderQuery,
) (trades []types.Trade, err error) {
	var op = func() (err2 error) {
		trades, err2 = ex.QueryOrderTrades(ctx, q)
		return err2
	}

	err = GeneralLiteBackoff(ctx, op)
	return trades, err
}

func QueryAccountUntilSuccessful(
	ctx context.Context, ex types.ExchangeAccountService,
) (account *types.Account, err error) {
	var op = func() (err2 error) {
		account, err2 = ex.QueryAccount(ctx)
		return err2
	}

	err = GeneralBackoff(ctx, op)
	return account, err
}

func QueryOrderUntilSuccessful(
	ctx context.Context, query types.ExchangeOrderQueryService, opts types.OrderQuery,
) (order *types.Order, err error) {
	var op = func() (err2 error) {
		order, err2 = query.QueryOrder(ctx, opts)
		return err2
	}

	err = GeneralBackoff(ctx, op)
	return order, err
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
