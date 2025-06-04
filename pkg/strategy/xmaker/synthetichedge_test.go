package xmaker

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestSyntheticHedge_GetQuotePrices_BaseQuote(t *testing.T) {
	// source: BTCUSDT, fiat: USDTTWD, source quote == fiat base
	sourceMarket := types.Market{Symbol: "BTCUSDT", BaseCurrency: "BTC", QuoteCurrency: "USDT"}
	fiatMarket := types.Market{Symbol: "USDTTWD", BaseCurrency: "USDT", QuoteCurrency: "TWD"}

	sourceBook := types.NewStreamBook("BTCUSDT", "")
	sourceBook.Load(types.SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids: types.PriceVolumeSlice{
			{Price: Number(10000), Volume: Number(1)},
		},
		Asks: types.PriceVolumeSlice{
			{Price: Number(10010), Volume: Number(1)},
		},
	})

	fiatBook := types.NewStreamBook("USDTTWD", "")
	fiatBook.Load(types.SliceOrderBook{
		Symbol: "USDTTWD",
		Bids: types.PriceVolumeSlice{
			{Price: Number(30), Volume: Number(1000)},
		},
		Asks: types.PriceVolumeSlice{
			{Price: Number(31), Volume: Number(1000)},
		},
	})

	source := &HedgeMarket{
		market:    sourceMarket,
		book:      sourceBook,
		depthBook: types.NewDepthBook(sourceBook, Number(1)),
	}
	fiat := &HedgeMarket{
		market:    fiatMarket,
		book:      fiatBook,
		depthBook: types.NewDepthBook(fiatBook, Number(1)),
	}

	hedge := &SyntheticHedge{
		sourceMarket: source,
		fiatMarket:   fiat,
	}

	bid, ask, ok := hedge.GetQuotePrices(fixedpoint.Zero)
	assert.True(t, ok)
	assert.Equal(t, Number(10000*30), bid)
	assert.Equal(t, Number(10010*31), ask)
}

func Test_newHedgeMarket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	market := Market("BTCUSDT")
	depth := Number(100.0)
	session, marketDataStream, userDataStream := newMockSession(mockCtrl, ctx, market.Symbol)
	_ = marketDataStream
	_ = userDataStream

	doneC := make(chan struct{})
	hm := newHedgeMarket(session, market, depth)
	go func() {
		err := hm.start(ctx, 3*time.Millisecond)
		assert.NoError(t, err)
		close(doneC)
	}()

	time.Sleep(10 * time.Millisecond)
	marketDataStream.EmitConnect()
	cancel()
	<-doneC

	assert.NotNil(t, hm)
	assert.NotNil(t, hm.session)
	assert.NotNil(t, hm.orderStore)
}

func TestHedgeMarket_hedge(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	market := Market("BTCUSDT")

	session, marketDataStream, userDataStream := newMockSession(mockCtrl, ctx, market.Symbol)
	_ = userDataStream

	mockExchange := session.Exchange.(*mocks.MockExchange)

	submitOrder := types.SubmitOrder{
		Market:   market,
		Symbol:   "BTCUSDT",
		Quantity: Number(1.0),
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeMarket,
	}
	createdOrder := types.Order{
		OrderID:          1,
		SubmitOrder:      submitOrder,
		ExecutedQuantity: Number(1.0),
		Status:           types.OrderStatusFilled,
	}
	mockExchange.EXPECT().SubmitOrder(gomock.Any(), submitOrder).Return(&createdOrder, nil)

	hm := newHedgeMarket(session, market, Number(100.0))

	err := hm.stream.Connect(ctx)
	assert.NoError(t, err)

	marketDataStream.EmitBookSnapshot(types.SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids: types.PriceVolumeSlice{
			{Price: Number(10000), Volume: Number(100)},
		},
		Asks: types.PriceVolumeSlice{
			{Price: Number(10010), Volume: Number(100)},
		},
	})

	err = hm.hedge(context.Background(), Number(1.0))
	assert.NoError(t, err)
}

