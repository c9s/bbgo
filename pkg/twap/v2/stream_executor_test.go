package twap

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

func getTestMarket() types.Market {
	market := types.Market{
		Symbol:          "BTCUSDT",
		PricePrecision:  8,
		VolumePrecision: 8,
		QuoteCurrency:   "USDT",
		BaseCurrency:    "BTC",
		MinNotional:     fixedpoint.MustNewFromString("0.001"),
		MinAmount:       fixedpoint.MustNewFromString("10.0"),
		MinQuantity:     fixedpoint.MustNewFromString("0.001"),
	}
	return market
}

type OrderMatcher struct {
	Order types.Order
}

func MatchOrder(o types.Order) *OrderMatcher {
	return &OrderMatcher{
		Order: o,
	}
}

func (m *OrderMatcher) Matches(x interface{}) bool {
	order, ok := x.(types.Order)
	if !ok {
		return false
	}

	return m.Order.OrderID == order.OrderID && m.Order.Price.Compare(m.Order.Price) == 0
}

func (m *OrderMatcher) String() string {
	return fmt.Sprintf("OrderMatcher expects %+v", m.Order)
}

type CatchMatcher struct {
	f func(x any)
}

func Catch(f func(x any)) *CatchMatcher {
	return &CatchMatcher{
		f: f,
	}
}

func (m *CatchMatcher) Matches(x interface{}) bool {
	m.f(x)
	return true
}

func (m *CatchMatcher) String() string {
	return "CatchMatcher"
}

func bindMockMarketDataStream(mockStream *mocks.MockStream, stream *types.StandardStream) {
	mockStream.EXPECT().OnBookSnapshot(Catch(func(x any) {
		stream.OnBookSnapshot(x.(func(book types.SliceOrderBook)))
	})).AnyTimes()
	mockStream.EXPECT().OnBookUpdate(Catch(func(x any) {
		stream.OnBookUpdate(x.(func(book types.SliceOrderBook)))
	})).AnyTimes()
	mockStream.EXPECT().OnConnect(Catch(func(x any) {
		stream.OnConnect(x.(func()))
	})).AnyTimes()
}

func bindMockUserDataStream(mockStream *mocks.MockStream, stream *types.StandardStream) {
	mockStream.EXPECT().OnOrderUpdate(Catch(func(x any) {
		stream.OnOrderUpdate(x.(func(order types.Order)))
	})).AnyTimes()
	mockStream.EXPECT().OnTradeUpdate(Catch(func(x any) {
		stream.OnTradeUpdate(x.(func(order types.Trade)))
	})).AnyTimes()
	mockStream.EXPECT().OnBalanceUpdate(Catch(func(x any) {
		stream.OnBalanceUpdate(x.(func(m types.BalanceMap)))
	})).AnyTimes()
	mockStream.EXPECT().OnConnect(Catch(func(x any) {
		stream.OnConnect(x.(func()))
	})).AnyTimes()
	mockStream.EXPECT().OnAuth(Catch(func(x any) {
		stream.OnAuth(x.(func()))
	}))
}

