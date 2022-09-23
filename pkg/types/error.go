package types

import "fmt"

type OrderError struct {
	error error
	order Order
}

func (e *OrderError) Error() string {
	return fmt.Sprintf("%s exchange: %s orderID:%d", e.error.Error(), e.order.Exchange, e.order.OrderID)
}

func (e *OrderError) Order() Order {
	return e.order
}

func NewOrderError(e error, o Order) error {
	return &OrderError{
		error: e,
		order: o,
	}
}

type ZeroAssetError struct {
	error
}

func NewZeroAssetError(e error) ZeroAssetError {
	return ZeroAssetError{
		error: e,
	}
}
