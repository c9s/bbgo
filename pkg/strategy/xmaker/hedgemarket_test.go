//go:build !dnum

package xmaker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/tradeid"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

func init() {
	tradeid.GlobalGenerator = tradeid.NewDeterministicGenerator()
}

func Test_NewHedgeMarket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	market := Market("BTCUSDT")
	depth := Number(100.0)
	session, marketDataStream, userDataStream := newMockSession(mockCtrl, ctx, market.Symbol)

	hm := NewHedgeMarket(&HedgeMarketConfig{
		SymbolSelector: "BTCUSDT",
		HedgeInterval:  hedgeInterval,
		QuotingDepth:   depth,
	}, session, market)

	// the connectivity waiting is blocking, so we need to run it in a goroutine
	go func() {
		err := hm.Start(ctx)
		assert.NoError(t, err)
	}()

	marketDataStream.EmitConnect()
	userDataStream.EmitConnect()

	time.Sleep(stepTime)

	cancel()

	hm.Stop(context.Background())

	assert.NotNil(t, hm)
	assert.NotNil(t, hm.session)
	assert.NotNil(t, hm.orderStore)
}

func TestHedgeMarket_Hedge(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	market := Market("BTCUSDT")

	session, marketDataStream, userDataStream := newMockSession(mockCtrl, ctx, market.Symbol)

	userDataStream.EmitConnect()
	userDataStream.EmitAuth()

	mockExchange := session.Exchange.(*mocks.MockExchangeExtended)

	submitOrder := types.SubmitOrder{
		Market:           market,
		Symbol:           market.Symbol,
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
	mockExchange.EXPECT().SubmitOrder(gomock.Any(), submitOrder).Return(&createdOrder, nil)

	depth := Number(100.0)
	hm := NewHedgeMarket(&HedgeMarketConfig{
		SymbolSelector: "BTCUSDT",
		HedgeInterval:  hedgeInterval,
		QuotingDepth:   depth,
		SyncOrder:      types.BoolPtr(false),
	}, session, market)

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

	hm.positionExposure.Open(Number(1.0))

	err = hm.hedge(context.Background())
	assert.NoError(t, err)
}

func TestHedgeMarket_GetBaseQuoteAvailableBalances(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	market := Market("BTCUSDT")
	session, _, _ := newMockSession(mockCtrl, ctx, market.Symbol)

	hm := NewHedgeMarket(&HedgeMarketConfig{
		SymbolSelector: "BTCUSDT",
		HedgeInterval:  hedgeInterval,
		QuotingDepth:   Number(1.0),
	}, session, market)

	// Satisfy the Connect call expectation from newMockSession
	err := hm.stream.Connect(ctx)
	assert.NoError(t, err)

	baseAvail, quoteAvail := hm.GetBaseQuoteAvailableBalances()
	assert.InEpsilon(t, 10.0, baseAvail.Float64(), 1e-9)
	assert.InEpsilon(t, 200000.0, quoteAvail.Float64(), 1e-9)
}

func TestHedgeMarket_GetQuotePriceBySessionBalances(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	market := Market("BTCUSDT")
	session, marketDataStream, _ := newMockSession(mockCtrl, ctx, market.Symbol)

	hm := NewHedgeMarket(&HedgeMarketConfig{
		SymbolSelector: "BTCUSDT",
		HedgeInterval:  hedgeInterval,
		QuotingDepth:   Number(1.0),
	}, session, market)

	// Connect underlying mock stream (expected by newMockSession)
	err := hm.stream.Connect(ctx)
	assert.NoError(t, err)

	// Provide a simple order book snapshot
	marketDataStream.EmitBookSnapshot(types.SliceOrderBook{
		Symbol: market.Symbol,
		Bids: types.PriceVolumeSlice{
			{Price: Number(10000), Volume: Number(100)},
		},
		Asks: types.PriceVolumeSlice{
			{Price: Number(10010), Volume: Number(100)},
		},
	})

	bid, ask := hm.GetQuotePriceBySessionBalances()

	// With zero taker fee rate by default, the prices should match raw computed levels
	assert.InEpsilon(t, 10000.0, bid.Float64(), 1e-9)
	assert.InEpsilon(t, 10010.0, ask.Float64(), 1e-9)
}

// fakeDepthQueryExchange wraps a mock exchange and additionally implements
// types.ExchangeDepthQueryService, so GetQuotePrice/GetQuotePriceBySessionBalances's
// refreshBookIfStale REST-poll fallback can be exercised in tests.
type fakeDepthQueryExchange struct {
	*mocks.MockExchangeExtended
	snapshot      types.SliceOrderBook
	queryDepthErr error
	queryDepthN   int
}

func (f *fakeDepthQueryExchange) QueryDepth(_ context.Context, _ string) (types.SliceOrderBook, int64, error) {
	f.queryDepthN++
	return f.snapshot, 0, f.queryDepthErr
}

func TestHedgeMarket_BindBookUpdateMonitor_PollsWhenStale(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	market := Market("BTCUSDT")
	_, mockMarketDataStream := newMockMarketDataStream(mockCtrl, ctx, market.Symbol)
	_, mockUserDataStream := newMockUserDataStream(mockCtrl)

	mockEx := mocks.NewMockExchangeExtended(mockCtrl)
	mockEx.EXPECT().NewStream().Return(mockMarketDataStream)
	mockEx.EXPECT().Name().Return(types.ExchangeBinance)

	fakeEx := &fakeDepthQueryExchange{
		MockExchangeExtended: mockEx,
		snapshot: types.SliceOrderBook{
			Symbol: market.Symbol,
			Bids:   types.PriceVolumeSlice{{Price: Number(20000), Volume: Number(1)}},
			Asks:   types.PriceVolumeSlice{{Price: Number(20010), Volume: Number(1)}},
		},
	}

	account := types.NewAccount()
	account.UpdateBalances(types.BalanceMap{
		"USDT": types.NewBalance("USDT", Number(200000)),
		"BTC":  types.NewBalance("BTC", Number(10)),
	})

	session := &bbgo.ExchangeSession{
		MarketDataStream: mockMarketDataStream,
		UserDataStream:   mockUserDataStream,
		Exchange:         fakeEx,
		Account:          account,
	}

	hm := NewHedgeMarket(&HedgeMarketConfig{
		SymbolSelector: "BTCUSDT",
		HedgeInterval:  hedgeInterval,
		QuotingDepth:   Number(1.0),
	}, session, market)

	hm.BindBookUpdateMonitor(&types.StreamBookMonitor{
		Enabled:        true,
		StaleThreshold: types.Duration(time.Hour),
	})

	err := hm.stream.Connect(ctx)
	assert.NoError(t, err)

	// no snapshot/update has ever arrived on the websocket stream, so the
	// book is stale by definition (see types.StreamBookMonitor.IsStale).
	bid, ask := hm.GetQuotePrice()

	assert.Equal(t, 1, fakeEx.queryDepthN, "expected a REST poll since the book never received a stream event")
	assert.InEpsilon(t, 20000.0, bid.Float64(), 1e-9)
	assert.InEpsilon(t, 20010.0, ask.Float64(), 1e-9)
}

func TestHedgeMarket_BindBookUpdateMonitor_NoPollWhenFresh(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	market := Market("BTCUSDT")
	session, marketDataStream, _ := newMockSession(mockCtrl, ctx, market.Symbol)

	hm := NewHedgeMarket(&HedgeMarketConfig{
		SymbolSelector: "BTCUSDT",
		HedgeInterval:  hedgeInterval,
		QuotingDepth:   Number(1.0),
	}, session, market)

	hm.BindBookUpdateMonitor(&types.StreamBookMonitor{
		Enabled:        true,
		StaleThreshold: types.Duration(time.Hour),
	})

	err := hm.stream.Connect(ctx)
	assert.NoError(t, err)

	marketDataStream.EmitBookSnapshot(types.SliceOrderBook{
		Symbol: market.Symbol,
		Bids:   types.PriceVolumeSlice{{Price: Number(10000), Volume: Number(100)}},
		Asks:   types.PriceVolumeSlice{{Price: Number(10010), Volume: Number(100)}},
	})

	// session.Exchange here (from newMockSession) does not implement
	// types.ExchangeDepthQueryService, so refreshBookIfStale is a no-op
	// regardless of staleness. This call mainly asserts GetQuotePrice still
	// behaves normally (reads the fresh snapshot) when bound to a monitor.
	bid, ask := hm.GetQuotePrice()
	assert.InEpsilon(t, 10000.0, bid.Float64(), 1e-9)
	assert.InEpsilon(t, 10010.0, ask.Float64(), 1e-9)
}

func TestCloneBookUpdateMonitor(t *testing.T) {
	template := &types.StreamBookMonitor{
		Enabled:        true,
		StaleThreshold: types.Duration(5 * time.Second),
	}

	clone1 := cloneBookUpdateMonitor(template)
	clone2 := cloneBookUpdateMonitor(template)

	// SplitHedge/SyntheticHedge clone this template once per HedgeMarket;
	// each must get its own instance since StreamBookMonitor.Bind holds
	// per-book mutable state (lastEventTime, the bound staleMetric).
	assert.NotSame(t, clone1, clone2)
	assert.Equal(t, template.Enabled, clone1.Enabled)
	assert.Equal(t, template.StaleThreshold, clone1.StaleThreshold)
	assert.Equal(t, template.Enabled, clone2.Enabled)
	assert.Equal(t, template.StaleThreshold, clone2.StaleThreshold)
}

func TestHedgeMarket_RedispatchPositionAfterFailure(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	market := Market("BTCUSDT")

	session, marketDataStream, userDataStream := newMockSession(mockCtrl, ctx, market.Symbol)
	_ = userDataStream

	mockExchange := session.Exchange.(*mocks.MockExchangeExtended)

	submitOrder := types.SubmitOrder{
		Market:           market,
		Symbol:           market.Symbol,
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
	retError := fmt.Errorf("failed to submit order due to network issue")
	mockExchange.EXPECT().SubmitOrder(gomock.Any(), submitOrder).Return(&createdOrder, retError)

	depth := Number(100.0)
	hm := NewHedgeMarket(&HedgeMarketConfig{
		SymbolSelector: "BTCUSDT",
		HedgeInterval:  hedgeInterval,
		QuotingDepth:   depth,
		SyncOrder:      types.BoolPtr(false),
	}, session, market)

	mainPosition := bbgo.NewPositionExposure("BTCUSDT")
	redispatchTriggered := false
	hm.OnRedispatchPosition(func(position fixedpoint.Value) {
		redispatchTriggered = true

		mainPosition.Open(position)
	})
	hm.positionExposure.OnClose(mainPosition.Close)

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

	delta := Number(1.0)
	mainPosition.Open(delta)
	mainPosition.Cover(delta) // mark as covered since it's dispatched to hedge market
	assert.Equal(t, Number(1.0), mainPosition.GetPending())
	assert.Equal(t, Number(1.0), mainPosition.GetNet())

	hm.positionExposure.Open(delta)

	err = hm.hedge(context.Background())
	assert.Error(t, err)

	<-hm.hedgedC
	assert.True(t, redispatchTriggered)

	assert.Equal(t, Number(0.0), hm.positionExposure.GetPending())
	assert.Equal(t, Number(0.0), hm.positionExposure.GetNet())
	assert.Equal(t, Number(0.0), hm.positionExposure.GetUncovered())

	assert.Equal(t, Number(0.0), mainPosition.GetPending(), "main position should have 0.0 pending after redispatch")
	assert.Equal(t, Number(1.0), mainPosition.GetNet(), "main position should still have 1.0 net")
	assert.Equal(t, Number(1.0), mainPosition.GetUncovered(), "main position should have 1.0 uncovered")
}

func TestHedgeMarket_startAndHedge(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	market := Market("BTCUSDT")

	session, marketDataStream, userDataStream := newMockSession(mockCtrl, ctx, market.Symbol)

	mockExchange := session.Exchange.(*mocks.MockExchangeExtended)

	depth := Number(100.0)
	// Use a large hedge interval so the ticker never fires during the test.
	// All hedging is driven by the positionDeltaC channel, which eliminates
	// the race between the ticker and the channel-driven hedge path.
	largeInterval := types.Duration(time.Hour)
	hm := NewHedgeMarket(&HedgeMarketConfig{
		SymbolSelector: "BTCUSDT",
		HedgeInterval:  largeInterval,
		QuotingDepth:   depth,
		SyncOrder:      types.BoolPtr(false),
	}, session, market)

	// the connectivity waiting is blocking, so we need to run it in a goroutine
	go func() {
		err := hm.Start(ctx)
		assert.NoError(t, err)
	}()

	userDataStream.EmitConnect()
	userDataStream.EmitAuth()

	marketDataStream.EmitConnect()

	submitOrder := types.SubmitOrder{
		Market:           market,
		Symbol:           market.Symbol,
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
	mockExchange.EXPECT().SubmitOrder(gomock.Any(), submitOrder).Return(&createdOrder, nil)

	submitOrder2 := types.SubmitOrder{
		Market:           market,
		Symbol:           market.Symbol,
		Quantity:         Number(1.0),
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeMarket,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
	}
	createdOrder2 := types.Order{
		OrderID:          2,
		SubmitOrder:      submitOrder2,
		ExecutedQuantity: Number(1.0),
		Status:           types.OrderStatusFilled,
	}
	mockExchange.EXPECT().SubmitOrder(gomock.Any(), submitOrder2).Return(&createdOrder2, nil)

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
	<-hm.hedgedC

	hm.positionDeltaC <- Number(1.0)
	<-hm.hedgedC

	// emit trades
	userDataStream.EmitTradeUpdate(types.Trade{
		ID:            1,
		OrderID:       1,
		Exchange:      types.ExchangeBinance,
		Price:         Number(103000.0),
		Quantity:      Number(1.0),
		QuoteQuantity: Number(103000.0 * 1.0),
		Symbol:        market.Symbol, // fixed to use market.Symbol
		Side:          types.SideTypeSell,
		IsBuyer:       false,
		Time:          types.Time{},
		Fee:           fixedpoint.Zero,
	})

	time.Sleep(stepTime)

	assert.Equal(t, Number(1.0), hm.positionExposure.GetPending())
	assert.Equal(t, Number(1.0), hm.positionExposure.GetNet())

	userDataStream.EmitTradeUpdate(types.Trade{
		ID:            2,
		OrderID:       1,
		Exchange:      types.ExchangeBinance,
		Price:         Number(103000.0),
		Quantity:      Number(1.0),
		QuoteQuantity: Number(103000.0 * 1.0),
		Symbol:        market.Symbol, // fixed to use market.Symbol
		Side:          types.SideTypeSell,
		IsBuyer:       false,
		Time:          types.Time{},
		Fee:           fixedpoint.Zero,
	})

	time.Sleep(stepTime)

	assert.Equal(t, Number(0.0), hm.positionExposure.GetPending())
	assert.Equal(t, Number(0.0), hm.positionExposure.GetNet())

	cancel()

	hm.Stop(context.Background())
}
