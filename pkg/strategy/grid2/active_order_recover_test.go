package grid2

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
	"github.com/golang/mock/gomock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestSyncActiveOrders(t *testing.T) {
	assert := assert.New(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	symbol := "ETHUSDT"
	labels := prometheus.Labels{
		"exchange": "default",
		"symbol":   symbol,
	}
	t.Run("all open orders are match with active orderbook", func(t *testing.T) {
		mockOrderQueryService := mocks.NewMockExchangeOrderQueryService(mockCtrl)
		mockExchange := mocks.NewMockExchange(mockCtrl)
		activeOrderbook := bbgo.NewActiveOrderBook(symbol)

		opts := SyncActiveOrdersOpts{
			logger:            log,
			metricsLabels:     labels,
			activeOrderBook:   activeOrderbook,
			orderQueryService: mockOrderQueryService,
			exchange:          mockExchange,
		}

		order := types.Order{
			OrderID: 1,
			Status:  types.OrderStatusNew,
		}
		order.Symbol = symbol

		activeOrderbook.Add(order)
		mockExchange.EXPECT().QueryOpenOrders(ctx, symbol).Return([]types.Order{order}, nil)

		assert.NoError(syncActiveOrders(ctx, opts))

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

		opts := SyncActiveOrdersOpts{
			logger:            log,
			metricsLabels:     labels,
			activeOrderBook:   activeOrderbook,
			orderQueryService: mockOrderQueryService,
			exchange:          mockExchange,
		}

		order := types.Order{
			OrderID: 1,
			Status:  types.OrderStatusNew,
			SubmitOrder: types.SubmitOrder{
				Symbol: symbol,
			},
		}
		updatedOrder := order
		updatedOrder.Status = types.OrderStatusFilled

		activeOrderbook.Add(order)
		mockExchange.EXPECT().QueryOpenOrders(ctx, symbol).Return(nil, nil)
		mockOrderQueryService.EXPECT().QueryOrder(ctx, types.OrderQuery{
			Symbol:  symbol,
			OrderID: strconv.FormatUint(order.OrderID, 10),
		}).Return(&updatedOrder, nil)

		assert.NoError(syncActiveOrders(ctx, opts))

		// verify active orderbook
		activeOrders := activeOrderbook.Orders()
		assert.Equal(0, len(activeOrders))
	})

	t.Run("there is order on open orders but not in active orderbook", func(t *testing.T) {
		mockOrderQueryService := mocks.NewMockExchangeOrderQueryService(mockCtrl)
		mockExchange := mocks.NewMockExchange(mockCtrl)
		activeOrderbook := bbgo.NewActiveOrderBook(symbol)

		opts := SyncActiveOrdersOpts{
			logger:            log,
			metricsLabels:     labels,
			activeOrderBook:   activeOrderbook,
			orderQueryService: mockOrderQueryService,
			exchange:          mockExchange,
		}

		order := types.Order{
			OrderID: 1,
			Status:  types.OrderStatusNew,
			SubmitOrder: types.SubmitOrder{
				Symbol: symbol,
			},
			CreationTime: types.Time(time.Now()),
		}

		mockExchange.EXPECT().QueryOpenOrders(ctx, symbol).Return([]types.Order{order}, nil)
		assert.NoError(syncActiveOrders(ctx, opts))

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

		opts := SyncActiveOrdersOpts{
			logger:            log,
			metricsLabels:     labels,
			activeOrderBook:   activeOrderbook,
			orderQueryService: mockOrderQueryService,
			exchange:          mockExchange,
		}

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

		activeOrderbook.Add(order1)
		mockExchange.EXPECT().QueryOpenOrders(ctx, symbol).Return([]types.Order{order2}, nil)
		mockOrderQueryService.EXPECT().QueryOrder(ctx, types.OrderQuery{
			Symbol:  symbol,
			OrderID: strconv.FormatUint(order1.OrderID, 10),
		}).Return(&updatedOrder1, nil)

		assert.NoError(syncActiveOrders(ctx, opts))

		// verify active orderbook
		activeOrders := activeOrderbook.Orders()
		assert.Equal(1, len(activeOrders))
		assert.Equal(uint64(2), activeOrders[0].OrderID)
		assert.Equal(types.OrderStatusNew, activeOrders[0].Status)
	})
}
