//go:build !dnum

package xmaker

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/xmaker/pricer"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

func newBookHedgeMarket(market types.Market, book types.SliceOrderBook) *HedgeMarket {
	streamBook := types.NewStreamBook(market.Symbol, "")
	streamBook.Load(book)
	return &HedgeMarket{
		HedgeMarketConfig: &HedgeMarketConfig{
			QuotingDepth: Number(1.0),
		},
		market:    market,
		book:      streamBook,
		depthBook: types.NewDepthBook(streamBook),
		bidPricer: pricer.Compose(pricer.FromBestPrice(types.SideTypeBuy, streamBook)),
		askPricer: pricer.Compose(pricer.FromBestPrice(types.SideTypeSell, streamBook)),
	}
}

func TestSyntheticHedge_GetQuotePrices_SplitSource(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	sourceMarket := Market("BTCUSDT")

	s1, md1, _ := newMockSession(mockCtrl, ctx, sourceMarket.Symbol)
	s2, md2, _ := newMockSession(mockCtrl, ctx, sourceMarket.Symbol)
	s1.Account.UpdateBalances(types.BalanceMap{
		"BTC":  types.NewBalance("BTC", Number(10)),
		"USDT": types.NewBalance("USDT", Number(10000)),
	})
	s2.Account.UpdateBalances(types.BalanceMap{
		"BTC":  types.NewBalance("BTC", Number(1)),
		"USDT": types.NewBalance("USDT", Number(200)),
	})

	hm1 := NewHedgeMarket(&HedgeMarketConfig{SymbolSelector: sourceMarket.Symbol, HedgeInterval: hedgeInterval, QuotingDepth: Number(1)}, s1, sourceMarket)
	hm2 := NewHedgeMarket(&HedgeMarketConfig{SymbolSelector: sourceMarket.Symbol, HedgeInterval: hedgeInterval, QuotingDepth: Number(1)}, s2, sourceMarket)
	assert.NoError(t, hm1.stream.Connect(ctx))
	assert.NoError(t, hm2.stream.Connect(ctx))

	md1.EmitBookSnapshot(types.SliceOrderBook{
		Symbol: sourceMarket.Symbol,
		Bids:   types.PriceVolumeSlice{{Price: Number(10000), Volume: Number(100)}},
		Asks:   types.PriceVolumeSlice{{Price: Number(10010), Volume: Number(100)}},
	})
	md2.EmitBookSnapshot(types.SliceOrderBook{
		Symbol: sourceMarket.Symbol,
		Bids:   types.PriceVolumeSlice{{Price: Number(9990), Volume: Number(100)}},
		Asks:   types.PriceVolumeSlice{{Price: Number(10020), Volume: Number(100)}},
	})

	fiat := newBookHedgeMarket(Market("USDTTWD"), types.SliceOrderBook{
		Symbol: "USDTTWD",
		Bids:   types.PriceVolumeSlice{{Price: Number(30), Volume: Number(1000)}},
		Asks:   types.PriceVolumeSlice{{Price: Number(31), Volume: Number(1000)}},
	})

	split := &SplitHedge{
		Enabled: true,
		Algo:    SplitHedgeAlgoProportion,
		hedgeMarketInstances: map[string]*HedgeMarket{
			"m1": hm1,
			"m2": hm2,
		},
		logger: logrus.New(),
	}

	hedge := &SyntheticHedge{
		Enabled:     true,
		SplitSource: split,
		fiatMarket:  fiat,
		forward:     true,
	}

	sourceBid, sourceAsk, ok := split.GetBalanceWeightedQuotePrice()
	assert.True(t, ok)

	// the weighted source prices should be in between the source books
	assert.True(t, sourceBid.Compare(Number(9990)) > 0 && sourceBid.Compare(Number(10000)) < 0, "weighted bid %s", sourceBid.String())
	assert.True(t, sourceAsk.Compare(Number(10010)) > 0 && sourceAsk.Compare(Number(10020)) < 0, "weighted ask %s", sourceAsk.String())

	bid, ask, ok := hedge.GetQuotePrices()
	assert.True(t, ok)
	assert.Equal(t, sourceBid.Mul(Number(30)), bid)
	assert.Equal(t, sourceAsk.Mul(Number(31)), ask)
}

