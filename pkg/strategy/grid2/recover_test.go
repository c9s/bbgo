package grid2

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func newStrategy(t *TestData) *Strategy {
	s := t.Strategy
	s.Debug = true
	s.Initialize()
	s.Market = t.Market
	s.Position = types.NewPositionFromMarket(t.Market)
	s.orderExecutor = bbgo.NewGeneralOrderExecutor(&bbgo.ExchangeSession{}, t.Market.Symbol, ID, s.InstanceID(), s.Position)
	return &s
}

func TestBuildTwinOrderBook(t *testing.T) {
	assert := assert.New(t)

	pins := []Pin{
		Pin(fixedpoint.NewFromInt(200)),
		Pin(fixedpoint.NewFromInt(300)),
		Pin(fixedpoint.NewFromInt(500)),
		Pin(fixedpoint.NewFromInt(400)),
		Pin(fixedpoint.NewFromInt(100)),
	}
	t.Run("build twin orderbook with no order", func(t *testing.T) {
		b, err := buildTwinOrderBook(pins, nil)
		if !assert.NoError(err) {
			return
		}

		assert.Equal(0, b.Size())
		assert.Nil(b.GetTwinOrder(fixedpoint.NewFromInt(100)))
		assert.False(b.GetTwinOrder(fixedpoint.NewFromInt(200)).Exist())
		assert.False(b.GetTwinOrder(fixedpoint.NewFromInt(300)).Exist())
		assert.False(b.GetTwinOrder(fixedpoint.NewFromInt(400)).Exist())
		assert.False(b.GetTwinOrder(fixedpoint.NewFromInt(500)).Exist())
	})

	t.Run("build twin orderbook with some valid orders", func(t *testing.T) {
		orders := []types.Order{
			{
				OrderID: 1,
				SubmitOrder: types.SubmitOrder{
					Side:  types.SideTypeBuy,
					Price: fixedpoint.NewFromInt(100),
				},
			},
			{
				OrderID: 5,
				SubmitOrder: types.SubmitOrder{
					Side:  types.SideTypeSell,
					Price: fixedpoint.NewFromInt(500),
				},
			},
		}
		b, err := buildTwinOrderBook(pins, orders)
		if !assert.NoError(err) {
			return
		}

		assert.Equal(2, b.Size())
		assert.Equal(2, b.EmptyTwinOrderSize())
		assert.Nil(b.GetTwinOrder(fixedpoint.NewFromInt(100)))
		assert.True(b.GetTwinOrder(fixedpoint.NewFromInt(200)).Exist())
		assert.False(b.GetTwinOrder(fixedpoint.NewFromInt(300)).Exist())
		assert.False(b.GetTwinOrder(fixedpoint.NewFromInt(400)).Exist())
		assert.True(b.GetTwinOrder(fixedpoint.NewFromInt(500)).Exist())
	})

	t.Run("build twin orderbook with invalid orders", func(t *testing.T) {})
}

