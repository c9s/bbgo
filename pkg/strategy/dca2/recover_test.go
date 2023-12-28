package dca2

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func generateTestOrder(side types.SideType, status types.OrderStatus, createdAt time.Time) types.Order {
	return types.Order{
		OrderID: rand.Uint64(),
		SubmitOrder: types.SubmitOrder{
			Side: side,
		},
		Status:       status,
		CreationTime: types.Time(createdAt),
	}

}

func Test_GetCurrenctAndLastRoundOrders(t *testing.T) {
	t.Run("case 1", func(t *testing.T) {
		now := time.Now()
		openOrders := []types.Order{
			generateTestOrder(types.SideTypeSell, types.OrderStatusNew, now),
		}

		closedOrders := []types.Order{
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-1*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-2*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-3*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-4*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-5*time.Second)),
		}

		currentRound, err := getCurrentRoundOrders(openOrders, closedOrders, 0)

		assert.NoError(t, err)
		assert.NotEqual(t, 0, currentRound.TakeProfitOrder.OrderID)
		assert.Equal(t, 5, len(currentRound.OpenPositionOrders))
	})

	t.Run("case 2", func(t *testing.T) {
		now := time.Now()
		openOrders := []types.Order{
			generateTestOrder(types.SideTypeSell, types.OrderStatusNew, now),
		}

		closedOrders := []types.Order{
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-1*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-2*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-3*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-4*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-5*time.Second)),
			generateTestOrder(types.SideTypeSell, types.OrderStatusFilled, now.Add(-6*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-7*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-8*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-9*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-10*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-11*time.Second)),
			generateTestOrder(types.SideTypeSell, types.OrderStatusFilled, now.Add(-12*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-13*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-14*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-15*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-16*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-17*time.Second)),
		}

		currentRound, err := getCurrentRoundOrders(openOrders, closedOrders, 0)

		assert.NoError(t, err)
		assert.NotEqual(t, 0, currentRound.TakeProfitOrder.OrderID)
		assert.Equal(t, 5, len(currentRound.OpenPositionOrders))
	})
}

type MockQueryOrders struct {
	OpenOrders   []types.Order
	ClosedOrders []types.Order
}

func (m *MockQueryOrders) QueryOpenOrders(ctx context.Context, symbol string) ([]types.Order, error) {
	return m.OpenOrders, nil
}

func (m *MockQueryOrders) QueryClosedOrdersDesc(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) ([]types.Order, error) {
	return m.ClosedOrders, nil
}

