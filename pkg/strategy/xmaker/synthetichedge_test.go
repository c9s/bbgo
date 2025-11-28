package xmaker

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/xmaker/pricer"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/tradeid"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

var stepTime = 30 * time.Millisecond
var hedgeInterval = types.Duration(10 * time.Millisecond)

func init() {
	tradeid.GlobalGenerator = tradeid.NewDeterministicGenerator()
}

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
		HedgeMarketConfig: &HedgeMarketConfig{
			QuotingDepth: Number(1.0),
		},
		market:    sourceMarket,
		book:      sourceBook,
		depthBook: types.NewDepthBook(sourceBook),
		bidPricer: pricer.Compose(
			pricer.FromBestPrice(types.SideTypeBuy, sourceBook),
		),
		askPricer: pricer.Compose(
			pricer.FromBestPrice(types.SideTypeSell, sourceBook),
		),
	}
	fiat := &HedgeMarket{
		HedgeMarketConfig: &HedgeMarketConfig{
			QuotingDepth: Number(1.0),
		},
		market:    fiatMarket,
		book:      fiatBook,
		depthBook: types.NewDepthBook(fiatBook),
		bidPricer: pricer.Compose(
			pricer.FromBestPrice(types.SideTypeBuy, fiatBook),
		),
		askPricer: pricer.Compose(
			pricer.FromBestPrice(types.SideTypeSell, fiatBook),
		),
	}

	hedge := &SyntheticHedge{
		sourceMarket: source,
		fiatMarket:   fiat,
		forward:      true,
	}

	bid, ask, ok := hedge.GetQuotePrices()
	assert.True(t, ok)
	assert.Equal(t, Number(10000).Mul(Number(30)), bid)
	assert.Equal(t, Number(10010).Mul(Number(31)), ask)
}

