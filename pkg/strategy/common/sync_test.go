package common

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestSyncActiveOrders(t *testing.T) {
	assert := assert.New(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := logrus.WithField("strategy", "test")
	symbol := "ETHUSDT"
	t.Run("all open orders are match with active orderbook", func(t *testing.T) {
		mockOrderQueryService := mocks.NewMockExchangeOrderQueryService(mockCtrl)
		mockExchange := mocks.NewMockExchange(mockCtrl)
		activeOrderbook := bbgo.NewActiveOrderBook(symbol)

		order := types.Order{
			OrderID: 1,
			Status:  types.OrderStatusNew,
		}
		order.Symbol = symbol

		opts := SyncActiveOrdersOpts{
			Logger:            log,
			Exchange:          mockExchange,
			OrderQueryService: mockOrderQueryService,
			ActiveOrderBook:   activeOrderbook,
			OpenOrders:        []types.Order{order},
		}

		activeOrderbook.Add(order)

		assert.NoError(SyncActiveOrders(ctx, opts))

		// verify active orderbook
		activeOrders := activeOrderbook.Orders()
		assert.Equal(1, len(activeOrders))
		assert.Equal(uint64(1), activeOrders[0].OrderID)
		assert.Equal(types.OrderStatusNew, activeOrders[0].Status)
	})

	t.Run("there is order in active orderbook but not in open orders", func(t *testing.T) {
		mockOrderQueryService := mocks.NewMockExchangeOrderQueryService(mockCtrl)
		mockExchange := mocks.NewMockExchange(mockCtrl)
		activeOrderbook := bbgo.NewActiveOrderBook(symbol)

		order := types.Order{
			OrderID: 1,
			Status:  types.OrderStatusNew,
			SubmitOrder: types.SubmitOrder{
				Symbol: symbol,
			},
		}
		updatedOrder := order
		updatedOrder.Status = types.OrderStatusFilled

		opts := SyncActiveOrdersOpts{
			Logger:            log,
			ActiveOrderBook:   activeOrderbook,
			OrderQueryService: mockOrderQueryService,
			Exchange:          mockExchange,
			OpenOrders:        nil,
		}

		activeOrderbook.Add(order)
		mockOrderQueryService.EXPECT().QueryOrder(ctx, types.OrderQuery{
			Symbol:  symbol,
			OrderID: strconv.FormatUint(order.OrderID, 10),
		}).Return(&updatedOrder, nil)

		assert.NoError(SyncActiveOrders(ctx, opts))

		// verify active orderbook
		activeOrders := activeOrderbook.Orders()
		assert.Equal(0, len(activeOrders))
	})

	t.Run("there is order on open orders but not in active orderbook", func(t *testing.T) {
		mockOrderQueryService := mocks.NewMockExchangeOrderQueryService(mockCtrl)
		mockExchange := mocks.NewMockExchange(mockCtrl)
		activeOrderbook := bbgo.NewActiveOrderBook(symbol)

		order := types.Order{
			OrderID: 1,
			Status:  types.OrderStatusNew,
			SubmitOrder: types.SubmitOrder{
				Symbol: symbol,
			},
			CreationTime: types.Time(time.Now()),
		}

		opts := SyncActiveOrdersOpts{
			Logger:            log,
			ActiveOrderBook:   activeOrderbook,
			OrderQueryService: mockOrderQueryService,
			Exchange:          mockExchange,
			OpenOrders:        []types.Order{order},
		}
		assert.NoError(SyncActiveOrders(ctx, opts))

		// verify active orderbook
		activeOrders := activeOrderbook.Orders()
		assert.Equal(1, len(activeOrders))
		assert.Equal(uint64(1), activeOrders[0].OrderID)
		assert.Equal(types.OrderStatusNew, activeOrders[0].Status)
	})

	t.Run("there is order on open order but not in active orderbook also order in active orderbook but not on open orders", func(t *testing.T) {
		mockOrderQueryService := mocks.NewMockExchangeOrderQueryService(mockCtrl)
		mockExchange := mocks.NewMockExchange(mockCtrl)
		activeOrderbook := bbgo.NewActiveOrderBook(symbol)

		order1 := types.Order{
			OrderID: 1,
			Status:  types.OrderStatusNew,
			SubmitOrder: types.SubmitOrder{
				Symbol: symbol,
			},
		}
		updatedOrder1 := order1
		updatedOrder1.Status = types.OrderStatusFilled
		order2 := types.Order{
			OrderID: 2,
			Status:  types.OrderStatusNew,
			SubmitOrder: types.SubmitOrder{
				Symbol: symbol,
			},
		}

		opts := SyncActiveOrdersOpts{
			Logger:            log,
			ActiveOrderBook:   activeOrderbook,
			OrderQueryService: mockOrderQueryService,
			Exchange:          mockExchange,
			OpenOrders:        []types.Order{order2},
		}

		activeOrderbook.Add(order1)
		mockOrderQueryService.EXPECT().QueryOrder(ctx, types.OrderQuery{
			Symbol:  symbol,
			OrderID: strconv.FormatUint(order1.OrderID, 10),
		}).Return(&updatedOrder1, nil)

		assert.NoError(SyncActiveOrders(ctx, opts))

		// verify active orderbook
		activeOrders := activeOrderbook.Orders()
		assert.Equal(1, len(activeOrders))
		assert.Equal(uint64(2), activeOrders[0].OrderID)
		assert.Equal(types.OrderStatusNew, activeOrders[0].Status)
	})
}
