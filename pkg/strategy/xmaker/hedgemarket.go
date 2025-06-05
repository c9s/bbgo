package xmaker

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type HedgeMarket struct {
	*HedgeMarketConfig

	session *bbgo.ExchangeSession
	market  types.Market
	stream  types.Stream

	connectivity *types.Connectivity

	book      *types.StreamOrderBook
	depthBook *types.DepthBook

	quotingPrice *types.Ticker

	positionExposure *PositionExposure

	positionDeltaC chan fixedpoint.Value // channel to receive position delta updates

	position          *types.Position
	orderStore        *core.OrderStore
	tradeCollector    *core.TradeCollector
	activeMakerOrders *bbgo.ActiveOrderBook

	logger logrus.FieldLogger

	mockTradeId uint64
	mu          sync.Mutex
}

func newHedgeMarket(
	config *HedgeMarketConfig,
	session *bbgo.ExchangeSession,
	market types.Market,
) *HedgeMarket {
	symbol := market.Symbol
	stream := session.Exchange.NewStream()
	stream.SetPublicOnly()
	stream.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{Depth: types.DepthLevelFull})

	connectivity := types.NewConnectivity()
	connectivity.Bind(stream)

	book := types.NewStreamBook(symbol, session.Exchange.Name())
	book.BindStream(stream)

	depthBook := types.NewDepthBook(book)

	position := types.NewPositionFromMarket(market)

	orderStore := core.NewOrderStore(symbol)
	orderStore.BindStream(session.UserDataStream)

	tradeCollector := core.NewTradeCollector(symbol, position, orderStore)
	tradeCollector.BindStream(session.UserDataStream)

	activeMakerOrders := bbgo.NewActiveOrderBook(symbol)
	activeMakerOrders.BindStream(session.UserDataStream)

	logger := log.WithFields(logrus.Fields{
		"exchange":     session.ExchangeName,
		"hedge_market": market.Symbol,
	})

	m := &HedgeMarket{
		HedgeMarketConfig: config,
		session:           session,
		market:            market,
		stream:            stream,
		book:              book,
		depthBook:         depthBook,

		connectivity: connectivity,

		positionExposure: newPositionExposure(symbol),

		positionDeltaC:    make(chan fixedpoint.Value, 5),
		position:          position,
		orderStore:        orderStore,
		tradeCollector:    tradeCollector,
		activeMakerOrders: activeMakerOrders,

		logger: logger,
	}

	tradeCollector.OnTrade(func(trade types.Trade, _, _ fixedpoint.Value) {
		delta := trade.PositionDelta()
		m.positionExposure.Close(delta)

		m.logger.Infof("trade collector received trade: %+v, position delta: %f, covered position: %f",
			trade, delta.Float64(), m.positionExposure.pending.Get().Float64())

		// TODO: pass Environment to HedgeMarket
		/*
			if profit.Compare(fixedpoint.Zero) == 0 {
				s.Environment.RecordPosition(s.Position, trade, nil)
			}
		*/
	})

	return m
}

func (m *HedgeMarket) newMockTrade(
	side types.SideType, price, quantity fixedpoint.Value, tradeTime time.Time,
) types.Trade {
	m.mockTradeId++

	return types.Trade{
		ID:            m.mockTradeId,
		OrderID:       m.mockTradeId,
		Exchange:      m.session.ExchangeName,
		Price:         price,
		Quantity:      quantity,
		QuoteQuantity: quantity.Mul(price),
		Symbol:        m.Symbol,
		Side:          side,
		IsBuyer:       side == types.SideTypeBuy,
		IsMaker:       true,
		Time:          types.Time(tradeTime),
		Fee:           fixedpoint.Zero,
		FeeCurrency:   "",
		StrategyID: sql.NullString{
			String: ID,
			Valid:  true,
		},
	}

}