func Test_RecoverState(t *testing.T) {
	symbol := "BTCUSDT"

	t.Run("new strategy", func(t *testing.T) {
		openOrders := []types.Order{}
		currentRound := Round{}
		activeOrderBook := bbgo.NewActiveOrderBook(symbol)
		orderStore := core.NewOrderStore(symbol)
		state, err := recoverState(context.Background(), symbol, 5, openOrders, currentRound, activeOrderBook, orderStore, 0)
		assert.NoError(t, err)
		assert.Equal(t, WaitToOpenPosition, state)
	})

	t.Run("at open position stage and no filled order", func(t *testing.T) {
		now := time.Now()
		openOrders := []types.Order{
			generateTestOrder(types.SideTypeBuy, types.OrderStatusPartiallyFilled, now.Add(-1*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusNew, now.Add(-2*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusNew, now.Add(-3*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusNew, now.Add(-4*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusNew, now.Add(-5*time.Second)),
		}
		currentRound := Round{
			OpenPositionOrders: openOrders,
		}
		orderStore := core.NewOrderStore(symbol)
		activeOrderBook := bbgo.NewActiveOrderBook(symbol)
		state, err := recoverState(context.Background(), symbol, 5, openOrders, currentRound, activeOrderBook, orderStore, 0)
		assert.NoError(t, err)
		assert.Equal(t, OpenPositionReady, state)
	})

	t.Run("at open position stage and there at least one filled order", func(t *testing.T) {
		now := time.Now()
		openOrders := []types.Order{
			generateTestOrder(types.SideTypeBuy, types.OrderStatusNew, now.Add(-2*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusNew, now.Add(-3*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusNew, now.Add(-4*time.Second)),
			generateTestOrder(types.SideTypeBuy, types.OrderStatusNew, now.Add(-5*time.Second)),
		}
		currentRound := Round{
			OpenPositionOrders: []types.Order{
				generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-1*time.Second)),
				openOrders[0],
				openOrders[1],
				openOrders[2],
				openOrders[3],
			},
		}
		orderStore := core.NewOrderStore(symbol)
		activeOrderBook := bbgo.NewActiveOrderBook(symbol)
		state, err := recoverState(context.Background(), symbol, 5, openOrders, currentRound, activeOrderBook, orderStore, 0)
		assert.NoError(t, err)
		assert.Equal(t, OpenPositionOrderFilled, state)
	})

	t.Run("open position stage finish, but stop at cancelling", func(t *testing.T) {
		now := time.Now()
		openOrders := []types.Order{
			generateTestOrder(types.SideTypeBuy, types.OrderStatusNew, now.Add(-5*time.Second)),
		}
		currentRound := Round{
			OpenPositionOrders: []types.Order{
				generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-1*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-2*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-3*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-4*time.Second)),
				openOrders[0],
			},
		}
		orderStore := core.NewOrderStore(symbol)
		activeOrderBook := bbgo.NewActiveOrderBook(symbol)
		state, err := recoverState(context.Background(), symbol, 5, openOrders, currentRound, activeOrderBook, orderStore, 0)
		assert.NoError(t, err)
		assert.Equal(t, OpenPositionOrdersCancelling, state)
	})

	t.Run("open-position orders are cancelled", func(t *testing.T) {
		now := time.Now()
		openOrders := []types.Order{}
		currentRound := Round{
			OpenPositionOrders: []types.Order{
				generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-1*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-2*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-3*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-4*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-5*time.Second)),
			},
		}
		orderStore := core.NewOrderStore(symbol)
		activeOrderBook := bbgo.NewActiveOrderBook(symbol)
		state, err := recoverState(context.Background(), symbol, 5, openOrders, currentRound, activeOrderBook, orderStore, 0)
		assert.NoError(t, err)
		assert.Equal(t, OpenPositionOrdersCancelled, state)
	})

	t.Run("at take profit stage, and not filled yet", func(t *testing.T) {
		now := time.Now()
		openOrders := []types.Order{
			generateTestOrder(types.SideTypeSell, types.OrderStatusNew, now),
		}
		currentRound := Round{
			TakeProfitOrder: openOrders[0],
			OpenPositionOrders: []types.Order{
				generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-1*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-2*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-3*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-4*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-5*time.Second)),
			},
		}
		orderStore := core.NewOrderStore(symbol)
		activeOrderBook := bbgo.NewActiveOrderBook(symbol)
		state, err := recoverState(context.Background(), symbol, 5, openOrders, currentRound, activeOrderBook, orderStore, 0)
		assert.NoError(t, err)
		assert.Equal(t, TakeProfitReady, state)
	})

	t.Run("at take profit stage, take-profit order filled", func(t *testing.T) {
		now := time.Now()
		openOrders := []types.Order{}
		currentRound := Round{
			TakeProfitOrder: generateTestOrder(types.SideTypeSell, types.OrderStatusFilled, now),
			OpenPositionOrders: []types.Order{
				generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-1*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-2*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-3*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-4*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-5*time.Second)),
			},
		}
		orderStore := core.NewOrderStore(symbol)
		activeOrderBook := bbgo.NewActiveOrderBook(symbol)
		state, err := recoverState(context.Background(), symbol, 5, openOrders, currentRound, activeOrderBook, orderStore, 0)
		assert.NoError(t, err)
		assert.Equal(t, WaitToOpenPosition, state)
	})
}
