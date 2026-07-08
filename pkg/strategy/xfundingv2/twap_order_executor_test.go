package xfundingv2

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

// testExecutorSetup creates a TWAPExecutor for testing with mocked dependencies
func testExecutorSetup(
	t *testing.T,
	ctrl *gomock.Controller,
	config TWAPWorkerConfig,
) (*TWAPExecutor, *mocks.MockExchangeOrderQueryService, *mocks.MockExchange) {
	market := Market("BTCUSDT")
	position := types.NewPositionFromMarket(market)

	mockExchange := mocks.NewMockExchange(ctrl)
	mockExchange.EXPECT().Name().Return(types.ExchangeBinance).AnyTimes()

	// Setup mock exchange with required methods for GeneralOrderExecutor
	mockOrderQuery := mocks.NewMockExchangeOrderQueryService(ctrl)

	session := &bbgo.ExchangeSession{
		Exchange: mockExchange,
	}
	session.SetMarkets(map[string]types.Market{
		"BTCUSDT": market,
	})

	generalExecutor := bbgo.NewGeneralOrderExecutor(session, "BTCUSDT", "test", "test-instance", position)

	ctx := context.Background()
	executor := NewTWAPExecutor(
		ctx,
		mockOrderQuery,
		false,
		market,
		generalExecutor,
		config,
	)

	return executor, mockOrderQuery, mockExchange
}

func TestNewTWAPOrderExecutor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := TWAPWorkerConfig{
		OrderType:  TWAPOrderTypeMaker,
		NumOfTicks: 2,
	}

	executor, _, _ := testExecutorSetup(t, ctrl, config)

	assert.NotNil(t, executor)
	assert.Equal(t, TWAPOrderTypeMaker, executor.syncState.Config.OrderType)
	assert.Equal(t, 2, executor.syncState.Config.NumOfTicks)
}

func TestTWAPOrderExecutor_Start(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := TWAPWorkerConfig{}
	executor, _, _ := testExecutorSetup(t, ctrl, config)

	assert.Nil(t, executor.logger)
	executor.Start()
	assert.NotNil(t, executor.logger)
}

func TestTWAPOrderExecutor_Stop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := TWAPWorkerConfig{}
	executor, _, _ := testExecutorSetup(t, ctrl, config)

	err := executor.Stop()
	assert.NoError(t, err)
}

func TestTWAPOrderExecutor_GetPrice_Taker(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := TWAPWorkerConfig{
		OrderType: TWAPOrderTypeTaker,
	}

	executor, _, _ := testExecutorSetup(t, ctrl, config)

	orderBook := newOrderBook(99.0, 10.0, 100.0, 10.0)

	t.Run("buy side returns ask price", func(t *testing.T) {
		price, err := executor.GetPrice(types.SideTypeBuy, orderBook)
		assert.NoError(t, err)
		assert.Equal(t, Number(100.0), price)
	})

	t.Run("sell side returns bid price", func(t *testing.T) {
		price, err := executor.GetPrice(types.SideTypeSell, orderBook)
		assert.NoError(t, err)
		assert.Equal(t, Number(99.0), price)
	})
}

func TestTWAPOrderExecutor_GetPrice_Maker(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := TWAPWorkerConfig{
		OrderType:  TWAPOrderTypeMaker,
		NumOfTicks: 2,
	}

	executor, _, _ := testExecutorSetup(t, ctrl, config)

	// orderBook with bid=99.0, ask=100.0, tickSize=0.01
	orderBook := newOrderBook(99.0, 10.0, 100.0, 10.0)

	t.Run("buy side improves bid price", func(t *testing.T) {
		// bid=99.0, numOfTicks=2, tickSize=0.01
		// improved price = 99.0 + 2*0.01 = 99.02
		price, err := executor.GetPrice(types.SideTypeBuy, orderBook)
		assert.NoError(t, err)
		assert.Equal(t, Number(99.02), price)
	})

	t.Run("sell side improves ask price", func(t *testing.T) {
		// ask=100.0, numOfTicks=2, tickSize=0.01
		// improved price = 100.0 - 2*0.01 = 99.98
		price, err := executor.GetPrice(types.SideTypeSell, orderBook)
		assert.NoError(t, err)
		assert.Equal(t, Number(99.98), price)
	})
}

