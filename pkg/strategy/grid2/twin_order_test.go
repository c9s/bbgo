package grid2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestTwinOrderBook(t *testing.T) {
	assert := assert.New(t)
	pins := []Pin{
		Pin(fixedpoint.NewFromInt(3)),
		Pin(fixedpoint.NewFromInt(4)),
		Pin(fixedpoint.NewFromInt(1)),
		Pin(fixedpoint.NewFromInt(5)),
		Pin(fixedpoint.NewFromInt(2)),
	}

	book := newTwinOrderBook(pins)
	assert.Equal(0, book.Size())
	assert.Equal(4, book.EmptyTwinOrderSize())
	for _, pin := range pins {
		twinOrder := book.GetTwinOrder(fixedpoint.Value(pin))
		if fixedpoint.NewFromInt(1) == fixedpoint.Value(pin) {
			assert.Nil(twinOrder)
			continue
		}

		if !assert.NotNil(twinOrder) {
			continue
		}

		assert.False(twinOrder.Exist())
	}

	orders := []types.Order{
		{
			OrderID: 1,
			SubmitOrder: types.SubmitOrder{
				Price: fixedpoint.NewFromInt(2),
				Side:  types.SideTypeBuy,
			},
		},
		{
			OrderID: 2,
			SubmitOrder: types.SubmitOrder{
				Price: fixedpoint.NewFromInt(4),
				Side:  types.SideTypeSell,
			},
		},
	}

	for _, order := range orders {
		assert.NoError(book.AddOrder(order))
	}
	assert.Equal(2, book.Size())
	assert.Equal(2, book.EmptyTwinOrderSize())

	for _, order := range orders {
		pin, err := book.GetTwinOrderPin(order)
		if !assert.NoError(err) {
			continue
		}
		twinOrder := book.GetTwinOrder(pin)
		if !assert.True(twinOrder.Exist()) {
			continue
		}

		assert.Equal(order.OrderID, twinOrder.GetOrder().OrderID)
	}
}
