package xfundingv2

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

// compositeExchange combines multiple exchange interfaces for testing
type compositeExchange struct {
	types.Exchange
	types.ExchangeOrderQueryService
}

func newTestTWAPWorker(
	t *testing.T,
	ctrl *gomock.Controller,
	config TWAPWorkerConfig,
) (*TWAPWorker, *mocks.MockExchange, *mocks.MockExchangeOrderQueryService, *bbgo.GeneralOrderExecutor) {
	mockExchange := mocks.NewMockExchange(ctrl)
	mockOrderQuery := mocks.NewMockExchangeOrderQueryService(ctrl)

	// Create a composite exchange that implements both interfaces
	compositeEx := &compositeExchange{
		Exchange:                  mockExchange,
		ExchangeOrderQueryService: mockOrderQuery,
	}

	mockExchange.EXPECT().Name().Return(types.ExchangeBinance).AnyTimes()

	market := Market("BTCUSDT")
	position := types.NewPositionFromMarket(market)

	session := &bbgo.ExchangeSession{
		Exchange: compositeEx,
	}
	session.SetMarkets(map[string]types.Market{
		"BTCUSDT": market,
	})
	// Attach a non-nil account so TWAPWorker.calculateSliceQuantity can query
	// balances without panicking (mirrors production where session.Account is set).
	session.Account = types.NewAccount()

	generalExecutor := bbgo.NewGeneralOrderExecutor(session, "BTCUSDT", "test", "test-instance", position)

	ctx := context.Background()
	worker, err := NewTWAPWorker(ctx, "BTCUSDT", session, generalExecutor, config)
	assert.NoError(t, err)

	// Attach a logger so methods called directly (without Start) don't panic on a nil logger.
	logger := logrus.New()
	logger.SetLevel(logrus.PanicLevel)
	worker.SetLogger(logger)

	return worker, mockExchange, mockOrderQuery, generalExecutor
}

// processTrade simulates processing a trade through the TradeCollector and the TWAPWorker
func processTrade(worker *TWAPWorker, executor *bbgo.GeneralOrderExecutor, trade types.Trade) {
	// Add order to store first (if not already there)
	if _, found := executor.OrderStore().Get(trade.OrderID); !found {
		executor.OrderStore().Add(types.Order{
			OrderID: trade.OrderID,
			SubmitOrder: types.SubmitOrder{
				Symbol: trade.Symbol,
				Side:   trade.Side,
			},
		})
	}
	// Process trade to update position
	executor.TradeCollector().ProcessTrade(trade)
	// Add trade to the worker (simulates the strategy-level OnTrade callback)
	worker.Executor().AddTrade(trade)
}

// makeTrade creates a trade with fee in BNB to avoid affecting base/quote quantity
func makeTrade(id uint64, orderID uint64, side types.SideType, price, quantity fixedpoint.Value) types.Trade {
	return types.Trade{
		ID:          id,
		OrderID:     orderID,
		Symbol:      "BTCUSDT",
		Side:        side,
		Price:       price,
		Quantity:    quantity,
		Fee:         Number(0.001), // small fee in BNB
		FeeCurrency: "BNB",         // fee in BNB doesn't affect base/quote quantity
	}
}

