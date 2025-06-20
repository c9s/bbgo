package xmaker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/tradeid"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

func init() {
	tradeid.GlobalGenerator = tradeid.NewDeterministicGenerator()
}

func Test_newHedgeMarket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	market := testhelper.Market("BTCUSDT")
	depth := testhelper.Number(100.0)
	session, marketDataStream, userDataStream := newMockSession(mockCtrl, ctx, market.Symbol)
	_ = marketDataStream
	_ = userDataStream

	hm := newHedgeMarket(&HedgeMarketConfig{
		SymbolSelector: "BTCUSDT",
		HedgeInterval:  hedgeInterval,
		QuotingDepth:   depth,
	}, session, market)

	// the connectivity waiting is blocking, so we need to run it in a goroutine
	go func() {
		err := hm.Start(ctx)
		assert.NoError(t, err)
	}()

	time.Sleep(stepTime)
	marketDataStream.EmitConnect()
	cancel()

	hm.Stop(context.Background())

	assert.NotNil(t, hm)
	assert.NotNil(t, hm.session)
	assert.NotNil(t, hm.orderStore)
}

func TestHedgeMarket_hedge(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	market := testhelper.Market("BTCUSDT")

	session, marketDataStream, userDataStream := newMockSession(mockCtrl, ctx, market.Symbol)
	_ = userDataStream

	mockExchange := session.Exchange.(*mocks.MockExchangeExtended)

	submitOrder := types.SubmitOrder{
		Market:           market,
		Symbol:           market.Symbol,
		Quantity:         testhelper.Number(1.0),
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeMarket,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
	}
	createdOrder := types.Order{
		OrderID:          1,
		SubmitOrder:      submitOrder,
		ExecutedQuantity: testhelper.Number(1.0),
		Status:           types.OrderStatusFilled,
	}
	mockExchange.EXPECT().SubmitOrder(gomock.Any(), submitOrder).Return(&createdOrder, nil)

	depth := testhelper.Number(100.0)
	hm := newHedgeMarket(&HedgeMarketConfig{
		SymbolSelector: "BTCUSDT",
		HedgeInterval:  hedgeInterval,
		QuotingDepth:   depth,
	}, session, market)

	err := hm.stream.Connect(ctx)
	assert.NoError(t, err)

	marketDataStream.EmitBookSnapshot(types.SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids: types.PriceVolumeSlice{
			{Price: testhelper.Number(10000), Volume: testhelper.Number(100)},
		},
		Asks: types.PriceVolumeSlice{
			{Price: testhelper.Number(10010), Volume: testhelper.Number(100)},
		},
	})

	err = hm.hedge(context.Background(), testhelper.Number(1.0))
	assert.NoError(t, err)
}

func TestHedgeMarket_startAndHedge(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	market := testhelper.Market("BTCUSDT")

	session, marketDataStream, userDataStream := newMockSession(mockCtrl, ctx, market.Symbol)
	_ = userDataStream

	mockExchange := session.Exchange.(*mocks.MockExchangeExtended)

	depth := testhelper.Number(100.0)
	hm := newHedgeMarket(&HedgeMarketConfig{
		SymbolSelector: "BTCUSDT",
		HedgeInterval:  hedgeInterval,
		QuotingDepth:   depth,
	}, session, market)

	// the connectivity waiting is blocking, so we need to run it in a goroutine
	go func() {
		err := hm.Start(ctx)
		assert.NoError(t, err)
	}()

	submitOrder := types.SubmitOrder{
		Market:           market,
		Symbol:           market.Symbol,
		Quantity:         testhelper.Number(2.0),
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeMarket,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
	}
	createdOrder := types.Order{
		OrderID:          1,
		SubmitOrder:      submitOrder,
		ExecutedQuantity: testhelper.Number(2.0),
		Status:           types.OrderStatusFilled,
	}
	mockExchange.EXPECT().SubmitOrder(gomock.Any(), submitOrder).Return(&createdOrder, nil)

	marketDataStream.EmitConnect()

	marketDataStream.EmitBookSnapshot(types.SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids: types.PriceVolumeSlice{
			{Price: testhelper.Number(10000), Volume: testhelper.Number(100)},
		},
		Asks: types.PriceVolumeSlice{
			{Price: testhelper.Number(10010), Volume: testhelper.Number(100)},
		},
	})

	hm.positionDeltaC <- testhelper.Number(1.0)
	hm.positionDeltaC <- testhelper.Number(1.0)

	time.Sleep(stepTime)

	// emit trades
	userDataStream.EmitTradeUpdate(types.Trade{
		ID:            1,
		OrderID:       1,
		Exchange:      types.ExchangeBinance,
		Price:         testhelper.Number(103000.0),
		Quantity:      testhelper.Number(1.0),
		QuoteQuantity: testhelper.Number(103000.0 * 1.0),
		Symbol:        market.Symbol, // fixed to use market.Symbol
		Side:          types.SideTypeSell,
		IsBuyer:       false,
		Time:          types.Time{},
		Fee:           fixedpoint.Zero,
	})

	time.Sleep(stepTime)

	assert.Equal(t, testhelper.Number(1.0), hm.positionExposure.pending.Get())
	assert.Equal(t, testhelper.Number(1.0), hm.positionExposure.net.Get())

	userDataStream.EmitTradeUpdate(types.Trade{
		ID:            2,
		OrderID:       1,
		Exchange:      types.ExchangeBinance,
		Price:         testhelper.Number(103000.0),
		Quantity:      testhelper.Number(1.0),
		QuoteQuantity: testhelper.Number(103000.0 * 1.0),
		Symbol:        market.Symbol, // fixed to use market.Symbol
		Side:          types.SideTypeSell,
		IsBuyer:       false,
		Time:          types.Time{},
		Fee:           fixedpoint.Zero,
	})

	time.Sleep(stepTime)
	assert.Equal(t, testhelper.Number(0.0), hm.positionExposure.pending.Get())
	assert.Equal(t, testhelper.Number(0.0), hm.positionExposure.net.Get())

	cancel()

	hm.Stop(context.Background())
}