func TestSyntheticHedge_SplitSource_Validation(t *testing.T) {
	t.Run("source and splitSource are mutually exclusive", func(t *testing.T) {
		syn := &SyntheticHedge{
			Enabled:     true,
			Source:      &HedgeMarketConfig{SymbolSelector: "binance.BTCUSDT"},
			SplitSource: &SplitHedge{Enabled: true},
			Fiat:        &HedgeMarketConfig{SymbolSelector: "max.USDTTWD"},
		}
		err := syn.InitializeAndBind(nil, &Strategy{})
		assert.ErrorContains(t, err, "mutually exclusive")
	})

	t.Run("source or splitSource is required", func(t *testing.T) {
		syn := &SyntheticHedge{
			Enabled: true,
			Fiat:    &HedgeMarketConfig{SymbolSelector: "max.USDTTWD"},
		}
		err := syn.InitializeAndBind(nil, &Strategy{})
		assert.ErrorContains(t, err, "source or splitSource must be set")
	})

	newSession := func(name string) *bbgo.ExchangeSession {
		return &bbgo.ExchangeSession{
			ExchangeSessionConfig: bbgo.ExchangeSessionConfig{Name: name},
		}
	}

	t.Run("mismatched source quote currencies", func(t *testing.T) {
		btcusdc := types.Market{Symbol: "BTCUSDC", BaseCurrency: "BTC", QuoteCurrency: "USDC"}
		syn := &SyntheticHedge{
			Enabled: true,
			SplitSource: &SplitHedge{
				Enabled: true,
				hedgeMarketInstances: map[string]*HedgeMarket{
					"binance": {market: Market("BTCUSDT"), session: newSession("binance")},
					"okex":    {market: btcusdc, session: newSession("okex")},
				},
			},
			fiatMarket: &HedgeMarket{market: Market("USDTTWD"), session: newSession("max")},
		}
		err := syn.initializeSourcePosition()
		assert.ErrorContains(t, err, "does not match source market")
	})

	t.Run("source quote currency not linked to the fiat market", func(t *testing.T) {
		syn := &SyntheticHedge{
			Enabled: true,
			SplitSource: &SplitHedge{
				Enabled: true,
				hedgeMarketInstances: map[string]*HedgeMarket{
					"binance": {market: Market("BTCUSDT"), session: newSession("binance")},
				},
			},
			fiatMarket: &HedgeMarket{market: Market("BTCTWD"), session: newSession("max")},
		}
		err := syn.initializeSourcePosition()
		assert.ErrorContains(t, err, "does not match fiat market")
	})

	t.Run("forward split source", func(t *testing.T) {
		syn := &SyntheticHedge{
			Enabled: true,
			SplitSource: &SplitHedge{
				Enabled: true,
				hedgeMarketInstances: map[string]*HedgeMarket{
					"binance": {market: Market("BTCUSDT"), session: newSession("binance")},
					"okex":    {market: Market("BTCUSDT"), session: newSession("okex")},
				},
			},
			fiatMarket: &HedgeMarket{market: Market("USDTTWD"), session: newSession("max")},
		}
		err := syn.initializeSourcePosition()
		assert.NoError(t, err)
		assert.True(t, syn.forward)
		if assert.NotNil(t, syn.sourcePosition) {
			assert.Equal(t, "BTCUSDT", syn.sourcePosition.Symbol)
		}
	})
}