// TestTWAPWorker_FillingOrders groups tests for order filling scenarios
func TestTWAPWorker_FillingOrders(t *testing.T) {
	t.Run("FullyFilledOrders", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{
			Duration:      types.Duration(10 * time.Minute),
			NumSlices:     5,
			OrderType:     TWAPOrderTypeMaker,
			CheckInterval: types.Duration(1 * time.Second),
			NumOfTicks:    1,
		}

		worker, mockExchange, mockOrderQuery, generalExecutor := newTestTWAPWorker(t, ctrl, config)

		// Target position: buy 5 BTC total
		targetPosition := Number(5.0)
		worker.SetTargetPosition(targetPosition)
		assert.Equal(t, types.SideTypeBuy, orderSide(worker.RemainingQuantity()))

		ctx := context.Background()
		startTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

		err := worker.Start(ctx, startTime)
		assert.NoError(t, err)

		orderBook := newOrderBook(99.0, 100.0, 100.0, 100.0)
		orderID := uint64(1000)

		mockExchange.EXPECT().
			SubmitOrder(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
				currentOrderID := orderID
				orderID++
				return &types.Order{
					OrderID: currentOrderID,
					SubmitOrder: types.SubmitOrder{
						Symbol:   order.Symbol,
						Side:     order.Side,
						Type:     order.Type,
						Quantity: order.Quantity,
						Price:    order.Price,
					},
					Status: types.OrderStatusNew,
				}, nil
			}).AnyTimes()

		mockExchange.EXPECT().
			CancelOrders(gomock.Any(), gomock.Any()).
			Return(nil).AnyTimes()

		mockOrderQuery.EXPECT().
			QueryOrder(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
				return &types.Order{OrderID: 1, Status: types.OrderStatusCanceled}, nil
			}).AnyTimes()

		mockOrderQuery.EXPECT().
			QueryOrderTrades(gomock.Any(), gomock.Any()).
			Return([]types.Trade{}, nil).AnyTimes()

		// Simulate 5 slices with full fills
		// Expected slice quantities for buying 5 BTC in 5 slices:
		// Slice 0: 5/5 = 1.0 BTC
		// Slice 1: 4/4 = 1.0 BTC
		// Slice 2: 3/3 = 1.0 BTC
		// Slice 3: 2/2 = 1.0 BTC
		// Slice 4: 1/1 = 1.0 BTC
		expectedSliceQuantities := []fixedpoint.Value{
			Number(1.0), Number(1.0), Number(1.0), Number(1.0), Number(1.0),
		}

		filledQty := fixedpoint.Zero
		tradeID := uint64(1)
		for slice := 0; slice < 5; slice++ {
			// Calculate what we expect to fill this slice (remaining / remaining_slices)
			remaining := targetPosition.Sub(filledQty)
			remainingSlices := 5 - slice
			expectedSliceQty := remaining.Div(fixedpoint.NewFromInt(int64(remainingSlices)))

			// Verify expected slice quantity matches our pre-calculated expectation
			assert.Equal(t, expectedSliceQuantities[slice], expectedSliceQty,
				"slice %d: expected slice quantity mismatch", slice)

			// Tick to place order
			sliceTime := startTime.Add(time.Duration(slice) * 2 * time.Minute)
			err = worker.Tick(sliceTime, orderBook)
			assert.NoError(t, err)

			// Get the active order and verify its quantity matches the expected slice quantity
			activeOrder := worker.ActiveOrder()
			assert.NotNil(t, activeOrder, "slice %d: active order should not be nil", slice)
			assert.Equal(t, expectedSliceQty, activeOrder.Quantity,
				"slice %d: active order quantity should match expected slice quantity", slice)

			// Simulate trade fill through TradeCollector
			trade := makeTrade(tradeID, activeOrder.OrderID, types.SideTypeBuy, Number(99.01), expectedSliceQty)
			processTrade(worker, generalExecutor, trade)
			filledQty = filledQty.Add(expectedSliceQty)

			// Verify cumulative filled quantity after each slice
			assert.Equal(t, filledQty, worker.FilledPosition(),
				"slice %d: cumulative filled quantity mismatch", slice)

			tradeID++
		}

		// Verify all filled
		assert.Equal(t, targetPosition, worker.FilledPosition())
		assert.True(t, worker.RemainingQuantity().IsZero())
	})

	t.Run("PartialFillCarryover", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{
			Duration:      types.Duration(6 * time.Minute),
			NumSlices:     3,
			OrderType:     TWAPOrderTypeMaker,
			CheckInterval: types.Duration(1 * time.Second),
			NumOfTicks:    1,
		}

		worker, mockExchange, mockOrderQuery, generalExecutor := newTestTWAPWorker(t, ctrl, config)

		// Target position: buy 3 BTC total
		targetPosition := Number(3.0)
		worker.SetTargetPosition(targetPosition)

		ctx := context.Background()
		startTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

		err := worker.Start(ctx, startTime)
		assert.NoError(t, err)

		orderBook := newOrderBook(99.0, 100.0, 100.0, 100.0)
		orderID := uint64(1000)

		mockExchange.EXPECT().
			SubmitOrder(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
				currentOrderID := orderID
				orderID++
				return &types.Order{
					OrderID: currentOrderID,
					SubmitOrder: types.SubmitOrder{
						Symbol:   order.Symbol,
						Side:     order.Side,
						Type:     order.Type,
						Quantity: order.Quantity,
						Price:    order.Price,
					},
					Status: types.OrderStatusNew,
				}, nil
			}).AnyTimes()

		mockExchange.EXPECT().
			CancelOrders(gomock.Any(), gomock.Any()).
			Return(nil).AnyTimes()

		mockOrderQuery.EXPECT().
			QueryOrder(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
				return &types.Order{OrderID: 1, Status: types.OrderStatusCanceled}, nil
			}).AnyTimes()

		mockOrderQuery.EXPECT().
			QueryOrderTrades(gomock.Any(), gomock.Any()).
			Return([]types.Trade{}, nil).AnyTimes()

		// Slice 1: Place order for ~1 BTC (3/3), only fill 0.5 BTC
		err = worker.Tick(startTime, orderBook)
		assert.NoError(t, err)

		activeOrder := worker.ActiveOrder()
		assert.NotNil(t, activeOrder)

		// Partial fill: 0.5 BTC
		trade1 := makeTrade(1, activeOrder.OrderID, types.SideTypeBuy, Number(99.01), Number(0.5))
		processTrade(worker, generalExecutor, trade1)

		assert.Equal(t, Number(0.5), worker.FilledPosition())
		assert.Equal(t, Number(2.5), worker.RemainingQuantity())

		// Slice 2: remaining = 2.5, remaining_slices = 2, so sliceQty = 1.25
		// Only fill 0.75
		slice2Time := startTime.Add(2 * time.Minute)
		err = worker.Tick(slice2Time, orderBook)
		assert.NoError(t, err)

		activeOrder = worker.ActiveOrder()
		trade2 := makeTrade(2, activeOrder.OrderID, types.SideTypeBuy, Number(99.01), Number(0.75))
		processTrade(worker, generalExecutor, trade2)

		assert.Equal(t, Number(1.25), worker.FilledPosition())
		assert.Equal(t, Number(1.75), worker.RemainingQuantity())

		// Slice 3: Fill remaining completely
		slice3Time := startTime.Add(4 * time.Minute)
		err = worker.Tick(slice3Time, orderBook)
		assert.NoError(t, err)

		activeOrder = worker.ActiveOrder()
		remaining := worker.RemainingQuantity()
		trade3 := makeTrade(3, activeOrder.OrderID, types.SideTypeBuy, Number(99.01), remaining)
		processTrade(worker, generalExecutor, trade3)

		// Verify all filled after carryover
		assert.Equal(t, targetPosition, worker.FilledPosition())
		assert.True(t, worker.RemainingQuantity().IsZero())
	})
}