func TestSyntheticHedge_MarketOrderHedge(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	makerMarket := Market("BTCTWD")
	sourceMarket := Market("BTCUSDT")
	fiatMarket := Market("USDTTWD")

	sourceSession, sourceMarketDataStream, sourceUserDataStream := newMockSession(mockCtrl, ctx, sourceMarket.Symbol)
	sourceSession.SetMarkets(AllMarkets())

	sourceHedgeMarket := NewHedgeMarket(&HedgeMarketConfig{
		SymbolSelector: sourceMarket.Symbol,
		HedgeInterval:  hedgeInterval,
		QuotingDepth:   Number(100.0),
	}, sourceSession, sourceMarket)

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
	fiatHedgeMarket := NewHedgeMarket(&HedgeMarketConfig{
		SymbolSelector: fiatMarket.Symbol,
		HedgeInterval:  hedgeInterval,
		QuotingDepth:   Number(10.0),
	}, fiatSession, fiatMarket)

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
	makerPosition := types.NewPositionFromMarket(makerMarket)
	strategy := &Strategy{
		makerSession: &bbgo.ExchangeSession{
			ExchangeSessionConfig: bbgo.ExchangeSessionConfig{
				ExchangeName: types.ExchangeMax,
			},
		},
		makerMarket:    makerMarket,
		orderStore:     orderStore,
		tradeCollector: core.NewTradeCollector(makerMarket.Symbol, makerPosition, orderStore),
	}

	syn := &SyntheticHedge{
		Enabled: true,
		Source: &HedgeMarketConfig{
			SymbolSelector: "binance." + sourceMarket.Symbol,
			QuotingDepth:   Number(10.0),
			HedgeInterval:  hedgeInterval,
		},
		Fiat: &HedgeMarketConfig{
			SymbolSelector: "max." + fiatMarket.Symbol,
			QuotingDepth:   Number(30.0 * 1000.0),
			HedgeInterval:  hedgeInterval,
		},
		sourceMarket:     sourceHedgeMarket,
		fiatMarket:       fiatHedgeMarket,
		syntheticTradeId: 0,
		logger:           logrus.StandardLogger(),
		forward:          true,
	}
	err := syn.initialize(strategy)
	assert.NoError(t, err)

	// the connectivity waiting is blocking, so we need to run it in a goroutine

	go func() {
		err := syn.Start(ctx)
		assert.NoError(t, err)
	}()

	sourceMarketDataStream.EmitConnect()
	sourceUserDataStream.EmitConnect()
	sourceUserDataStream.EmitAuth()

	fiatMarketDataStream.EmitConnect()
	fiatUserDataStream.EmitConnect()
	fiatUserDataStream.EmitAuth()

	submitOrder := types.SubmitOrder{
		Market:           sourceMarket,
		Symbol:           "BTCUSDT",
		Quantity:         Number(1.0),
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeMarket,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
	}
	createdOrder := types.Order{
		OrderID:          1,
		SubmitOrder:      submitOrder,
		ExecutedQuantity: Number(1.0),
		Status:           types.OrderStatusFilled,
	}
	sourceSession.Exchange.(*mocks.MockExchangeExtended).EXPECT().SubmitOrder(gomock.Any(), submitOrder).Return(&createdOrder, nil).Times(1)

	submitOrder2 := types.SubmitOrder{
		Market:           fiatMarket,
		Symbol:           "USDTTWD",
		Quantity:         Number(104000.0),
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeMarket,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
	}
	createdOrder2 := types.Order{
		OrderID:          2,
		SubmitOrder:      submitOrder2,
		ExecutedQuantity: Number(104000.0),
		Status:           types.OrderStatusFilled,
	}
	fiatSession.Exchange.(*mocks.MockExchangeExtended).EXPECT().SubmitOrder(gomock.Any(), submitOrder2).Return(&createdOrder2, nil).Times(1)

	// add position to delta
	makerPosition.AddTrade(types.Trade{
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
	assert.Equal(t, Number(1.0).Float64(), makerPosition.GetBase().Float64(), "make sure position is updated correctly")
	assert.Equal(t, Number(3_110_000.0).Float64(), makerPosition.GetAverageCost().Float64(), "make sure average cost is updated correctly")

	// TRIGGER: send position delta to source hedge market
	sourceHedgeMarket.positionDeltaC <- Number(1.0)
	time.Sleep(stepTime)
	<-sourceHedgeMarket.hedgedC

	sourceUserDataStream.EmitTradeUpdate(types.Trade{
		ID:            createdOrder.OrderID,
		OrderID:       createdOrder.OrderID,
		Exchange:      createdOrder.Exchange,
		Price:         Number(104_000.0),
		Quantity:      createdOrder.Quantity,
		QuoteQuantity: Number(104000.0).Mul(createdOrder.Quantity),
		Symbol:        createdOrder.Symbol,
		Side:          createdOrder.Side,
		IsBuyer:       createdOrder.Side == types.SideTypeBuy,
		IsMaker:       false,
		Time:          types.Time{},
		Fee:           fixedpoint.Zero,
		FeeCurrency:   "",
	})
	time.Sleep(stepTime)
	assert.Equal(t, Number(104000.0*1.0).Float64(), fiatHedgeMarket.Position.Base.Float64(), "fiat position should be updated to the quote quantity")

	fiatUserDataStream.EmitTradeUpdate(types.Trade{
		ID:            createdOrder2.OrderID,
		OrderID:       createdOrder2.OrderID,
		Exchange:      createdOrder2.Exchange,
		Price:         Number(30.0),
		Quantity:      createdOrder2.Quantity,
		QuoteQuantity: Number(30.0).Mul(createdOrder2.Quantity),
		Symbol:        createdOrder2.Symbol,
		Side:          createdOrder2.Side,
		IsBuyer:       createdOrder2.Side == types.SideTypeBuy,
		IsMaker:       false,
		Time:          types.Time{},
		Fee:           fixedpoint.Zero,
		FeeCurrency:   "",
	})
	time.Sleep(stepTime)
	<-fiatHedgeMarket.hedgedC
	assert.Equal(t, Number(0).Float64(), sourceHedgeMarket.Position.GetBase().Float64(), "source position should be closed to 0")
	assert.Equal(t, Number(0).Float64(), fiatHedgeMarket.Position.GetBase().Float64(), "fiat position should be closed to 0")
	assert.Equal(t, Number(0).Float64(), makerPosition.GetBase().Float64(), "the maker position should be closed to 0")

	cancel()
}

func TestSyntheticHedge_CounterpartyOrderHedge(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	makerMarket := Market("BTCTWD")
	sourceMarket := Market("BTCUSDT")
	fiatMarket := Market("USDTTWD")

	sourceSession, sourceMarketDataStream, sourceUserDataStream := newMockSession(mockCtrl, ctx, sourceMarket.Symbol)
	sourceSession.SetMarkets(AllMarkets())

	sourceHedgeMarket := NewHedgeMarket(&HedgeMarketConfig{
		SymbolSelector: sourceMarket.Symbol,
		HedgeInterval:  hedgeInterval,
		QuotingDepth:   Number(100.0),
		HedgeMethod:    HedgeMethodCounterparty,
		HedgeMethodCounterparty: &CounterpartyHedgeExecutorConfig{
			PriceLevel: 1,
		},
	}, sourceSession, sourceMarket)

	sourceHedgeMarket.book.Load(types.SliceOrderBook{
		Symbol: "BTCUSDT",
		Asks: types.PriceVolumeSlice{
			{Price: Number(104_050.0), Volume: Number(2)},
			{Price: Number(104_060.0), Volume: Number(1)},
		},
		Bids: types.PriceVolumeSlice{
			{Price: Number(104_000.), Volume: Number(1)},
			{Price: Number(103_990.), Volume: Number(2)},
		},
	})

	fiatSession, fiatMarketDataStream, fiatUserDataStream := newMockSession(mockCtrl, ctx, fiatMarket.Symbol)
	fiatSession.SetMarkets(AllMarkets())
	fiatHedgeMarket := NewHedgeMarket(&HedgeMarketConfig{
		SymbolSelector: fiatMarket.Symbol,
		HedgeInterval:  hedgeInterval,
		QuotingDepth:   Number(10.0),
		HedgeMethod:    HedgeMethodCounterparty,
		HedgeMethodCounterparty: &CounterpartyHedgeExecutorConfig{
			PriceLevel: 1,
		},
	}, fiatSession, fiatMarket)

	fiatHedgeMarket.book.Load(types.SliceOrderBook{
		Symbol: "USDTTWD",
		Bids: types.PriceVolumeSlice{
			{Price: Number(30.0), Volume: Number(100_000)},
		},
		Asks: types.PriceVolumeSlice{
			{Price: Number(30.1), Volume: Number(100_000)},
		},
	})

	orderStore := core.NewOrderStore(makerMarket.Symbol)
	makerPosition := types.NewPositionFromMarket(makerMarket)
	strategy := &Strategy{
		makerSession: &bbgo.ExchangeSession{
			ExchangeSessionConfig: bbgo.ExchangeSessionConfig{
				ExchangeName: types.ExchangeMax,
			},
		},
		makerMarket:    makerMarket,
		orderStore:     orderStore,
		tradeCollector: core.NewTradeCollector(makerMarket.Symbol, makerPosition, orderStore),
	}

	syn := &SyntheticHedge{
		Enabled: true,
		Source: &HedgeMarketConfig{
			SymbolSelector: "binance." + sourceMarket.Symbol,
			QuotingDepth:   Number(10.0),
			HedgeInterval:  hedgeInterval,
			HedgeMethod:    HedgeMethodCounterparty,
			HedgeMethodCounterparty: &CounterpartyHedgeExecutorConfig{
				PriceLevel: 1,
			},
		},
		Fiat: &HedgeMarketConfig{
			SymbolSelector: "max." + fiatMarket.Symbol,
			QuotingDepth:   Number(30.0 * 1000.0),
			HedgeInterval:  hedgeInterval,
			HedgeMethod:    HedgeMethodCounterparty,
			HedgeMethodCounterparty: &CounterpartyHedgeExecutorConfig{
				PriceLevel: 1,
			},
		},
		sourceMarket:     sourceHedgeMarket,
		fiatMarket:       fiatHedgeMarket,
		syntheticTradeId: 0,
		logger:           logrus.StandardLogger(),
		forward:          true,
	}
	err := syn.initialize(strategy)
	assert.NoError(t, err)

	// the connectivity waiting is blocking, so we need to run it in a goroutine
	go func() {
		err := syn.Start(ctx)
		assert.NoError(t, err)
	}()

	sourceMarketDataStream.EmitConnect()
	sourceUserDataStream.EmitConnect()
	sourceUserDataStream.EmitAuth()

	fiatMarketDataStream.EmitConnect()
	fiatUserDataStream.EmitConnect()
	fiatUserDataStream.EmitAuth()

	// add position to delta
	makerPosition.AddTrade(types.Trade{
		ID:            99,
		OrderID:       99,
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

	// Make sure the position is added correctly
	assert.Equal(t, Number(1.0).Float64(), makerPosition.GetBase().Float64(), "make sure position is updated correctly")
	assert.Equal(t, Number(3_110_000.0).Float64(), makerPosition.GetAverageCost().Float64(), "make sure average cost is updated correctly")

	// The first hedge order
	submitOrder1 := types.SubmitOrder{
		Market:   sourceMarket,
		Symbol:   "BTCUSDT",
		Quantity: Number(1.0),
		Price:    Number(104000.0),
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
	}
	createdOrder1 := types.Order{
		OrderID:          1,
		SubmitOrder:      submitOrder1,
		ExecutedQuantity: Number(0.4),
		Status:           types.OrderStatusNew,
	}
	sourceSession.Exchange.(*mocks.MockExchangeExtended).EXPECT().SubmitOrder(gomock.Any(), submitOrder1).Return(&createdOrder1, nil)

	// Since it's not fully filled, we expect a cancel and a query later to check the filled quantity
	sourceSession.Exchange.(*mocks.MockExchangeExtended).EXPECT().CancelOrders(gomock.Any(), createdOrder1).Return(nil)
	sourceSession.Exchange.(*mocks.MockExchangeExtended).EXPECT().
		QueryOrder(gomock.Any(), createdOrder1.AsQuery()).
		Return(&types.Order{
			OrderID:          createdOrder1.OrderID,
			SubmitOrder:      submitOrder1,
			ExecutedQuantity: Number(0.4),
			Status:           types.OrderStatusCanceled,
		}, nil)

	// The second hedge order for the remaining quantity
	submitOrder2 := types.SubmitOrder{
		Market:   sourceMarket,
		Symbol:   "BTCUSDT",
		Quantity: Number(0.6),
		Price:    Number(104000.0),
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
	}
	createdOrder2 := types.Order{
		OrderID:          2,
		SubmitOrder:      submitOrder2,
		ExecutedQuantity: Number(0.0),
		Status:           types.OrderStatusNew,
	}
	sourceSession.Exchange.(*mocks.MockExchangeExtended).EXPECT().SubmitOrder(gomock.Any(), submitOrder2).Return(&createdOrder2, nil)

	// Expect a cancel and a query for the second order as well
	sourceSession.Exchange.(*mocks.MockExchangeExtended).EXPECT().CancelOrders(gomock.Any(), createdOrder2).Return(nil)
	sourceSession.Exchange.(*mocks.MockExchangeExtended).EXPECT().QueryOrder(gomock.Any(), createdOrder2.AsQuery()).Return(&types.Order{
		OrderID:          createdOrder2.OrderID,
		SubmitOrder:      submitOrder2,
		ExecutedQuantity: Number(0.6),
		Status:           types.OrderStatusCanceled,
	}, nil)

	// TRIGGER: send position delta to source hedge market
	sourceHedgeMarket.positionDeltaC <- Number(1.0)

	// Wait for the hedge done, 2 orders
	<-sourceHedgeMarket.hedgedC
	time.Sleep(stepTime)

	// fiatHedgeMarket should receive a position update later after sourceHedgeMarket sent a hedge order
	// then this should trigger a hedge order to the fiat market
	submitOrder4 := types.SubmitOrder{
		Market:   fiatMarket,
		Symbol:   "USDTTWD",
		Quantity: Number(104000.0 * 0.4),
		Price:    Number(30.0),
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
	}
	createdOrder4 := types.Order{
		OrderID:          4,
		SubmitOrder:      submitOrder4,
		ExecutedQuantity: Number(104000.0 * 0.4),
		Status:           types.OrderStatusFilled,
	}
	fiatSession.Exchange.(*mocks.MockExchangeExtended).EXPECT().SubmitOrder(gomock.Any(), submitOrder4).Return(&createdOrder4, nil)
	fiatSession.Exchange.(*mocks.MockExchangeExtended).EXPECT().QueryOrder(gomock.Any(), createdOrder4.AsQuery()).Return(&types.Order{
		OrderID:          createdOrder4.OrderID,
		SubmitOrder:      submitOrder4,
		ExecutedQuantity: Number(104000.0 * 0.4),
		Status:           types.OrderStatusFilled,
	}, nil)

	// TRIGGER the fiat market hedge
	sourceUserDataStream.EmitTradeUpdate(types.Trade{
		ID:            createdOrder1.OrderID,
		OrderID:       createdOrder1.OrderID,
		Exchange:      createdOrder1.Exchange,
		Price:         Number(104_000.0),
		Quantity:      Number(0.4),
		QuoteQuantity: Number(104000.0 * 0.4),
		Symbol:        createdOrder1.Symbol,
		Side:          createdOrder1.Side,
		IsBuyer:       createdOrder1.Side == types.SideTypeBuy,
		IsMaker:       false,
		Time:          types.Time{},
		Fee:           fixedpoint.Zero,
		FeeCurrency:   "",
	})
	sourceUserDataStream.EmitOrderUpdate(types.Order{
		OrderID:          1,
		SubmitOrder:      submitOrder1,
		ExecutedQuantity: Number(0.4),
		Status:           types.OrderStatusPartiallyFilled,
	})
	sourceUserDataStream.EmitOrderUpdate(types.Order{
		OrderID:          1,
		SubmitOrder:      submitOrder1,
		ExecutedQuantity: Number(0.4),
		Status:           types.OrderStatusCanceled,
	})

	fiatUserDataStream.EmitTradeUpdate(types.Trade{
		ID:            createdOrder4.OrderID,
		OrderID:       createdOrder4.OrderID,
		Exchange:      createdOrder4.Exchange,
		Price:         Number(30.0),
		Quantity:      Number(104000.0 * 0.4), // 40% of the source hedge order
		QuoteQuantity: Number(104000.0 * 0.4 * 30.0),
		Symbol:        createdOrder4.Symbol,
		Side:          createdOrder4.Side,
		IsBuyer:       createdOrder4.Side == types.SideTypeBuy,
		IsMaker:       false,
		Time:          types.Time{},
		Fee:           fixedpoint.Zero,
		FeeCurrency:   "",
	})

	<-fiatHedgeMarket.hedgedC
	time.Sleep(stepTime)

	assert.Equal(t, Number(0.).Float64(), sourceHedgeMarket.Position.GetBase().Float64(), "source position should be closed to 0.5")
	assert.Equal(t, Number(0.).Float64(), fiatHedgeMarket.Position.GetBase().Float64(), "fiat position should be updated to the quote quantity")

	submitOrder5 := types.SubmitOrder{
		Market:   fiatMarket,
		Symbol:   "USDTTWD",
		Quantity: Number(104000.0 * 0.6),
		Price:    Number(30.0),
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
	}
	createdOrder5 := types.Order{
		OrderID:          5,
		SubmitOrder:      submitOrder5,
		ExecutedQuantity: Number(104000.0 * 0.6),
		Status:           types.OrderStatusFilled,
	}
	fiatSession.Exchange.(*mocks.MockExchangeExtended).EXPECT().SubmitOrder(gomock.Any(), submitOrder5).Return(&createdOrder5, nil)
	fiatSession.Exchange.(*mocks.MockExchangeExtended).EXPECT().QueryOrder(gomock.Any(), createdOrder5.AsQuery()).Return(&types.Order{
		OrderID:          5,
		SubmitOrder:      submitOrder5,
		ExecutedQuantity: Number(104000.0 * 0.6),
		Status:           types.OrderStatusCanceled,
	}, nil)

	// TRIGGER
	sourceUserDataStream.EmitTradeUpdate(types.Trade{
		ID:            createdOrder2.OrderID,
		OrderID:       createdOrder2.OrderID,
		Exchange:      createdOrder2.Exchange,
		Price:         Number(104_000.0),
		Quantity:      Number(0.6),
		QuoteQuantity: Number(104000.0 * 0.6),
		Symbol:        createdOrder1.Symbol,
		Side:          createdOrder1.Side,
		IsBuyer:       createdOrder1.Side == types.SideTypeBuy,
		IsMaker:       false,
		Time:          types.Time{},
		Fee:           fixedpoint.Zero,
		FeeCurrency:   "",
	})
	sourceUserDataStream.EmitOrderUpdate(types.Order{
		OrderID:          createdOrder2.OrderID,
		SubmitOrder:      submitOrder1,
		ExecutedQuantity: Number(0.6),
		Status:           types.OrderStatusPartiallyFilled,
	})

	sourceUserDataStream.EmitOrderUpdate(types.Order{
		OrderID:          createdOrder2.OrderID,
		SubmitOrder:      submitOrder1,
		ExecutedQuantity: Number(0.6),
		Status:           types.OrderStatusCanceled,
	})

	fiatUserDataStream.EmitTradeUpdate(types.Trade{
		ID:            createdOrder5.OrderID,
		OrderID:       createdOrder5.OrderID,
		Exchange:      createdOrder5.Exchange,
		Price:         Number(30.0),
		Quantity:      Number(104000.0 * 0.6), // 40% of the source hedge order
		QuoteQuantity: Number(104000.0 * 0.6 * 30.0),
		Symbol:        createdOrder5.Symbol,
		Side:          createdOrder5.Side,
		IsBuyer:       createdOrder5.Side == types.SideTypeBuy,
		IsMaker:       false,
		Time:          types.Time{},
		Fee:           fixedpoint.Zero,
		FeeCurrency:   "",
	})

	time.Sleep(stepTime)

	cancel()
	time.Sleep(stepTime)

	assert.Equal(t, Number(0).Float64(), sourceHedgeMarket.Position.GetBase().Float64(), "source position should be closed to 0")
	assert.Equal(t, Number(0).Float64(), fiatHedgeMarket.Position.GetBase().Float64(), "fiat position should be closed to 0")
	assert.Equal(t, Number(0).Float64(), makerPosition.GetBase().Float64(), "the maker position should be closed to 0")
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

	mockEx := mocks.NewMockExchangeExtended(mockCtrl)
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

	// NewHedgeMarket calls these methods
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
