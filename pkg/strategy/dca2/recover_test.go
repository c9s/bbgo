package dca2

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
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

func Test_RecoverState(t *testing.T) {
	strategy := newTestStrategy()

	t.Run("new strategy", func(t *testing.T) {
		currentRound := Round{}
		position := types.NewPositionFromMarket(strategy.Market)
		orderExecutor := bbgo.NewGeneralOrderExecutor(&bbgo.ExchangeSession{}, strategy.Symbol, ID, "", position)
		state, err := recoverState(context.Background(), 5, currentRound, orderExecutor)
		assert.NoError(t, err)
		assert.Equal(t, WaitToOpenPosition, state)
	})

	t.Run("at open position stage and no filled order", func(t *testing.T) {
		now := time.Now()
		currentRound := Round{
			OpenPositionOrders: []types.Order{
				generateTestOrder(types.SideTypeBuy, types.OrderStatusPartiallyFilled, now.Add(-1*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusNew, now.Add(-2*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusNew, now.Add(-3*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusNew, now.Add(-4*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusNew, now.Add(-5*time.Second)),
			},
		}
		position := types.NewPositionFromMarket(strategy.Market)
		orderExecutor := bbgo.NewGeneralOrderExecutor(&bbgo.ExchangeSession{}, strategy.Symbol, ID, "", position)
		state, err := recoverState(context.Background(), 5, currentRound, orderExecutor)
		assert.NoError(t, err)
		assert.Equal(t, OpenPositionReady, state)
	})

	t.Run("at open position stage and there at least one filled order", func(t *testing.T) {
		now := time.Now()
		currentRound := Round{
			OpenPositionOrders: []types.Order{
				generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-1*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusNew, now.Add(-2*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusNew, now.Add(-3*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusNew, now.Add(-4*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusNew, now.Add(-5*time.Second)),
			},
		}
		position := types.NewPositionFromMarket(strategy.Market)
		orderExecutor := bbgo.NewGeneralOrderExecutor(&bbgo.ExchangeSession{}, strategy.Symbol, ID, "", position)
		state, err := recoverState(context.Background(), 5, currentRound, orderExecutor)
		assert.NoError(t, err)
		assert.Equal(t, OpenPositionOrderFilled, state)
	})

	t.Run("open position stage finish, but stop at cancelling", func(t *testing.T) {
		now := time.Now()
		currentRound := Round{
			OpenPositionOrders: []types.Order{
				generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-1*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-2*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-3*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-4*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusNew, now.Add(-5*time.Second)),
			},
		}
		position := types.NewPositionFromMarket(strategy.Market)
		orderExecutor := bbgo.NewGeneralOrderExecutor(&bbgo.ExchangeSession{}, strategy.Symbol, ID, "", position)
		state, err := recoverState(context.Background(), 5, currentRound, orderExecutor)
		assert.NoError(t, err)
		assert.Equal(t, OpenPositionOrdersCancelling, state)
	})

	t.Run("open-position orders are cancelled", func(t *testing.T) {
		now := time.Now()
		currentRound := Round{
			OpenPositionOrders: []types.Order{
				generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-1*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-2*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-3*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-4*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-5*time.Second)),
			},
		}
		position := types.NewPositionFromMarket(strategy.Market)
		orderExecutor := bbgo.NewGeneralOrderExecutor(&bbgo.ExchangeSession{}, strategy.Symbol, ID, "", position)
		state, err := recoverState(context.Background(), 5, currentRound, orderExecutor)
		assert.NoError(t, err)
		assert.Equal(t, OpenPositionOrdersCancelling, state)
	})

	t.Run("at take profit stage, and not filled yet", func(t *testing.T) {
		now := time.Now()
		currentRound := Round{
			TakeProfitOrders: []types.Order{
				generateTestOrder(types.SideTypeSell, types.OrderStatusNew, now),
			},
			OpenPositionOrders: []types.Order{
				generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-1*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-2*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-3*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-4*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-5*time.Second)),
			},
		}
		position := types.NewPositionFromMarket(strategy.Market)
		orderExecutor := bbgo.NewGeneralOrderExecutor(&bbgo.ExchangeSession{}, strategy.Symbol, ID, "", position)
		state, err := recoverState(context.Background(), 5, currentRound, orderExecutor)
		assert.NoError(t, err)
		assert.Equal(t, TakeProfitReady, state)
	})

	t.Run("at take profit stage, take-profit order filled", func(t *testing.T) {
		now := time.Now()
		currentRound := Round{
			TakeProfitOrders: []types.Order{
				generateTestOrder(types.SideTypeSell, types.OrderStatusFilled, now),
			},
			OpenPositionOrders: []types.Order{
				generateTestOrder(types.SideTypeBuy, types.OrderStatusFilled, now.Add(-1*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-2*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-3*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-4*time.Second)),
				generateTestOrder(types.SideTypeBuy, types.OrderStatusCanceled, now.Add(-5*time.Second)),
			},
		}
		position := types.NewPositionFromMarket(strategy.Market)
		orderExecutor := bbgo.NewGeneralOrderExecutor(&bbgo.ExchangeSession{}, strategy.Symbol, ID, "", position)
		state, err := recoverState(context.Background(), 5, currentRound, orderExecutor)
		assert.NoError(t, err)
		assert.Equal(t, WaitToOpenPosition, state)
	})
}

func Test_classifyOrders(t *testing.T) {
	orders := []types.Order{
		types.Order{Status: types.OrderStatusCanceled},
		types.Order{Status: types.OrderStatusFilled},
		types.Order{Status: types.OrderStatusCanceled},
		types.Order{Status: types.OrderStatusFilled},
		types.Order{Status: types.OrderStatusPartiallyFilled},
		types.Order{Status: types.OrderStatusCanceled},
		types.Order{Status: types.OrderStatusPartiallyFilled},
		types.Order{Status: types.OrderStatusNew},
		types.Order{Status: types.OrderStatusRejected},
		types.Order{Status: types.OrderStatusCanceled},
	}

	opened, cancelled, filled, unexpected := classifyOrders(orders)
	assert.Equal(t, 3, len(opened))
	assert.Equal(t, 4, len(cancelled))
	assert.Equal(t, 2, len(filled))
	assert.Equal(t, 1, len(unexpected))
}