// TestTWAPWorker_OpenThenClose groups tests for opening and closing positions
func TestTWAPWorker_OpenThenClose(t *testing.T) {
	t.Run("OpenLongThenClose", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{
			Duration:      types.Duration(4 * time.Minute),
			NumSlices:     2,
			OrderType:     TWAPOrderTypeMaker,
			CheckInterval: types.Duration(1 * time.Second),
			NumOfTicks:    1,
		}

		worker, mockExchange, mockOrderQuery, generalExecutor := newTestTWAPWorker(t, ctrl, config)

		// Phase 1: Open long position of 2 BTC
		targetPosition := Number(2.0)
		worker.SetTargetPosition(targetPosition)
		assert.Equal(t, types.SideTypeBuy, orderSide(worker.RemainingQuantity()))

		ctx := context.Background()
		startTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

		err := worker.Start(ctx, startTime)
		assert.NoError(t, err)

		orderBook := newOrderBook(99.0, 100.0, 100.0, 100.0)
		orderID := uint64(1000)

		mockExchange.EXPECT().
			SubmitOrder(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
				currentOrderID := orderID
				orderID++
				return &types.Order{
					OrderID: currentOrderID,
					SubmitOrder: types.SubmitOrder{
						Symbol:   order.Symbol,
						Side:     order.Side,
						Type:     order.Type,
						Quantity: order.Quantity,
						Price:    order.Price,
					},
					Status: types.OrderStatusNew,
				}, nil
			}).AnyTimes()

		mockExchange.EXPECT().
			CancelOrders(gomock.Any(), gomock.Any()).
			Return(nil).AnyTimes()

		mockOrderQuery.EXPECT().
			QueryOrder(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
				return &types.Order{OrderID: 1, Status: types.OrderStatusCanceled}, nil
			}).AnyTimes()

		mockOrderQuery.EXPECT().
			QueryOrderTrades(gomock.Any(), gomock.Any()).
			Return([]types.Trade{}, nil).AnyTimes()

		// First slice: buy 1 BTC (2/2)
		err = worker.Tick(startTime, orderBook)
		assert.NoError(t, err)

		activeOrder := worker.ActiveOrder()
		assert.NotNil(t, activeOrder)
		assert.Equal(t, types.SideTypeBuy, activeOrder.Side)

		trade1 := makeTrade(1, activeOrder.OrderID, types.SideTypeBuy, Number(99.01), Number(1.0))
		processTrade(worker, generalExecutor, trade1)

		// Second slice: buy another 1 BTC
		slice2Time := startTime.Add(2 * time.Minute)
		err = worker.Tick(slice2Time, orderBook)
		assert.NoError(t, err)

		activeOrder = worker.ActiveOrder()
		trade2 := makeTrade(2, activeOrder.OrderID, types.SideTypeBuy, Number(99.01), Number(1.0))
		processTrade(worker, generalExecutor, trade2)

		// Verify long position opened
		assert.Equal(t, Number(2.0), worker.FilledPosition())
		assert.True(t, worker.RemainingQuantity().IsZero())

		// Phase 2: Close the position (target = 0)
		worker.SetTargetPosition(fixedpoint.Zero)
		assert.Equal(t, types.SideTypeSell, orderSide(worker.RemainingQuantity()))

		// Reset time for closing phase - this also clears active order state
		closeStartTime := startTime.Add(5 * time.Minute)
		worker.ResetTime(closeStartTime, config.Duration)

		// Now remaining should be -2 (need to sell 2 BTC to reach target 0)
		assert.Equal(t, Number(-2.0), worker.RemainingQuantity())

		// First close slice: sell 1 BTC
		err = worker.Tick(closeStartTime, orderBook)
		assert.NoError(t, err)

		activeOrder = worker.ActiveOrder()
		assert.NotNil(t, activeOrder)
		assert.Equal(t, types.SideTypeSell, activeOrder.Side)

		trade3 := makeTrade(3, activeOrder.OrderID, types.SideTypeSell, Number(99.98), Number(1.0))
		processTrade(worker, generalExecutor, trade3)

		assert.Equal(t, Number(1.0), worker.FilledPosition())
		assert.Equal(t, Number(-1.0), worker.RemainingQuantity())

		// Second close slice: sell remaining 1 BTC
		closeSlice2Time := closeStartTime.Add(2 * time.Minute)
		err = worker.Tick(closeSlice2Time, orderBook)
		assert.NoError(t, err)

		activeOrder = worker.ActiveOrder()
		trade4 := makeTrade(4, activeOrder.OrderID, types.SideTypeSell, Number(99.98), Number(1.0))
		processTrade(worker, generalExecutor, trade4)

		// Verify position closed
		assert.True(t, worker.FilledPosition().IsZero())
		assert.True(t, worker.RemainingQuantity().IsZero())
	})

	t.Run("OpenShortThenClose", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{
			Duration:      types.Duration(4 * time.Minute),
			NumSlices:     2,
			OrderType:     TWAPOrderTypeMaker,
			CheckInterval: types.Duration(1 * time.Second),
			NumOfTicks:    1,
		}

		worker, mockExchange, mockOrderQuery, generalExecutor := newTestTWAPWorker(t, ctrl, config)

		// Phase 1: Open short position of -2 BTC (sell 2 BTC)
		targetPosition := Number(-2.0)
		worker.SetTargetPosition(targetPosition)
		assert.Equal(t, types.SideTypeSell, orderSide(worker.RemainingQuantity()))

		ctx := context.Background()
		startTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

		err := worker.Start(ctx, startTime)
		assert.NoError(t, err)

		orderBook := newOrderBook(99.0, 100.0, 100.0, 100.0)
		orderID := uint64(1000)

		mockExchange.EXPECT().
			SubmitOrder(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
				currentOrderID := orderID
				orderID++
				return &types.Order{
					OrderID: currentOrderID,
					SubmitOrder: types.SubmitOrder{
						Symbol:   order.Symbol,
						Side:     order.Side,
						Type:     order.Type,
						Quantity: order.Quantity,
						Price:    order.Price,
					},
					Status: types.OrderStatusNew,
				}, nil
			}).AnyTimes()

		mockExchange.EXPECT().
			CancelOrders(gomock.Any(), gomock.Any()).
			Return(nil).AnyTimes()

		mockOrderQuery.EXPECT().
			QueryOrder(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
				return &types.Order{OrderID: 1, Status: types.OrderStatusCanceled}, nil
			}).AnyTimes()

		mockOrderQuery.EXPECT().
			QueryOrderTrades(gomock.Any(), gomock.Any()).
			Return([]types.Trade{}, nil).AnyTimes()

		// First slice: sell 1 BTC (-2/2 = -1)
		err = worker.Tick(startTime, orderBook)
		assert.NoError(t, err)

		activeOrder := worker.ActiveOrder()
		assert.NotNil(t, activeOrder)
		assert.Equal(t, types.SideTypeSell, activeOrder.Side)

		trade1 := makeTrade(1, activeOrder.OrderID, types.SideTypeSell, Number(99.98), Number(1.0))
		processTrade(worker, generalExecutor, trade1)

		// Second slice: sell another 1 BTC
		slice2Time := startTime.Add(2 * time.Minute)
		err = worker.Tick(slice2Time, orderBook)
		assert.NoError(t, err)

		activeOrder = worker.ActiveOrder()
		trade2 := makeTrade(2, activeOrder.OrderID, types.SideTypeSell, Number(99.98), Number(1.0))
		processTrade(worker, generalExecutor, trade2)

		// Verify short position opened
		assert.Equal(t, Number(-2.0), worker.FilledPosition())
		assert.True(t, worker.RemainingQuantity().IsZero())

		// Phase 2: Close the position (target = 0)
		worker.SetTargetPosition(fixedpoint.Zero)
		assert.Equal(t, types.SideTypeBuy, orderSide(worker.RemainingQuantity()))

		// Reset time for closing phase
		closeStartTime := startTime.Add(5 * time.Minute)
		worker.ResetTime(closeStartTime, config.Duration)

		// Now remaining should be +2 (need to buy 2 BTC to reach target 0)
		assert.Equal(t, Number(2.0), worker.RemainingQuantity())

		// First close slice: buy 1 BTC
		err = worker.Tick(closeStartTime, orderBook)
		assert.NoError(t, err)

		activeOrder = worker.ActiveOrder()
		assert.NotNil(t, activeOrder)
		assert.Equal(t, types.SideTypeBuy, activeOrder.Side)

		trade3 := makeTrade(3, activeOrder.OrderID, types.SideTypeBuy, Number(99.01), Number(1.0))
		processTrade(worker, generalExecutor, trade3)

		assert.Equal(t, Number(-1.0), worker.FilledPosition())
		assert.Equal(t, Number(1.0), worker.RemainingQuantity())

		// Second close slice: buy remaining 1 BTC
		closeSlice2Time := closeStartTime.Add(2 * time.Minute)
		err = worker.Tick(closeSlice2Time, orderBook)
		assert.NoError(t, err)

		activeOrder = worker.ActiveOrder()
		trade4 := makeTrade(4, activeOrder.OrderID, types.SideTypeBuy, Number(99.01), Number(1.0))
		processTrade(worker, generalExecutor, trade4)

		// Verify position closed
		assert.True(t, worker.FilledPosition().IsZero())
		assert.True(t, worker.RemainingQuantity().IsZero())
	})
}

