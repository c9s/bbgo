package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestSortTradesAscending(t *testing.T) {
	var trades = []Trade{
		{
			ID:      1,
			Symbol:  "BTCUSDT",
			Side:    SideTypeBuy,
			IsBuyer: false,
			IsMaker: false,
			Time:    Time(time.Unix(2000, 0)),
		},
		{
			ID:      2,
			Symbol:  "BTCUSDT",
			Side:    SideTypeBuy,
			IsBuyer: false,
			IsMaker: false,
			Time:    Time(time.Unix(1000, 0)),
		},
	}
	trades = SortTradesAscending(trades)
	assert.True(t, trades[0].Time.Before(trades[1].Time.Time()))
}

func getOrderPrices(orders []Order) (prices fixedpoint.Slice) {
	for _, o := range orders {
		prices = append(prices, o.Price)
	}

	return prices
}

func TestSortOrdersByPrice(t *testing.T) {

	t.Run("ascending", func(t *testing.T) {
		orders := []Order{
			{SubmitOrder: SubmitOrder{Price: number("10.0")}},
			{SubmitOrder: SubmitOrder{Price: number("30.0")}},
			{SubmitOrder: SubmitOrder{Price: number("20.0")}},
			{SubmitOrder: SubmitOrder{Price: number("25.0")}},
			{SubmitOrder: SubmitOrder{Price: number("15.0")}},
		}
		orders = SortOrdersByPrice(orders, false)
		prices := getOrderPrices(orders)
		assert.Equal(t, fixedpoint.Slice{
			number(10.0),
			number(15.0),
			number(20.0),
			number(25.0),
			number(30.0),
		}, prices)
	})

	t.Run("descending", func(t *testing.T) {
		orders := []Order{
			{SubmitOrder: SubmitOrder{Price: number("10.0")}},
			{SubmitOrder: SubmitOrder{Price: number("30.0")}},
			{SubmitOrder: SubmitOrder{Price: number("20.0")}},
			{SubmitOrder: SubmitOrder{Price: number("25.0")}},
			{SubmitOrder: SubmitOrder{Price: number("15.0")}},
		}
		orders = SortOrdersByPrice(orders, true)
		prices := getOrderPrices(orders)
		assert.Equal(t, fixedpoint.Slice{
			number(30.0),
			number(25.0),
			number(20.0),
			number(15.0),
			number(10.0),
		}, prices)
	})
}