func TestHedgeMarket_startAndHedge(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	market := Market("BTCUSDT")

	session, marketDataStream, userDataStream := newMockSession(mockCtrl, ctx, market.Symbol)
	_ = userDataStream

	mockExchange := session.Exchange.(*mocks.MockExchange)

	submitOrder := types.SubmitOrder{
		Market:   market,
		Symbol:   "BTCUSDT",
		Quantity: Number(2.0),
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeMarket,
	}
	createdOrder := types.Order{
		OrderID:          1,
		SubmitOrder:      submitOrder,
		ExecutedQuantity: Number(2.0),
		Status:           types.OrderStatusFilled,
	}
	mockExchange.EXPECT().SubmitOrder(gomock.Any(), submitOrder).Return(&createdOrder, nil)

	doneC := make(chan struct{})
	hm := newHedgeMarket(session, market, Number(100.0))
	go func() {
		err := hm.start(ctx, 40*time.Millisecond)
		assert.NoError(t, err)
		close(doneC)
	}()

	marketDataStream.EmitConnect()

	marketDataStream.EmitBookSnapshot(types.SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids: types.PriceVolumeSlice{
			{Price: Number(10000), Volume: Number(100)},
		},
		Asks: types.PriceVolumeSlice{
			{Price: Number(10010), Volume: Number(100)},
		},
	})

	hm.positionDeltaC <- Number(1.0)
	hm.positionDeltaC <- Number(1.0)

	time.Sleep(100 * time.Millisecond)

	// emit trades
	userDataStream.EmitTradeUpdate(types.Trade{
		ID:            1,
		OrderID:       1,
		Exchange:      types.ExchangeBinance,
		Price:         Number(103000.0),
		Quantity:      Number(1.0),
		QuoteQuantity: Number(103000.0 * 1.0),
		Symbol:        "BTCUSDT",
		Side:          types.SideTypeSell,
		IsBuyer:       false,
		Time:          types.Time{},
		Fee:           fixedpoint.Zero,
	})

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, Number(1.0), hm.positionExposure.pending.Get())
	assert.Equal(t, Number(1.0), hm.positionExposure.net.Get())

	userDataStream.EmitTradeUpdate(types.Trade{
		ID:            2,
		OrderID:       1,
		Exchange:      types.ExchangeBinance,
		Price:         Number(103000.0),
		Quantity:      Number(1.0),
		QuoteQuantity: Number(103000.0 * 1.0),
		Symbol:        "BTCUSDT",
		Side:          types.SideTypeSell,
		IsBuyer:       false,
		Time:          types.Time{},
		Fee:           fixedpoint.Zero,
	})

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, Number(0.0), hm.positionExposure.pending.Get())
	assert.Equal(t, Number(0.0), hm.positionExposure.net.Get())

	cancel()
	<-doneC
}

