package twap

import (
	"context"
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

func TestNewStreamExecutor(t *testing.T) {
	symbol := "BTCUSDT"
	market := getTestMarket()

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
	mockMarketDataStream.EXPECT().OnBookSnapshot(Catch(func(x any) {
		marketDataStream.OnBookSnapshot(x.(func(book types.SliceOrderBook)))
	})).AnyTimes()
	mockMarketDataStream.EXPECT().OnBookUpdate(Catch(func(x any) {
		marketDataStream.OnBookUpdate(x.(func(book types.SliceOrderBook)))
	})).AnyTimes()
	mockMarketDataStream.EXPECT().OnConnect(Catch(func(x any) {
		marketDataStream.OnConnect(x.(func()))
	})).AnyTimes()
	mockMarketDataStream.EXPECT().Connect(gomock.AssignableToTypeOf(ctx))

	mockUserDataStream := mocks.NewMockStream(mockCtrl)
	mockUserDataStream.EXPECT().OnOrderUpdate(Catch(func(x any) {
		userDataStream.OnOrderUpdate(x.(func(order types.Order)))
	})).AnyTimes()
	mockUserDataStream.EXPECT().OnTradeUpdate(Catch(func(x any) {
		userDataStream.OnTradeUpdate(x.(func(order types.Trade)))
	})).AnyTimes()
	mockUserDataStream.EXPECT().OnConnect(Catch(func(x any) {
		userDataStream.OnConnect(x.(func()))
	})).AnyTimes()
	mockUserDataStream.EXPECT().OnAuth(Catch(func(x any) {
		userDataStream.OnAuth(x.(func()))
	}))
	mockUserDataStream.EXPECT().Connect(gomock.AssignableToTypeOf(ctx))

	mockEx.EXPECT().NewStream().Return(mockMarketDataStream)
	mockEx.EXPECT().NewStream().Return(mockUserDataStream)

	executor := NewStreamExecutor(mockEx, symbol, market, types.SideTypeBuy, Number(100), Number(1))
	executor.SetUpdateInterval(time.Second)

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

	select {
	case <-ctx.Done():
	case <-time.After(10 * time.Second):
	case <-executor.Done():
	}
	t.Logf("executor done")
}