// TestTWAPWorker_Deadline groups tests for deadline exceeded scenarios
func TestTWAPWorker_Deadline(t *testing.T) {
	t.Run("StateTransitionToDone", func(t *testing.T) {
		// Tests state transition to Done when deadline exceeds with filled position
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{
			Duration:      types.Duration(2 * time.Minute),
			NumSlices:     2,
			OrderType:     TWAPOrderTypeMaker,
			CheckInterval: types.Duration(1 * time.Second),
			NumOfTicks:    1,
		}

		worker, mockExchange, _, generalExecutor := newTestTWAPWorker(t, ctrl, config)

		targetPosition := Number(1.0)
		worker.SetTargetPosition(targetPosition)

		ctx := context.Background()
		startTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

		err := worker.Start(ctx, startTime)
		assert.NoError(t, err)

		orderBook := newOrderBook(99.0, 100.0, 100.0, 100.0)
		orderID := uint64(1000)

		mockExchange.EXPECT().
			SubmitOrder(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
				currentOrderID := orderID
				orderID++
				return &types.Order{
					OrderID: currentOrderID,
					SubmitOrder: types.SubmitOrder{
						Symbol:   order.Symbol,
						Side:     order.Side,
						Type:     order.Type,
						Quantity: order.Quantity,
						Price:    order.Price,
					},
					Status: types.OrderStatusNew,
				}, nil
			}).AnyTimes()

		// First slice: full fill
		err = worker.Tick(startTime, orderBook)
		assert.NoError(t, err)

		activeOrder := worker.ActiveOrder()
		trade1 := makeTrade(1, activeOrder.OrderID, types.SideTypeBuy, Number(99.01), targetPosition)
		processTrade(worker, generalExecutor, trade1)

		assert.Equal(t, targetPosition, worker.FilledPosition())
		assert.True(t, worker.RemainingQuantity().IsZero())

		// Tick past deadline - position is filled so no new order needed
		deadlineTime := startTime.Add(3 * time.Minute)

		err = worker.Tick(deadlineTime, orderBook)
		assert.NoError(t, err)

		// After deadline, state should be done
		assert.Equal(t, TWAPWorkerStateDone, worker.State())
		assert.True(t, worker.IsDone())
	})

	t.Run("CancelActiveOrderAtDeadline", func(t *testing.T) {
		// Tests that active orders are canceled when deadline is exceeded
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{
			Duration:      types.Duration(2 * time.Minute),
			NumSlices:     2,
			OrderType:     TWAPOrderTypeMaker,
			CheckInterval: types.Duration(1 * time.Second),
			NumOfTicks:    1,
		}

		worker, mockExchange, mockOrderQuery, _ := newTestTWAPWorker(t, ctrl, config)

		targetPosition := Number(1.0)
		worker.SetTargetPosition(targetPosition)

		ctx := context.Background()
		startTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

		err := worker.Start(ctx, startTime)
		assert.NoError(t, err)

		orderBook := newOrderBook(99.0, 100.0, 100.0, 100.0)
		orderID := uint64(1000)

		mockExchange.EXPECT().
			SubmitOrder(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
				currentOrderID := orderID
				orderID++
				return &types.Order{
					OrderID: currentOrderID,
					SubmitOrder: types.SubmitOrder{
						Symbol:   order.Symbol,
						Side:     order.Side,
						Type:     order.Type,
						Quantity: order.Quantity,
						Price:    order.Price,
					},
					Status: types.OrderStatusNew,
				}, nil
			}).AnyTimes()

		// Track if CancelOrders was called
		cancelCalled := false
		mockExchange.EXPECT().
			CancelOrders(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, orders ...types.Order) error {
				cancelCalled = true
				return nil
			}).AnyTimes()

		mockOrderQuery.EXPECT().
			QueryOrder(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
				return &types.Order{OrderID: 1, Status: types.OrderStatusCanceled}, nil
			}).AnyTimes()

		mockOrderQuery.EXPECT().
			QueryOrderTrades(gomock.Any(), gomock.Any()).
			Return([]types.Trade{}, nil).AnyTimes()

		// First tick: place order but don't fill it
		err = worker.Tick(startTime, orderBook)
		assert.NoError(t, err)
		assert.NotNil(t, worker.ActiveOrder())

		// Tick past deadline with unfilled order
		// Note: This will fail to place the final market order due to dust check
		// but we can verify CancelOrders was called
		deadlineTime := startTime.Add(3 * time.Minute)
		_ = worker.Tick(deadlineTime, orderBook)

		// Verify that cancel was attempted
		assert.True(t, cancelCalled, "CancelOrders should be called when deadline exceeded with active order")
	})
}