func TestTWAPOrderExecutor_GetTakerPrice(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := TWAPWorkerConfig{
		OrderType:   TWAPOrderTypeTaker,
		MaxSlippage: Number(0.01), // 1%
	}

	executor, _, _ := testExecutorSetup(t, ctrl, config)

	orderBook := newOrderBook(99.0, 10.0, 100.0, 10.0)

	t.Run("buy with max slippage", func(t *testing.T) {
		// ask=100.0, maxSlippage=0.01
		// maxPrice = 100.0 * (1 + 0.01) = 101.0
		price, err := executor.GetPrice(types.SideTypeBuy, orderBook)
		assert.NoError(t, err)
		assert.Equal(t, Number(101.0), price)
	})

	t.Run("sell with max slippage", func(t *testing.T) {
		// bid=99.0, maxSlippage=0.01
		// minPrice = 99.0 * (1 - 0.01) = 98.01
		price, err := executor.GetPrice(types.SideTypeSell, orderBook)
		assert.NoError(t, err)
		assert.Equal(t, Number("98.01"), price)
	})

	t.Run("empty order book returns error for buy", func(t *testing.T) {
		emptyBook := &types.SliceOrderBook{}
		_, err := executor.GetPrice(types.SideTypeBuy, emptyBook)
		assert.Error(t, err)
	})

	t.Run("empty order book returns error for sell", func(t *testing.T) {
		emptyBook := &types.SliceOrderBook{}
		_, err := executor.GetPrice(types.SideTypeSell, emptyBook)
		assert.Error(t, err)
	})
}

func TestTWAPOrderExecutor_GetMakerPrice(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := TWAPWorkerConfig{
		OrderType:  TWAPOrderTypeMaker,
		NumOfTicks: 5,
	}

	executor, _, _ := testExecutorSetup(t, ctrl, config)

	t.Run("buy price does not cross spread", func(t *testing.T) {
		// narrow spread: bid=99.99, ask=100.0
		// improvement = 5 * 0.01 = 0.05
		// improved = 99.99 + 0.05 = 100.04, but ask=100.0
		// should not cross: price = ask - tickSize = 100.0 - 0.01 = 99.99
		narrowBook := newOrderBook(99.99, 10.0, 100.0, 10.0)
		price, err := executor.GetPrice(types.SideTypeBuy, narrowBook)
		assert.NoError(t, err)
		assert.Equal(t, Number(99.99), price)
	})

	t.Run("sell price does not cross spread", func(t *testing.T) {
		// narrow spread: bid=99.99, ask=100.0
		// improvement = 5 * 0.01 = 0.05
		// improved = 100.0 - 0.05 = 99.95, but bid=99.99
		// should not cross: price = bid + tickSize = 99.99 + 0.01 = 100.0
		narrowBook := newOrderBook(99.99, 10.0, 100.0, 10.0)
		price, err := executor.GetPrice(types.SideTypeSell, narrowBook)
		assert.NoError(t, err)
		assert.Equal(t, Number(100.0), price)
	})

	t.Run("wide spread allows full improvement", func(t *testing.T) {
		// wide spread: bid=90.0, ask=100.0
		// improvement = 5 * 0.01 = 0.05
		// improved buy = 90.0 + 0.05 = 90.05
		wideBook := newOrderBook(90.0, 10.0, 100.0, 10.0)
		price, err := executor.GetPrice(types.SideTypeBuy, wideBook)
		assert.NoError(t, err)
		assert.Equal(t, Number(90.05), price)

		// improved sell = 100.0 - 0.05 = 99.95
		price, err = executor.GetPrice(types.SideTypeSell, wideBook)
		assert.NoError(t, err)
		assert.Equal(t, Number(99.95), price)
	})

	t.Run("empty order book returns error", func(t *testing.T) {
		emptyBook := &types.SliceOrderBook{}
		_, err := executor.GetPrice(types.SideTypeBuy, emptyBook)
		assert.Error(t, err)

		_, err = executor.GetPrice(types.SideTypeSell, emptyBook)
		assert.Error(t, err)
	})
}