func TestNewStreamExecutor(t *testing.T) {
	exchangeName := types.ExchangeBinance
	symbol := "BTCUSDT"
	market := getTestMarket()

	targetQuantity := Number(100)
	sliceQuantity := Number(1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockEx := mocks.NewMockExchange(mockCtrl)

	marketDataStream := &types.StandardStream{}
	userDataStream := &types.StandardStream{}

	mockMarketDataStream := mocks.NewMockStream(mockCtrl)
	mockMarketDataStream.EXPECT().SetPublicOnly()
	mockMarketDataStream.EXPECT().Subscribe(types.BookChannel, symbol, types.SubscribeOptions{
		Depth: types.DepthLevelMedium,
	})

	bindMockMarketDataStream(mockMarketDataStream, marketDataStream)

	mockMarketDataStream.EXPECT().Connect(gomock.AssignableToTypeOf(ctx))

	mockUserDataStream := mocks.NewMockStream(mockCtrl)
	bindMockUserDataStream(mockUserDataStream, userDataStream)
	mockUserDataStream.EXPECT().Connect(gomock.AssignableToTypeOf(ctx))

	initialBalances := types.BalanceMap{
		"BTC": types.Balance{
			Available: Number(2),
		},
		"USDT": types.Balance{
			Available: Number(20_000),
		},
	}

	mockEx.EXPECT().NewStream().Return(mockMarketDataStream)
	mockEx.EXPECT().NewStream().Return(mockUserDataStream)
	mockEx.EXPECT().QueryAccountBalances(gomock.AssignableToTypeOf(ctx)).Return(initialBalances, nil)

	// first order
	firstSubmitOrder := types.SubmitOrder{
		Symbol:      symbol,
		Side:        types.SideTypeBuy,
		Type:        types.OrderTypeLimitMaker,
		Quantity:    Number(1),
		Price:       Number(19400),
		Market:      market,
		TimeInForce: types.TimeInForceGTC,
	}
	firstSubmitOrderTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	firstOrder := types.Order{
		SubmitOrder:      firstSubmitOrder,
		Exchange:         exchangeName,
		OrderID:          1,
		Status:           types.OrderStatusNew,
		ExecutedQuantity: Number(0.0),
		IsWorking:        true,
		CreationTime:     types.Time(firstSubmitOrderTime),
		UpdateTime:       types.Time(firstSubmitOrderTime),
	}
	mockEx.EXPECT().SubmitOrder(gomock.AssignableToTypeOf(ctx), firstSubmitOrder).Return(&firstOrder, nil)

	executor := NewFixedQuantityExecutor(mockEx, symbol, market, types.SideTypeBuy, targetQuantity, sliceQuantity)
	executor.SetUpdateInterval(200 * time.Millisecond)

	go func() {
		err := executor.Start(ctx)
		assert.NoError(t, err)
	}()

	go func() {
		time.Sleep(500 * time.Millisecond)
		marketDataStream.EmitConnect()
		userDataStream.EmitConnect()
		userDataStream.EmitAuth()
	}()

	err := executor.WaitForConnection(ctx)
	assert.NoError(t, err)

	t.Logf("sending book snapshot...")
	snapshotTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	marketDataStream.EmitBookSnapshot(types.SliceOrderBook{
		Symbol: symbol,
		Bids: types.PriceVolumeSlice{
			{Price: Number(19400), Volume: Number(1)},
			{Price: Number(19300), Volume: Number(2)},
			{Price: Number(19200), Volume: Number(3)},
		},
		Asks: types.PriceVolumeSlice{
			{Price: Number(19450), Volume: Number(1)},
			{Price: Number(19550), Volume: Number(2)},
			{Price: Number(19650), Volume: Number(3)},
		},
		Time:         snapshotTime,
		LastUpdateId: 101,
	})

	time.Sleep(500 * time.Millisecond)

	t.Logf("sending book update...")

	// we expect the second order will be placed when the order update is received
	secondSubmitOrder := types.SubmitOrder{
		Symbol:      symbol,
		Side:        types.SideTypeBuy,
		Type:        types.OrderTypeLimitMaker,
		Quantity:    Number(1),
		Price:       Number(19420),
		Market:      market,
		TimeInForce: types.TimeInForceGTC,
	}
	secondSubmitOrderTime := time.Date(2021, 1, 1, 0, 1, 0, 0, time.UTC)
	secondOrder := types.Order{
		SubmitOrder:      secondSubmitOrder,
		Exchange:         exchangeName,
		OrderID:          2,
		Status:           types.OrderStatusNew,
		ExecutedQuantity: Number(0.0),
		IsWorking:        true,
		CreationTime:     types.Time(secondSubmitOrderTime),
		UpdateTime:       types.Time(secondSubmitOrderTime),
	}
	mockEx.EXPECT().CancelOrders(context.Background(), MatchOrder(firstOrder)).DoAndReturn(func(
		ctx context.Context, orders ...types.Order,
	) error {
		orderUpdate := firstOrder
		orderUpdate.Status = types.OrderStatusCanceled
		userDataStream.EmitOrderUpdate(orderUpdate)
		t.Logf("emit order update: %+v", orderUpdate)
		return nil
	})
	mockEx.EXPECT().QueryAccountBalances(gomock.AssignableToTypeOf(ctx)).Return(initialBalances, nil)
	mockEx.EXPECT().SubmitOrder(gomock.AssignableToTypeOf(ctx), secondSubmitOrder).Return(&secondOrder, nil)

	t.Logf("waiting for the order update...")
	time.Sleep(500 * time.Millisecond)
	{
		orders := executor.orderStore.Orders()
		assert.Len(t, orders, 1, "should have 1 order in the order store")
	}

	marketDataStream.EmitBookUpdate(types.SliceOrderBook{
		Symbol: symbol,
		Bids: types.PriceVolumeSlice{
			{Price: Number(19420), Volume: Number(1)},
			{Price: Number(19300), Volume: Number(2)},
			{Price: Number(19200), Volume: Number(3)},
		},
		Asks: types.PriceVolumeSlice{
			{Price: Number(19450), Volume: Number(1)},
			{Price: Number(19550), Volume: Number(2)},
			{Price: Number(19650), Volume: Number(3)},
		},
		Time:         snapshotTime,
		LastUpdateId: 101,
	})

	t.Logf("waiting for the next order update...")
	time.Sleep(500 * time.Millisecond)

	{
		orders := executor.orderStore.Orders()
		assert.Len(t, orders, 1, "should have 1 order in the order store")
	}

	t.Logf("emitting trade update...")
	userDataStream.EmitTradeUpdate(types.Trade{
		ID:            1,
		OrderID:       2,
		Exchange:      exchangeName,
		Price:         Number(19420.0),
		Quantity:      Number(100.0),
		QuoteQuantity: Number(100.0 * 19420.0),
		Symbol:        symbol,
		Side:          types.SideTypeBuy,
		IsBuyer:       true,
		IsMaker:       true,
		Time:          types.Time(secondSubmitOrderTime),
	})

	t.Logf("waiting for the trade callbacks...")
	time.Sleep(500 * time.Millisecond)

	executor.tradeCollector.Process()
	assert.Equal(t, Number(100), executor.position.GetBase())

	mockEx.EXPECT().CancelOrders(context.Background(), MatchOrder(secondOrder)).DoAndReturn(func(
		ctx context.Context, orders ...types.Order,
	) error {
		orderUpdate := secondOrder
		orderUpdate.Status = types.OrderStatusCanceled
		userDataStream.EmitOrderUpdate(orderUpdate)
		t.Logf("emit order #2 update: %+v", orderUpdate)
		return nil
	})
	assert.True(t, executor.cancelContextIfTargetQuantityFilled(), "target quantity should be filled")

	// finalizing and stop the executor
	select {
	case <-ctx.Done():
	case <-time.After(10 * time.Second):
	case <-executor.Done():
	}
	t.Logf("executor done")
}