func TestSyncActiveOrder(t *testing.T) {
	assert := assert.New(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	symbol := "ETHUSDT"

	t.Run("sync filled order in active orderbook, active orderbook should remove this order", func(t *testing.T) {
		mockOrderQueryService := mocks.NewMockExchangeOrderQueryService(mockCtrl)
		activeOrderbook := bbgo.NewActiveOrderBook(symbol)

		order := types.Order{
			OrderID: 1,
			Status:  types.OrderStatusNew,
			SubmitOrder: types.SubmitOrder{
				Symbol: symbol,
			},
		}
		activeOrderbook.Add(order)

		updatedOrder := order
		updatedOrder.Status = types.OrderStatusFilled

		mockOrderQueryService.EXPECT().QueryOrder(ctx, types.OrderQuery{
			Symbol:  symbol,
			OrderID: strconv.FormatUint(order.OrderID, 10),
		}).Return(&updatedOrder, nil)

		_, err := syncActiveOrder(ctx, activeOrderbook, mockOrderQueryService, order.OrderID, time.Now())
		if !assert.NoError(err) {
			return
		}

		// verify active orderbook
		activeOrders := activeOrderbook.Orders()
		assert.Equal(0, len(activeOrders))
	})

	t.Run("sync partial-filled order in active orderbook, active orderbook should still keep this order", func(t *testing.T) {
		mockOrderQueryService := mocks.NewMockExchangeOrderQueryService(mockCtrl)
		activeOrderbook := bbgo.NewActiveOrderBook(symbol)

		order := types.Order{
			OrderID: 1,
			Status:  types.OrderStatusNew,
			SubmitOrder: types.SubmitOrder{
				Symbol: symbol,
			},
		}
		activeOrderbook.Add(order)

		updatedOrder := order
		updatedOrder.Status = types.OrderStatusPartiallyFilled

		mockOrderQueryService.EXPECT().QueryOrder(ctx, types.OrderQuery{
			Symbol:  symbol,
			OrderID: strconv.FormatUint(order.OrderID, 10),
		}).Return(&updatedOrder, nil)

		_, err := syncActiveOrder(ctx, activeOrderbook, mockOrderQueryService, order.OrderID, time.Now())
		if !assert.NoError(err) {
			return
		}

		// verify active orderbook
		activeOrders := activeOrderbook.Orders()
		assert.Equal(1, len(activeOrders))
		assert.Equal(order.OrderID, activeOrders[0].OrderID)
		assert.Equal(updatedOrder.Status, activeOrders[0].Status)
	})
}

func TestQueryTradesToUpdateTwinOrderBook(t *testing.T) {
	assert := assert.New(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	symbol := "ETHUSDT"
	pins := []Pin{
		Pin(fixedpoint.NewFromInt(100)),
		Pin(fixedpoint.NewFromInt(200)),
		Pin(fixedpoint.NewFromInt(300)),
		Pin(fixedpoint.NewFromInt(400)),
		Pin(fixedpoint.NewFromInt(500)),
	}

	t.Run("query trades and update twin orderbook successfully in one page", func(t *testing.T) {
		book := newTwinOrderBook(pins)
		mockTradeHistoryService := mocks.NewMockExchangeTradeHistoryService(mockCtrl)
		mockOrderQueryService := mocks.NewMockExchangeOrderQueryService(mockCtrl)

		trades := []types.Trade{
			{
				ID:      1,
				OrderID: 1,
				Symbol:  symbol,
				Time:    types.Time(time.Now().Add(-2 * time.Hour)),
			},
			{
				ID:      2,
				OrderID: 2,
				Symbol:  symbol,
				Time:    types.Time(time.Now().Add(-1 * time.Hour)),
			},
		}
		orders := []types.Order{
			{
				OrderID: 1,
				Status:  types.OrderStatusNew,
				SubmitOrder: types.SubmitOrder{
					Symbol: symbol,
					Side:   types.SideTypeBuy,
					Price:  fixedpoint.NewFromInt(100),
				},
			},
			{
				OrderID: 2,
				Status:  types.OrderStatusFilled,
				SubmitOrder: types.SubmitOrder{
					Symbol: symbol,
					Side:   types.SideTypeSell,
					Price:  fixedpoint.NewFromInt(500),
				},
			},
		}
		mockTradeHistoryService.EXPECT().QueryTrades(gomock.Any(), gomock.Any(), gomock.Any()).Return(trades, nil).Times(1)
		mockOrderQueryService.EXPECT().QueryOrder(gomock.Any(), types.OrderQuery{
			Symbol:  symbol,
			OrderID: "1",
		}).Return(&orders[0], nil)
		mockOrderQueryService.EXPECT().QueryOrder(gomock.Any(), types.OrderQuery{
			Symbol:  symbol,
			OrderID: "2",
		}).Return(&orders[1], nil)

		assert.Equal(0, book.Size())
		if !assert.NoError(queryTradesToUpdateTwinOrderBook(ctx, symbol, book, mockTradeHistoryService, mockOrderQueryService, book.SyncOrderMap(), time.Now().Add(-24*time.Hour), time.Now(), nil)) {
			return
		}

		assert.Equal(2, book.Size())
		assert.True(book.GetTwinOrder(fixedpoint.NewFromInt(200)).Exist())
		assert.Equal(orders[0].OrderID, book.GetTwinOrder(fixedpoint.NewFromInt(200)).GetOrder().OrderID)
		assert.True(book.GetTwinOrder(fixedpoint.NewFromInt(500)).Exist())
		assert.Equal(orders[1].OrderID, book.GetTwinOrder(fixedpoint.NewFromInt(500)).GetOrder().OrderID)
	})
}
