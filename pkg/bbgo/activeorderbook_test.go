package bbgo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

func TestActiveOrderBook_pendingOrders(t *testing.T) {
	now := time.Now()
	t1 := now
	t2 := now.Add(time.Millisecond)

	ob := NewActiveOrderBook("BTCUSDT")

	filled := false
	ob.OnFilled(func(o types.Order) {
		filled = true
	})

	quantity := Number("0.01")
	orderUpdate1 := types.Order{
		OrderID: 99,
		SubmitOrder: types.SubmitOrder{
			Symbol:       "BTCUSDT",
			Side:         types.SideTypeBuy,
			Type:         types.OrderTypeLimit,
			Quantity:     quantity,
			Price:        Number(19000.0),
			AveragePrice: fixedpoint.Zero,
			StopPrice:    fixedpoint.Zero,
		},
		ExecutedQuantity: Number(0.0),
		Status:           types.OrderStatusNew,
		CreationTime:     types.Time(t1),
		UpdateTime:       types.Time(t1),
	}

	orderUpdate2 := types.Order{
		OrderID: 99,
		SubmitOrder: types.SubmitOrder{
			Symbol:       "BTCUSDT",
			Side:         types.SideTypeBuy,
			Type:         types.OrderTypeLimit,
			Quantity:     quantity,
			Price:        Number(19000.0),
			AveragePrice: fixedpoint.Zero,
			StopPrice:    fixedpoint.Zero,
		},
		ExecutedQuantity: quantity,
		Status:           types.OrderStatusFilled,
		CreationTime:     types.Time(t1),
		UpdateTime:       types.Time(t2),
	}

	assert.True(t, isNewerOrderUpdate(orderUpdate2, orderUpdate1), "orderUpdate2 should be newer than orderUpdate1")

	// if we received filled order first
	// should be added to pending orders
	ob.Update(orderUpdate2)
	assert.Len(t, ob.pendingOrderUpdates.Orders(), 1)

	o99, ok := ob.pendingOrderUpdates.Get(99)
	if assert.True(t, ok) {
		assert.Equal(t, types.OrderStatusFilled, o99.Status)
	}

	// when adding the older order update to the book,
	// it should trigger the filled event once the order is registered to the active order book
	ob.Add(orderUpdate1)
	assert.True(t, filled, "filled event should be fired")
}

func Test_isNewerUpdate(t *testing.T) {
	a := types.Order{
		Status:           types.OrderStatusPartiallyFilled,
		ExecutedQuantity: number(0.2),
	}
	b := types.Order{
		Status:           types.OrderStatusPartiallyFilled,
		ExecutedQuantity: number(0.1),
	}
	ret := isNewerOrderUpdate(a, b)
	assert.True(t, ret)
}

func Test_isNewerUpdateTime(t *testing.T) {
	a := types.Order{
		UpdateTime: types.NewTimeFromUnix(200, 0),
	}
	b := types.Order{
		UpdateTime: types.NewTimeFromUnix(100, 0),
	}
	ret := isNewerOrderUpdateTime(a, b)
	assert.True(t, ret)
}