func TestSyntheticHedgeMarket_StartAndHedge(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	stepTime := 30 * time.Millisecond

	makerMarket := Market("BTCTWD")
	_ = makerMarket

	sourceMarket := Market("BTCUSDT")
	fiatMarket := Market("USDTTWD")

	sourceSession, sourceMarketDataStream, sourceUserDataStream := newMockSession(mockCtrl, ctx, sourceMarket.Symbol)
	sourceSession.SetMarkets(AllMarkets())
	sourceHedgeMarket := newHedgeMarket(sourceSession, sourceMarket, Number(100.0))
	sourceHedgeMarket.book.Load(types.SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids: types.PriceVolumeSlice{
			{Price: Number(104_000.), Volume: Number(1)},
		},
		Asks: types.PriceVolumeSlice{
			{Price: Number(104_050.0), Volume: Number(1)},
		},
	})

	fiatSession, fiatMarketDataStream, fiatUserDataStream := newMockSession(mockCtrl, ctx, fiatMarket.Symbol)
	fiatSession.SetMarkets(AllMarkets())
	fiatHedgeMarket := newHedgeMarket(fiatSession, fiatMarket, Number(10.0))
	fiatHedgeMarket.book.Load(types.SliceOrderBook{
		Symbol: "USDTTWD",
		Bids: types.PriceVolumeSlice{
			{Price: Number(30.1), Volume: Number(125000)},
		},
		Asks: types.PriceVolumeSlice{
			{Price: Number(30.0), Volume: Number(125000)},
		},
	})

	orderStore := core.NewOrderStore(makerMarket.Symbol)
	position := types.NewPositionFromMarket(makerMarket)
	strategy := &Strategy{
		makerSession: &bbgo.ExchangeSession{
			ExchangeName: types.ExchangeMax,
		},
		makerMarket:    makerMarket,
		orderStore:     orderStore,
		tradeCollector: core.NewTradeCollector(makerMarket.Symbol, position, orderStore),
	}

	syn := &SyntheticHedge{
		Enabled:            true,
		SourceSymbol:       "binance." + sourceMarket.Symbol,
		SourceDepthInQuote: Number(1000.0),
		FiatSymbol:         "max." + fiatMarket.Symbol,
		FiatDepthInQuote:   Number(30.0 * 1000.0),
		HedgeInterval:      types.Duration(10 * time.Millisecond),
		sourceMarket:       sourceHedgeMarket,
		fiatMarket:         fiatHedgeMarket,
		syntheticTradeId:   0,
		logger:             logrus.StandardLogger(),
	}
	err := syn.initialize(strategy)
	assert.NoError(t, err)

	doneC := make(chan struct{})

	go func() {
		err := syn.Start(ctx)
		assert.NoError(t, err)

		close(doneC)
	}()

	sourceMarketDataStream.EmitConnect()
	fiatMarketDataStream.EmitConnect()

	submitOrder := types.SubmitOrder{
		Market:   sourceMarket,
		Symbol:   "BTCUSDT",
		Quantity: Number(1.0),
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeMarket,
	}
	createdOrder := types.Order{
		OrderID:          1,
		SubmitOrder:      submitOrder,
		ExecutedQuantity: Number(1.0),
		Status:           types.OrderStatusFilled,
	}
	sourceSession.Exchange.(*mocks.MockExchange).EXPECT().SubmitOrder(gomock.Any(), submitOrder).Return(&createdOrder, nil)

	submitOrder2 := types.SubmitOrder{
		Market:   fiatMarket,
		Symbol:   "USDTTWD",
		Quantity: Number(104000.0),
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeMarket,
	}
	createdOrder2 := types.Order{
		OrderID:          2,
		SubmitOrder:      submitOrder2,
		ExecutedQuantity: Number(104000.0),
		Status:           types.OrderStatusFilled,
	}
	fiatSession.Exchange.(*mocks.MockExchange).EXPECT().SubmitOrder(gomock.Any(), submitOrder2).Return(&createdOrder2, nil)

	// add position to delta
	position.AddTrade(types.Trade{
		ID:            1,
		OrderID:       1,
		Exchange:      types.ExchangeMax,
		Price:         Number(3_110_000.0),
		Quantity:      Number(1.0),
		QuoteQuantity: Number(3_110_000.0 * 1.0),
		Symbol:        makerMarket.Symbol,
		Side:          types.SideTypeBuy,
		IsBuyer:       true,
		IsMaker:       true,
		Time:          types.Time{},
		Fee:           fixedpoint.Zero,
		FeeCurrency:   "",
	})
	assert.Equal(t, Number(1.0).Float64(), position.GetBase().Float64(), "make sure position is updated correctly")
	assert.Equal(t, Number(3_110_000.0).Float64(), position.GetAverageCost().Float64(), "make sure average cost is updated correctly")

	// send position delta to source hedge market
	sourceHedgeMarket.positionDeltaC <- Number(1.0)
	time.Sleep(stepTime)

	sourceUserDataStream.EmitTradeUpdate(types.Trade{
		ID:            createdOrder.OrderID,
		OrderID:       createdOrder.OrderID,
		Exchange:      createdOrder.Exchange,
		Price:         Number(104_000.0),
		Quantity:      Number(1.0),
		QuoteQuantity: Number(104000.0 * 1.0),
		Symbol:        createdOrder.Symbol,
		Side:          createdOrder.Side,
		IsBuyer:       createdOrder.Side == types.SideTypeBuy,
		IsMaker:       false,
		Time:          types.Time{},
		Fee:           fixedpoint.Zero,
		FeeCurrency:   "",
	})
	time.Sleep(stepTime)
	assert.Equal(t, Number(104000.0*1.0).Float64(), fiatHedgeMarket.position.Base.Float64(), "fiat position should be updated to the quote quantity")

	fiatUserDataStream.EmitTradeUpdate(types.Trade{
		ID:            createdOrder2.OrderID,
		OrderID:       createdOrder2.OrderID,
		Exchange:      createdOrder2.Exchange,
		Price:         Number(30.0),
		Quantity:      Number(104000.0),
		QuoteQuantity: Number(104000.0 * 30.0),
		Symbol:        createdOrder2.Symbol,
		Side:          createdOrder2.Side,
		IsBuyer:       createdOrder2.Side == types.SideTypeBuy,
		IsMaker:       false,
		Time:          types.Time{},
		Fee:           fixedpoint.Zero,
		FeeCurrency:   "",
	})
	time.Sleep(stepTime)
	assert.Equal(t, Number(0).Float64(), sourceHedgeMarket.position.Base.Float64(), "source position should be closed to 0")
	assert.Equal(t, Number(0).Float64(), fiatHedgeMarket.position.Base.Float64(), "fiat position should be closed to 0")
	assert.Equal(t, Number(0).Float64(), position.Base.Float64(), "the maker position should be closed to 0")

	cancel()
	<-doneC
}