func TestSyntheticHedge_SplitSource_MarketOrderHedge(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	makerMarket := Market("BTCTWD")
	sourceMarket := Market("BTCUSDT")
	fiatMarket := Market("USDTTWD")

	sourceBook := types.SliceOrderBook{
		Symbol: sourceMarket.Symbol,
		Bids: types.PriceVolumeSlice{
			{Price: Number(104_000.), Volume: Number(1)},
		},
		Asks: types.PriceVolumeSlice{
			{Price: Number(104_050.0), Volume: Number(1)},
		},
	}

	newSourceHedgeMarket := func(name string) (*HedgeMarket, *bbgo.ExchangeSession, *types.StandardStream, *types.StandardStream) {
		session, marketDataStream, userDataStream := newMockSession(mockCtrl, ctx, sourceMarket.Symbol)
		session.Name = name
		session.SetMarkets(AllMarkets())

		hedgeMarket := NewHedgeMarket(&HedgeMarketConfig{
			SymbolSelector: sourceMarket.Symbol,
			HedgeInterval:  hedgeInterval,
			QuotingDepth:   Number(100.0),
			SyncOrder:      types.BoolPtr(false),
		}, session, sourceMarket)
		hedgeMarket.book.Load(sourceBook)
		return hedgeMarket, session, marketDataStream, userDataStream
	}

	sourceHedgeMarket1, sourceSession1, sourceMarketDataStream1, sourceUserDataStream1 := newSourceHedgeMarket("binance")
	sourceHedgeMarket2, sourceSession2, sourceMarketDataStream2, sourceUserDataStream2 := newSourceHedgeMarket("okex")

	fiatSession, fiatMarketDataStream, fiatUserDataStream := newMockSession(mockCtrl, ctx, fiatMarket.Symbol)
	fiatSession.Name = "max"
	fiatSession.SetMarkets(AllMarkets())
	fiatHedgeMarket := NewHedgeMarket(&HedgeMarketConfig{
		SymbolSelector: fiatMarket.Symbol,
		HedgeInterval:  hedgeInterval,
		QuotingDepth:   Number(10.0),
		SyncOrder:      types.BoolPtr(false),
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
		makerMarket:      makerMarket,
		orderStore:       orderStore,
		tradeCollector:   core.NewTradeCollector(makerMarket.Symbol, makerPosition, orderStore),
		positionExposure: bbgo.NewPositionExposure(makerMarket.Symbol),
	}

	split := &SplitHedge{
		Enabled: true,
		Algo:    SplitHedgeAlgoProportion,
		ProportionAlgo: &SplitHedgeProportionAlgo{
			ProportionMarkets: []*SplitHedgeProportionMarket{
				{Name: "binance", Ratio: Number(0.5)},
				{Name: "okex"}, // remaining
			},
		},
		hedgeMarketInstances: map[string]*HedgeMarket{
			"binance": sourceHedgeMarket1,
			"okex":    sourceHedgeMarket2,
		},
		strategy: strategy,
		logger:   logrus.StandardLogger(),
	}

	syn := &SyntheticHedge{
		Enabled:     true,
		SplitSource: split,
		Fiat: &HedgeMarketConfig{
			SymbolSelector: "max." + fiatMarket.Symbol,
			QuotingDepth:   Number(30.0 * 1000.0),
			HedgeInterval:  hedgeInterval,
		},
		fiatMarket: fiatHedgeMarket,
		logger:     logrus.StandardLogger(),
	}
	assert.NoError(t, syn.initializeSourcePosition())
	assert.True(t, syn.forward)
	assert.NoError(t, syn.initialize(strategy))

	// the connectivity waiting is blocking, so we need to run it in a goroutine
	go func() {
		err := syn.Start(ctx)
		assert.NoError(t, err)
	}()
	defer func() {
		sourceHedgeMarket1.Stop(context.Background())
		sourceHedgeMarket2.Stop(context.Background())
		fiatHedgeMarket.Stop(context.Background())
	}()

	for _, stream := range []*types.StandardStream{sourceMarketDataStream1, sourceMarketDataStream2, fiatMarketDataStream} {
		stream.EmitConnect()
	}
	for _, stream := range []*types.StandardStream{sourceUserDataStream1, sourceUserDataStream2, fiatUserDataStream} {
		stream.EmitConnect()
		stream.EmitAuth()
	}

	// each source market sells half of the position
	sourceSubmitOrder := types.SubmitOrder{
		Market:           sourceMarket,
		Symbol:           "BTCUSDT",
		Quantity:         Number(0.5),
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeMarket,
		MarginSideEffect: types.SideEffectTypeMarginBuy,
	}
	sourceOrder1 := types.Order{
		OrderID:          1,
		SubmitOrder:      sourceSubmitOrder,
		ExecutedQuantity: Number(0.5),
		Status:           types.OrderStatusFilled,
	}
	sourceOrder2 := types.Order{
		OrderID:          2,
		SubmitOrder:      sourceSubmitOrder,
		ExecutedQuantity: Number(0.5),
		Status:           types.OrderStatusFilled,
	}
	sourceSession1.Exchange.(*mocks.MockExchangeExtended).EXPECT().SubmitOrder(gomock.Any(), sourceSubmitOrder).Return(&sourceOrder1, nil).Times(1)
	sourceSession2.Exchange.(*mocks.MockExchangeExtended).EXPECT().SubmitOrder(gomock.Any(), sourceSubmitOrder).Return(&sourceOrder2, nil).Times(1)

	// the fiat market converts the quote quantity of each source trade
	newFiatOrder := func(orderID uint64, quantity fixedpoint.Value) types.Order {
		submitOrder := types.SubmitOrder{
			Market:           fiatMarket,
			Symbol:           "USDTTWD",
			Quantity:         quantity,
			Side:             types.SideTypeSell,
			Type:             types.OrderTypeMarket,
			MarginSideEffect: types.SideEffectTypeMarginBuy,
		}
		createdOrder := types.Order{
			OrderID:          orderID,
			SubmitOrder:      submitOrder,
			ExecutedQuantity: quantity,
			Status:           types.OrderStatusFilled,
		}
		fiatSession.Exchange.(*mocks.MockExchangeExtended).EXPECT().SubmitOrder(gomock.Any(), submitOrder).Return(&createdOrder, nil).Times(1)
		return createdOrder
	}
	fiatOrder1 := newFiatOrder(3, Number(104_000.0*0.5))
	fiatOrder2 := newFiatOrder(4, Number(104_100.0*0.5))

	makerPosition.AddTrade(types.Trade{
		ID:            1,
		OrderID:       1,
		Exchange:      types.ExchangeMax,
		Price:         Number(3_121_500.0),
		Quantity:      Number(1.0),
		QuoteQuantity: Number(3_121_500.0 * 1.0),
		Symbol:        makerMarket.Symbol,
		Side:          types.SideTypeBuy,
		IsBuyer:       true,
		IsMaker:       true,
	})

	// TRIGGER: split the hedge across the source markets
	assert.NoError(t, syn.Hedge(ctx, Number(1.0), Number(-1.0)))
	assert.Equal(t, Number(1.0), strategy.positionExposure.GetPending(), "the strategy position exposure should be covered")

	time.Sleep(stepTime)
	<-sourceHedgeMarket1.hedgedC
	<-sourceHedgeMarket2.hedgedC

	newTrade := func(order types.Order, price fixedpoint.Value) types.Trade {
		return types.Trade{
			ID:            order.OrderID,
			OrderID:       order.OrderID,
			Exchange:      order.Exchange,
			Price:         price,
			Quantity:      order.Quantity,
			QuoteQuantity: price.Mul(order.Quantity),
			Symbol:        order.Symbol,
			Side:          order.Side,
			IsBuyer:       order.Side == types.SideTypeBuy,
		}
	}

	// the source markets are filled at different prices
	sourceUserDataStream1.EmitTradeUpdate(newTrade(sourceOrder1, Number(104_000.0)))
	time.Sleep(stepTime)
	<-fiatHedgeMarket.hedgedC

	sourceUserDataStream2.EmitTradeUpdate(newTrade(sourceOrder2, Number(104_100.0)))
	time.Sleep(stepTime)
	<-fiatHedgeMarket.hedgedC

	// the aggregated source position averages the source trades
	assert.Equal(t, -1.0, syn.sourcePosition.GetBase().Float64(), "aggregated source position")
	assert.Equal(t, 104_050.0, syn.sourcePosition.GetAverageCost().Float64(), "aggregated source average cost")
	assert.Equal(t, 104_050.0, fiatHedgeMarket.Position.GetBase().Float64(), "fiat position should be updated to the quote quantity")

	// the individual source market positions only record the real trades
	assert.Equal(t, -0.5, sourceHedgeMarket1.Position.GetBase().Float64())
	assert.Equal(t, -0.5, sourceHedgeMarket2.Position.GetBase().Float64())

	fiatUserDataStream.EmitTradeUpdate(newTrade(fiatOrder1, Number(30.0)))
	fiatUserDataStream.EmitTradeUpdate(newTrade(fiatOrder2, Number(30.0)))
	time.Sleep(stepTime)

	assert.InDelta(t, 0.0, syn.sourcePosition.GetBase().Float64(), 1e-6, "aggregated source position should be closed to 0")
	assert.InDelta(t, 0.0, fiatHedgeMarket.Position.GetBase().Float64(), 1e-6, "fiat position should be closed to 0")
	assert.InDelta(t, 0.0, makerPosition.GetBase().Float64(), 1e-6, "the maker position should be closed to 0")

	// the synthetic trades are priced at the aggregated source average cost times the fiat price
	assert.Equal(t, 104_050.0*30.0, orderStore.Orders()[0].Price.Float64())
}
