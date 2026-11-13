package types

import (
	"context"
	"errors"
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/stretchr/testify/require"
)

type cancelReplaceFallbackStub struct {
	calls        []string
	cancelErr    error
	submitErr    error
	submitted    SubmitOrder
	createdOrder *Order
}

func (s *cancelReplaceFallbackStub) CancelOrders(_ context.Context, _ ...Order) error {
	s.calls = append(s.calls, "cancel")
	return s.cancelErr
}

func (s *cancelReplaceFallbackStub) SubmitOrder(_ context.Context, order SubmitOrder) (*Order, error) {
	s.calls = append(s.calls, "create")
	s.submitted = order
	return s.createdOrder, s.submitErr
}

func TestCancelReplaceByCancelAndCreateOrdersOperations(t *testing.T) {
	order := Order{
		SubmitOrder: SubmitOrder{
			Symbol:   "ETHJPY",
			Side:     SideTypeSell,
			Type:     OrderTypeLimitMaker,
			Price:    fixedpoint.MustNewFromString("300000"),
			Quantity: fixedpoint.MustNewFromString("0.001"),
		},
		OrderID: 123,
	}
	created := &Order{OrderID: 456, SubmitOrder: order.SubmitOrder}
	stub := &cancelReplaceFallbackStub{createdOrder: created}

	got, err := CancelReplaceByCancelAndCreate(context.Background(), stub, order)
	require.NoError(t, err)
	require.Same(t, created, got)
	require.Equal(t, []string{"cancel", "create"}, stub.calls)
	require.Equal(t, order.SubmitOrder, stub.submitted)
}

func TestCancelReplaceByCancelAndCreateStopsAfterCancelFailure(t *testing.T) {
	stub := &cancelReplaceFallbackStub{cancelErr: errors.New("cancel failed")}

	got, err := CancelReplaceByCancelAndCreate(context.Background(), stub, Order{OrderID: 123})
	require.Error(t, err)
	require.Nil(t, got)
	require.Equal(t, []string{"cancel"}, stub.calls)
}

func TestCancelReplaceByCancelAndCreateReturnsCreateFailure(t *testing.T) {
	stub := &cancelReplaceFallbackStub{submitErr: errors.New("create failed")}

	got, err := CancelReplaceByCancelAndCreate(context.Background(), stub, Order{OrderID: 123})
	require.Error(t, err)
	require.Nil(t, got)
	require.Equal(t, []string{"cancel", "create"}, stub.calls)
}