func TestTWAPOrderExecutor_BuildSubmitOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	market := Market("BTCUSDT")

	t.Run("deadline exceeded creates market order", func(t *testing.T) {
		config := TWAPWorkerConfig{
			OrderType: TWAPOrderTypeMaker,
		}
		executor, _, _ := testExecutorSetup(t, ctrl, config)

		order := executor.buildSubmitOrder(Number(1.0), Number(100.0), types.SideTypeBuy, TWAPExecuteOrderOptions{DeadlineExceeded: true})

		assert.Equal(t, market.Symbol, order.Symbol)
		assert.Equal(t, types.SideTypeBuy, order.Side)
		assert.Equal(t, types.OrderTypeMarket, order.Type)
		assert.Equal(t, Number(1.0), order.Quantity)
	})

	t.Run("maker order creates limit maker with GTC", func(t *testing.T) {
		ctrl2 := gomock.NewController(t)
		defer ctrl2.Finish()

		config := TWAPWorkerConfig{
			OrderType: TWAPOrderTypeMaker,
		}
		executor, _, _ := testExecutorSetup(t, ctrl2, config)

		order := executor.buildSubmitOrder(Number(1.5), Number(50000.0), types.SideTypeSell, TWAPExecuteOrderOptions{})

		assert.Equal(t, market.Symbol, order.Symbol)
		assert.Equal(t, types.SideTypeSell, order.Side)
		assert.Equal(t, types.OrderTypeLimitMaker, order.Type)
		assert.Equal(t, Number(1.5), order.Quantity)
		assert.Equal(t, Number(50000.0), order.Price)
		assert.Equal(t, types.TimeInForce(""), order.TimeInForce)
	})

	t.Run("taker order creates limit with IOC", func(t *testing.T) {
		ctrl2 := gomock.NewController(t)
		defer ctrl2.Finish()

		config := TWAPWorkerConfig{
			OrderType: TWAPOrderTypeTaker,
		}
		executor, _, _ := testExecutorSetup(t, ctrl2, config)

		order := executor.buildSubmitOrder(Number(2.0), Number(48000.0), types.SideTypeBuy, TWAPExecuteOrderOptions{})

		assert.Equal(t, market.Symbol, order.Symbol)
		assert.Equal(t, types.SideTypeBuy, order.Side)
		assert.Equal(t, types.OrderTypeLimit, order.Type)
		assert.Equal(t, Number(2.0), order.Quantity)
		assert.Equal(t, Number(48000.0), order.Price)
		assert.Equal(t, types.TimeInForceIOC, order.TimeInForce)
	})
}

func TestTWAPOrderExecutor_PlaceOrder(t *testing.T) {
	t.Run("successful order placement", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{
			OrderType:  TWAPOrderTypeMaker,
			NumOfTicks: 1,
		}

		executor, _, mockExchange := testExecutorSetup(t, ctrl, config)
		executor.Start()

		orderBook := newOrderBook(99.0, 10.0, 100.0, 10.0)
		expectedOrder := types.Order{
			OrderID: 12345,
			SubmitOrder: types.SubmitOrder{
				Symbol:   "BTCUSDT",
				Side:     types.SideTypeBuy,
				Type:     types.OrderTypeLimitMaker,
				Quantity: Number(1.0),
				Price:    Number(99.01),
			},
		}

		mockExchange.EXPECT().
			SubmitOrder(gomock.Any(), gomock.Any()).
			Return(&expectedOrder, nil).
			Times(1)

		createdOrder, err := executor.PlaceOrder(Number(1.0), types.SideTypeBuy, orderBook, TWAPExecuteOrderOptions{
			DeadlineExceeded: false,
		})

		assert.NoError(t, err)
		assert.NotNil(t, createdOrder)
		assert.Equal(t, uint64(12345), createdOrder.OrderID)
	})

	t.Run("dust quantity returns error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{
			OrderType:  TWAPOrderTypeMaker,
			NumOfTicks: 1,
		}

		executor, _, _ := testExecutorSetup(t, ctrl, config)
		executor.Start()

		orderBook := newOrderBook(99.0, 10.0, 100.0, 10.0)

		// Very small quantity that would be dust
		_, err := executor.PlaceOrder(Number(0.00000001), types.SideTypeBuy, orderBook, TWAPExecuteOrderOptions{
			DeadlineExceeded: false,
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dust quantity")
	})

	t.Run("exchange error is returned", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{
			OrderType:  TWAPOrderTypeMaker,
			NumOfTicks: 1,
		}

		executor, _, mockExchange := testExecutorSetup(t, ctrl, config)
		executor.Start()

		orderBook := newOrderBook(99.0, 10.0, 100.0, 10.0)
		expectedErr := errors.New("exchange error")

		mockExchange.EXPECT().
			SubmitOrder(gomock.Any(), gomock.Any()).
			Return(nil, expectedErr).
			Times(1)

		createdOrder, err := executor.PlaceOrder(Number(1.0), types.SideTypeBuy, orderBook, TWAPExecuteOrderOptions{
			DeadlineExceeded: false,
		})

		assert.Error(t, err)
		assert.Nil(t, createdOrder)
	})
}