func (m *HedgeMarket) submitOrder(ctx context.Context, submitOrder types.SubmitOrder) (*types.Order, error) {
	submitOrder.Market = m.market
	submitOrder.Symbol = m.Symbol

	submitOrders := []types.SubmitOrder{
		submitOrder,
	}

	orderCreateCallback := func(createdOrder types.Order) {
		m.orderStore.Add(createdOrder)

		// for track non-market-orders
		if createdOrder.Type != types.OrderTypeMarket {
			m.activeMakerOrders.Add(createdOrder)
		}
	}

	defer m.tradeCollector.Process()

	createdOrders, _, err := bbgo.BatchPlaceOrder(
		ctx, m.session.Exchange, orderCreateCallback, submitOrders...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit order: %w", err)
	}

	if len(createdOrders) == 0 {
		return nil, fmt.Errorf("no hedge order created")
	}

	createdOrder := createdOrders[0]
	return &createdOrder, nil
}

func (m *HedgeMarket) getQuotePrice() (bid, ask fixedpoint.Value) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	if m.QuotingDepthInQuote.Sign() > 0 {
		bid, ask = m.depthBook.BestBidAndAskAtQuoteDepth(m.QuotingDepthInQuote)
	} else {
		bid, ask = m.depthBook.BestBidAndAskAtDepth(m.QuotingDepth)
	}

	// store prices as snapshot
	m.quotingPrice = &types.Ticker{
		Buy:  bid,
		Sell: ask,
		Time: now,
	}

	return bid, ask
}

func (m *HedgeMarket) hedge(
	ctx context.Context, uncoveredPosition fixedpoint.Value,
) error {

	if uncoveredPosition.IsZero() {
		return nil
	}

	var bid, ask, price fixedpoint.Value

	// TODO: use hedge method to get the price
	bid, ask = m.getQuotePrice()

	hedgeDelta := uncoveredPosition.Neg()
	quantity := hedgeDelta.Abs()
	side := types.SideTypeBuy
	if hedgeDelta.Sign() < 0 {
		side = types.SideTypeSell
	}

	// use taker price
	switch side {
	case types.SideTypeBuy:
		price = ask
	case types.SideTypeSell:
		price = bid
	}

	quantity = AdjustHedgeQuantityWithAvailableBalance(
		m.session.GetAccount(), m.market, side, quantity, price,
	)

	// skip dust quantity
	if m.market.IsDustQuantity(quantity, price) {
		m.logger.Infof("skip dust quantity: %s @ price %f", quantity.String(), price.Float64())
		return nil
	}

	hedgeOrder, err := m.submitOrder(ctx, types.SubmitOrder{
		Symbol: m.Symbol,
		Market: m.market,
		Side:   side,
		Type:   types.OrderTypeMarket,
		// Price: price,
		Quantity: quantity,
	})
	if err != nil {
		return err
	}

	m.positionExposure.pending.Add(hedgeDelta.Neg())

	m.logger.Infof("hedge order created: %+v, covered position: %f", hedgeOrder, m.positionExposure.pending.Get().Float64())
	return nil
}

func (m *HedgeMarket) Start(ctx context.Context) error {
	interval := m.HedgeInterval.Duration()
	if interval == 0 {
		interval = 3 * time.Second // default interval
	}

	return m.start(ctx, interval)
}

func (m *HedgeMarket) WaitForReady(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-m.connectivity.ConnectedC():
		return
	}
}

func (m *HedgeMarket) start(ctx context.Context, hedgeInterval time.Duration) error {
	if err := m.stream.Connect(ctx); err != nil {
		return err
	}

	m.logger.Infof("waiting for %s hedge market connectivity...", m.Symbol)
	<-m.connectivity.ConnectedC()

	ticker := time.NewTicker(hedgeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case now := <-ticker.C:
			_ = now
			uncoveredPosition := m.positionExposure.GetUncovered()
			if err := m.hedge(ctx, uncoveredPosition); err != nil {
				m.logger.WithError(err).Errorf("hedge failed")
			}

		case delta := <-m.positionDeltaC:
			m.positionExposure.net.Add(delta)
		}
	}
}