func TestPositionExposure(t *testing.T) {
	pe := newPositionExposure("BTCUSDT")

	// initial value
	assert.Equal(t, Number(0), pe.net.Get())
	assert.Equal(t, Number(0), pe.pending.Get())
	assert.Equal(t, Number(0), pe.GetUncovered())

	// open position (maker orders are filled)
	pe.Open(Number(2))
	assert.Equal(t, Number(2), pe.net.Get())
	assert.Equal(t, Number(0), pe.pending.Get())
	assert.Equal(t, Number(2), pe.GetUncovered())

	// cover 1 (hedge order is placed, and pending is updated)
	pe.Cover(Number(1))
	assert.Equal(t, Number(2), pe.net.Get())
	assert.Equal(t, Number(1), pe.pending.Get())
	assert.Equal(t, Number(1), pe.GetUncovered())

	// close 1 (hedge order is filled, pending is updated)
	pe.Close(Number(-1))
	assert.Equal(t, Number(1), pe.net.Get())
	assert.Equal(t, Number(0), pe.pending.Get())
	assert.Equal(t, Number(1), pe.GetUncovered())

	// open 1 (another maker order is filled, pending is updated)
	pe.Open(Number(-1))
	assert.Equal(t, Number(0), pe.net.Get())
	assert.Equal(t, Number(0), pe.pending.Get())
	assert.Equal(t, Number(0), pe.GetUncovered())
}

// bindMockMarketDataStream binds default market data stream behaviors
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
	mockStream.EXPECT().OnDisconnect(Catch(func(x any) {
		stream.OnDisconnect(x.(func()))
	})).AnyTimes()
	mockStream.EXPECT().OnAuth(Catch(func(x any) {
		stream.OnAuth(x.(func()))
	})).AnyTimes()
}

// bindMockUserDataStream binds default user data stream behaviors
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

// newMockSession creates a mock ExchangeSession with a marketDataStream and userDataStream.
// marketDataStream and userDataStream are used to simulate the events
func newMockSession(
	mockCtrl *gomock.Controller, ctx context.Context, symbol string,
) (*bbgo.ExchangeSession, *types.StandardStream, *types.StandardStream) {
	// setup market data stream
	marketDataStream, mockMarketDataStream := newMockMarketDataStream(mockCtrl, ctx, symbol)

	// setup user data stream
	userDataStream, mockUserDataStream := newMockUserDataStream(mockCtrl)

	mockEx := mocks.NewMockExchange(mockCtrl)
	mockEx.EXPECT().NewStream().Return(mockMarketDataStream)
	mockEx.EXPECT().Name().Return(types.ExchangeBinance)

	mockBalances := types.BalanceMap{
		"USDT": types.NewBalance("USDT", Number(200000)),
		"BTC":  types.NewBalance("BTC", Number(10)),
	}

	account := types.NewAccount()
	account.UpdateBalances(mockBalances)

	session := &bbgo.ExchangeSession{
		MarketDataStream: mockMarketDataStream,
		UserDataStream:   mockUserDataStream,
		Exchange:         mockEx,
		Account:          account,
	}

	return session, marketDataStream, userDataStream
}

func newMockMarketDataStream(
	mockCtrl *gomock.Controller, ctx context.Context, symbol string,
) (*types.StandardStream, *mocks.MockStream) {
	marketDataStream := &types.StandardStream{}
	mockMarketDataStream := mocks.NewMockStream(mockCtrl)
	bindMockMarketDataStream(mockMarketDataStream, marketDataStream)

	// newHedgeMarket calls these methods
	mockMarketDataStream.EXPECT().SetPublicOnly()
	mockMarketDataStream.EXPECT().Subscribe(types.BookChannel, symbol, types.SubscribeOptions{
		Depth: types.DepthLevelFull,
	})

	mockMarketDataStream.EXPECT().Connect(gomock.AssignableToTypeOf(ctx))

	return marketDataStream, mockMarketDataStream
}

func newMockUserDataStream(mockCtrl *gomock.Controller) (*types.StandardStream, *mocks.MockStream) {
	userDataStream := &types.StandardStream{}
	mockUserDataStream := mocks.NewMockStream(mockCtrl)
	bindMockUserDataStream(mockUserDataStream, userDataStream)
	mockUserDataStream.OnAuth(func() {})
	return userDataStream, mockUserDataStream
}