func TestTWAPOrderExecutor_SyncOrder(t *testing.T) {
	t.Run("order not in store skips sync", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{}
		executor, _, _ := testExecutorSetup(t, ctrl, config)
		executor.Start()

		order := types.Order{
			OrderID: 12345,
			SubmitOrder: types.SubmitOrder{
				Symbol:   "BTCUSDT",
				Quantity: Number(1.0),
			},
		}

		// order is not in the store, so SyncOrder should skip without querying the exchange
		// (mockOrderQuery has no expectations set, so any call would fail the test)
		err := executor.SyncOrder(order)
		assert.NoError(t, err)

		// Verify order was NOT added to store
		_, found := executor.executor.OrderStore().Get(12345)
		assert.False(t, found)
	})

	t.Run("fully filled order skips sync", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{}
		executor, _, _ := testExecutorSetup(t, ctrl, config)

		// Add a fully filled order to store
		filledOrder := types.Order{
			OrderID: 12345,
			SubmitOrder: types.SubmitOrder{
				Symbol:   "BTCUSDT",
				Quantity: Number(1.0),
			},
			ExecutedQuantity: Number(1.0),
			Status:           types.OrderStatusFilled,
		}
		executor.executor.OrderStore().Add(filledOrder)

		// SyncOrder should return nil without querying exchange
		err := executor.SyncOrder(filledOrder)
		assert.NoError(t, err)
	})

	t.Run("successful sync updates order and processes trades", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{}
		executor, mockOrderQuery, _ := testExecutorSetup(t, ctrl, config)

		// Add initial order to store
		order := types.Order{
			OrderID: 12345,
			SubmitOrder: types.SubmitOrder{
				Symbol:   "BTCUSDT",
				Quantity: Number(1.0),
			},
			Status: types.OrderStatusNew,
		}
		executor.executor.OrderStore().Add(order)

		updatedOrder := &types.Order{
			OrderID: 12345,
			SubmitOrder: types.SubmitOrder{
				Symbol:   "BTCUSDT",
				Quantity: Number(1.0),
			},
			ExecutedQuantity: Number(0.5),
			Status:           types.OrderStatusPartiallyFilled,
		}
		trades := []types.Trade{
			{ID: 1, OrderID: 12345},
		}

		mockOrderQuery.EXPECT().
			QueryOrder(gomock.Any(), gomock.Any()).
			Return(updatedOrder, nil).
			Times(1)
		mockOrderQuery.EXPECT().
			QueryOrderTrades(gomock.Any(), gomock.Any()).
			Return(trades, nil).
			Times(1)

		err := executor.SyncOrder(order)
		assert.NoError(t, err)

		// Verify order was updated in store
		storedOrder, found := executor.executor.OrderStore().Get(12345)
		assert.True(t, found)
		assert.Equal(t, types.OrderStatusPartiallyFilled, storedOrder.Status)
	})

	t.Run("query order failure returns error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{}
		executor, mockOrderQuery, _ := testExecutorSetup(t, ctrl, config)

		order := types.Order{
			OrderID: 12345,
			SubmitOrder: types.SubmitOrder{
				Symbol:   "BTCUSDT",
				Quantity: Number(1.0),
			},
			Status: types.OrderStatusNew,
		}
		executor.executor.OrderStore().Add(order)

		mockOrderQuery.EXPECT().
			QueryOrder(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("query failed")).
			Times(1)

		err := executor.SyncOrder(order)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to query order")
	})

	t.Run("query trades failure returns error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{}
		executor, mockOrderQuery, _ := testExecutorSetup(t, ctrl, config)

		order := types.Order{
			OrderID: 12345,
			SubmitOrder: types.SubmitOrder{
				Symbol:   "BTCUSDT",
				Quantity: Number(1.0),
			},
			Status: types.OrderStatusNew,
		}
		executor.executor.OrderStore().Add(order)

		updatedOrder := &types.Order{OrderID: 12345}

		mockOrderQuery.EXPECT().
			QueryOrder(gomock.Any(), gomock.Any()).
			Return(updatedOrder, nil).
			Times(1)
		mockOrderQuery.EXPECT().
			QueryOrderTrades(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("trades query failed")).
			Times(1)

		err := executor.SyncOrder(order)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to query order trades")
	})
}

