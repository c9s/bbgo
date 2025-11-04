//go:build !dnum

package xmaker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

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
	}, session, market)

	mainPosition := NewPositionExposure("BTCUSDT")
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
	assert.Equal(t, Number(1.0), mainPosition.pending.Get())
	assert.Equal(t, Number(1.0), mainPosition.net.Get())

	hm.positionExposure.Open(delta)

	err = hm.hedge(context.Background())
	assert.Error(t, err)

	<-hm.hedgedC
	assert.True(t, redispatchTriggered)

	assert.Equal(t, Number(0.0), hm.positionExposure.pending.Get())
	assert.Equal(t, Number(0.0), hm.positionExposure.net.Get())
	assert.Equal(t, Number(0.0), hm.positionExposure.GetUncovered())

	assert.Equal(t, Number(0.0), mainPosition.pending.Get(), "main position should have 0.0 pending after redispatch")
	assert.Equal(t, Number(1.0), mainPosition.net.Get(), "main position should still have 1.0 net")
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

	assert.Equal(t, Number(1.0), hm.positionExposure.pending.Get())
	assert.Equal(t, Number(1.0), hm.positionExposure.net.Get())

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

	assert.Equal(t, Number(0.0), hm.positionExposure.pending.Get())
	assert.Equal(t, Number(0.0), hm.positionExposure.net.Get())

	cancel()

	hm.Stop(context.Background())
}