// TestTWAPWorker_Misc groups miscellaneous tests
func TestTWAPWorker_Misc(t *testing.T) {
	t.Run("CalculateSliceQuantity", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{
			Duration:     types.Duration(10 * time.Minute),
			NumSlices:    5,
			OrderType:    TWAPOrderTypeMaker,
			MaxSliceSize: Number(0.5),
			MinSliceSize: Number(0.1),
		}

		worker, _, _, _ := newTestTWAPWorker(t, ctrl, config)

		market := Market("BTCUSDT")
		startTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
		worker.ResetTime(startTime, config.Duration)

		t.Run("normal slice calculation", func(t *testing.T) {
			// remaining = 2.0, time left = 8 min, interval = 2 min, remaining_slices = 4
			// expected = 2.0 / 4 = 0.5
			currentTime := startTime.Add(2 * time.Minute)
			sliceQty := worker.calculateSliceQuantity(currentTime, Number(2.0), false, market, fixedpoint.Zero)
			assert.Equal(t, Number(0.5), sliceQty)
		})

		t.Run("respects max slice size", func(t *testing.T) {
			// remaining = 5.0, remaining_slices = 5
			// expected = 5.0 / 5 = 1.0, but max = 0.5
			sliceQty := worker.calculateSliceQuantity(startTime, Number(5.0), false, market, fixedpoint.Zero)
			assert.Equal(t, Number(0.5), sliceQty)
		})

		t.Run("large remaining capped at max slice size", func(t *testing.T) {
			// remaining = 20.0, time left = 10 min, interval = 2 min, remaining_slices = 5
			// dynamic slice = 20.0 / 5 = 4.0, but max = 0.5 so it caps at 0.5
			sliceQty := worker.calculateSliceQuantity(startTime, Number(20.0), false, market, fixedpoint.Zero)
			assert.Equal(t, Number(0.5), sliceQty)
		})

		t.Run("respects min slice size", func(t *testing.T) {
			// remaining = 0.5, remaining_slices = 5
			// expected = 0.5 / 5 = 0.1
			sliceQty := worker.calculateSliceQuantity(startTime, Number(0.5), false, market, fixedpoint.Zero)
			assert.Equal(t, Number(0.1), sliceQty)
		})

		t.Run("remaining less than min returns remaining", func(t *testing.T) {
			// remaining = 0.05, which is less than min 0.1
			sliceQty := worker.calculateSliceQuantity(startTime, Number(0.05), false, market, fixedpoint.Zero)
			assert.Equal(t, Number(0.05), sliceQty)
		})

		t.Run("deadline exceeded returns all remaining", func(t *testing.T) {
			sliceQty := worker.calculateSliceQuantity(startTime, Number(3.0), true, market, fixedpoint.Zero)
			assert.Equal(t, Number(3.0), sliceQty)
		})

		t.Run("dust slice re-sliced by minQty", func(t *testing.T) {
			// BTCUSDT: MinNotional=10, MinQuantity=0.001
			// Use 20 slices so slice=1.0/20=0.05, notional=5<=10 => dust
			// Re-slice adds MinQuantity (0.001) until not dust. Since IsDustQuantity
			// treats notional<=MinNotional as dust, 0.1 (notional=10) is still dust,
			// so the smallest non-dust quantity is 0.101 (notional=10.1).
			dustConfig := TWAPWorkerConfig{
				Duration:  types.Duration(10 * time.Minute),
				NumSlices: 20,
				OrderType: TWAPOrderTypeMaker,
			}
			dustWorker, _, _, _ := newTestTWAPWorker(t, ctrl, dustConfig)
			dustWorker.ResetTime(startTime, dustConfig.Duration)

			sliceQty := dustWorker.calculateSliceQuantity(startTime, Number(1.0), false, market, Number(100.0))
			assert.Equal(t, Number(0.101), sliceQty)
		})

		t.Run("remaining less than minQty returns remaining", func(t *testing.T) {
			// remaining=0.05, price=100, notional=5<10 => dust
			// minQty=0.1, n=floor(0.05/0.1)=0 => return remaining
			dustConfig := TWAPWorkerConfig{
				Duration:  types.Duration(10 * time.Minute),
				NumSlices: 1,
				OrderType: TWAPOrderTypeMaker,
			}
			dustWorker, _, _, _ := newTestTWAPWorker(t, ctrl, dustConfig)
			dustWorker.ResetTime(startTime, dustConfig.Duration)

			sliceQty := dustWorker.calculateSliceQuantity(startTime, Number(0.05), false, market, Number(100.0))
			assert.Equal(t, Number(0.05), sliceQty)
		})

		t.Run("non-dust slice unchanged", func(t *testing.T) {
			// remaining=2.0, 5 slices, slice=0.4, price=100, notional=40>10 => not dust
			dustConfig := TWAPWorkerConfig{
				Duration:  types.Duration(10 * time.Minute),
				NumSlices: 5,
				OrderType: TWAPOrderTypeMaker,
			}
			dustWorker, _, _, _ := newTestTWAPWorker(t, ctrl, dustConfig)
			dustWorker.ResetTime(startTime, dustConfig.Duration)

			sliceQty := dustWorker.calculateSliceQuantity(startTime, Number(2.0), false, market, Number(100.0))
			assert.Equal(t, Number(0.4), sliceQty)
		})
	})

	t.Run("StateTransitions", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{
			Duration:  types.Duration(2 * time.Minute),
			NumSlices: 2,
			OrderType: TWAPOrderTypeMaker,
		}

		worker, mockExchange, _, generalExecutor := newTestTWAPWorker(t, ctrl, config)

		// Initially pending
		assert.Equal(t, TWAPWorkerStatePending, worker.State())

		worker.SetTargetPosition(Number(1.0))

		ctx := context.Background()
		startTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

		err := worker.Start(ctx, startTime)
		assert.NoError(t, err)

		// After start, should be running
		assert.Equal(t, TWAPWorkerStateRunning, worker.State())

		orderBook := newOrderBook(99.0, 100.0, 100.0, 100.0)

		mockExchange.EXPECT().
			SubmitOrder(gomock.Any(), gomock.Any()).
			Return(&types.Order{
				OrderID: 1,
				SubmitOrder: types.SubmitOrder{
					Symbol:   "BTCUSDT",
					Quantity: Number(1.0),
				},
			}, nil).AnyTimes()

		// Tick to place an order and fill it
		err = worker.Tick(startTime, orderBook)
		assert.NoError(t, err)

		// Fill the order completely so we don't need market order at deadline
		activeOrder := worker.ActiveOrder()
		trade := makeTrade(1, activeOrder.OrderID, types.SideTypeBuy, Number(99.01), Number(1.0))
		processTrade(worker, generalExecutor, trade)

		assert.True(t, worker.RemainingQuantity().IsZero())

		// Tick past end time - position is filled, no market order needed
		endTime := startTime.Add(3 * time.Minute)
		err = worker.Tick(endTime, orderBook)
		assert.NoError(t, err)

		// After deadline, should be done
		assert.Equal(t, TWAPWorkerStateDone, worker.State())
		assert.True(t, worker.IsDone())
	})

	t.Run("ShouldUpdateActiveOrder", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{
			Duration:   types.Duration(10 * time.Minute),
			NumSlices:  5,
			OrderType:  TWAPOrderTypeMaker,
			NumOfTicks: 1,
		}

		worker, _, _, _ := newTestTWAPWorker(t, ctrl, config)

		worker.SetTargetPosition(Number(1.0))

		ctx := context.Background()
		startTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

		err := worker.Start(ctx, startTime)
		assert.NoError(t, err)

		t.Run("no active order returns false", func(t *testing.T) {
			orderBook := newOrderBook(99.0, 100.0, 100.0, 100.0)
			result := worker.shouldUpdateActiveOrder(orderBook)
			assert.False(t, result)
		})

		t.Run("better buy price triggers update", func(t *testing.T) {
			worker.activeOrder = &types.Order{
				OrderID: 1,
				SubmitOrder: types.SubmitOrder{
					Side:     types.SideTypeBuy,
					Price:    Number(99.51),
					Quantity: Number(1.0),
				},
			}
			// New best bid is lower, so computed price 99.0+0.01=99.01 < 99.51 → better (cheaper)
			orderBook := newOrderBook(99.0, 100.0, 100.0, 100.0)
			result := worker.shouldUpdateActiveOrder(orderBook)
			assert.True(t, result)
		})

		t.Run("worse buy price does not trigger update", func(t *testing.T) {
			worker.activeOrder = &types.Order{
				OrderID: 1,
				SubmitOrder: types.SubmitOrder{
					Side:     types.SideTypeBuy,
					Price:    Number(99.01),
					Quantity: Number(1.0),
				},
			}
			// New best bid is higher, computed price 99.5+0.01=99.51 > 99.01 → not better
			orderBook := newOrderBook(99.5, 100.0, 100.0, 100.0)
			result := worker.shouldUpdateActiveOrder(orderBook)
			assert.False(t, result)
		})

		t.Run("better sell price triggers update", func(t *testing.T) {
			worker.SetTargetPosition(Number(-1.0))
			worker.activeOrder = &types.Order{
				OrderID: 1,
				SubmitOrder: types.SubmitOrder{
					Side:     types.SideTypeSell,
					Price:    Number(99.49),
					Quantity: Number(1.0),
				},
			}
			// New best ask is higher, computed price 100.0-0.01=99.99 > 99.49 → better (more expensive)
			orderBook := newOrderBook(99.0, 100.0, 100.0, 100.0)
			result := worker.shouldUpdateActiveOrder(orderBook)
			assert.True(t, result)
		})

		t.Run("worse sell price does not trigger update", func(t *testing.T) {
			worker.SetTargetPosition(Number(-1.0))
			worker.activeOrder = &types.Order{
				OrderID: 1,
				SubmitOrder: types.SubmitOrder{
					Side:     types.SideTypeSell,
					Price:    Number(99.99),
					Quantity: Number(1.0),
				},
			}
			// New best ask is lower, computed price 99.5-0.01=99.49 < 99.99 → not better
			orderBook := newOrderBook(99.0, 100.0, 99.5, 100.0)
			result := worker.shouldUpdateActiveOrder(orderBook)
			assert.False(t, result)
		})
	})

	t.Run("TakerOrderTypeAlwaysUpdates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := TWAPWorkerConfig{
			Duration:   types.Duration(10 * time.Minute),
			NumSlices:  5,
			OrderType:  TWAPOrderTypeTaker, // Taker orders
			NumOfTicks: 1,
		}

		worker, _, _, _ := newTestTWAPWorker(t, ctrl, config)

		worker.SetTargetPosition(Number(1.0))

		ctx := context.Background()
		startTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

		err := worker.Start(ctx, startTime)
		assert.NoError(t, err)

		worker.activeOrder = &types.Order{
			OrderID: 1,
			SubmitOrder: types.SubmitOrder{
				Side:     types.SideTypeBuy,
				Price:    Number(100.0),
				Quantity: Number(1.0),
			},
		}

		// For taker orders, should always return true to refresh
		orderBook := newOrderBook(99.0, 100.0, 100.0, 100.0)
		result := worker.shouldUpdateActiveOrder(orderBook)
		assert.True(t, result)
	})
}