func TestTWAPOrderExecutor_CancelOrder(t *testing.T) {
	t.Run("successful cancel", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{}
		executor, _, mockExchange := testExecutorSetup(t, ctrl, config)
		executor.Start()

		order := types.Order{OrderID: 12345, SubmitOrder: types.SubmitOrder{Symbol: "BTCUSDT"}}
		executor.syncState.Orders[order.OrderID] = order.AsQuery()

		mockExchange.EXPECT().
			CancelOrders(gomock.Any(), order).
			Return(nil).
			Times(1)

		ctx := context.Background()
		err := executor.CancelOrder(ctx, order)
		assert.NoError(t, err)
	})

	t.Run("cancel exchange error is gracefully ignored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{}
		executor, _, mockExchange := testExecutorSetup(t, ctrl, config)
		executor.Start()

		order := types.Order{OrderID: 12345, SubmitOrder: types.SubmitOrder{Symbol: "BTCUSDT"}}
		executor.syncState.Orders[order.OrderID] = order.AsQuery()

		// GracefulCancel logs CancelOrders errors but does not propagate them
		mockExchange.EXPECT().
			CancelOrders(gomock.Any(), order).
			Return(errors.New("cancel failed")).
			Times(1)

		ctx := context.Background()
		err := executor.CancelOrder(ctx, order)
		assert.NoError(t, err)
	})
}

func TestTWAPOrderExecutor_GetPrice_UnknownSide(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := TWAPWorkerConfig{
		OrderType: TWAPOrderTypeTaker,
	}

	executor, _, _ := testExecutorSetup(t, ctrl, config)

	orderBook := newOrderBook(99.0, 10.0, 100.0, 10.0)

	// Unknown side should return error
	_, err := executor.GetPrice(types.SideType("unknown"), orderBook)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown side")
}

func TestTWAPOrderExecutor_GetPrice_DefaultToTaker(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := TWAPWorkerConfig{
		OrderType: TWAPOrderType("invalid"), // invalid type should default to taker
	}

	executor, _, _ := testExecutorSetup(t, ctrl, config)

	orderBook := newOrderBook(99.0, 10.0, 100.0, 10.0)

	// Should default to taker price (ask for buy)
	price, err := executor.GetPrice(types.SideTypeBuy, orderBook)
	assert.NoError(t, err)
	assert.Equal(t, Number(100.0), price)
}

func TestTWAPOrderExecutor_PlaceOrder_EmptyOrderBook(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := TWAPWorkerConfig{
		OrderType:  TWAPOrderTypeMaker,
		NumOfTicks: 1,
	}

	executor, _, _ := testExecutorSetup(t, ctrl, config)
	executor.Start()

	emptyBook := &types.SliceOrderBook{}

	_, err := executor.PlaceOrder(Number(1.0), types.SideTypeBuy, emptyBook, TWAPExecuteOrderOptions{
		DeadlineExceeded: false,
	})
	assert.Error(t, err)
}

func TestTWAPOrderExecutor_GetOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := TWAPWorkerConfig{}
	executor, _, _ := testExecutorSetup(t, ctrl, config)
	executor.Start()

	// Add order to store
	order := types.Order{
		OrderID: 12345,
		SubmitOrder: types.SubmitOrder{
			Symbol: "BTCUSDT",
		},
	}
	executor.syncState.Orders[order.OrderID] = order.AsQuery() // Add to ordersMap to simulate tracking
	executor.executor.OrderStore().Add(order)

	// Test GetOrder
	foundOrder, found := executor.GetOrder(12345)
	assert.True(t, found)
	assert.Equal(t, uint64(12345), foundOrder.OrderID)

	// Test not found
	_, found = executor.GetOrder(99999)
	assert.False(t, found)
}
