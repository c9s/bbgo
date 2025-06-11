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
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type HedgeExecutor interface {
	// hedge executes a hedge order based on the uncovered position and the hedge delta
	// uncoveredPosition: the current uncovered position that needs to be hedged
	// hedgeDelta: the delta that needs to be hedged, which is the negative of uncoveredPosition
	// quantity: the absolute value of hedgeDelta, which is the order quantity to be hedged
	// side: the side of the hedge order, which is determined by the sign of hedgeDelta
	hedge(
		ctx context.Context,
		uncoveredPosition, hedgeDelta, quantity fixedpoint.Value,
		side types.SideType,
	) error
}

type MarketOrderHedgeExecutorConfig struct {
	MaxOrderQuantity fixedpoint.Value `json:"maxOrderQuantity,omitempty"` // max order quantity for market order hedge
}

type MarketOrderHedgeExecutor struct {
	*HedgeMarket

	config *MarketOrderHedgeExecutorConfig
}

func newMarketOrderHedgeExecutor(
	market *HedgeMarket,
	config *MarketOrderHedgeExecutorConfig,
) *MarketOrderHedgeExecutor {
	return &MarketOrderHedgeExecutor{
		HedgeMarket: market,
		config:      config,
	}
}

func (m *MarketOrderHedgeExecutor) hedge(
	ctx context.Context,
	uncoveredPosition, hedgeDelta, quantity fixedpoint.Value,
	side types.SideType,
) error {
	if uncoveredPosition.IsZero() {
		return nil
	}

	bid, ask := m.getQuotePrice()
	price := sideTakerPrice(bid, ask, side)

	quantity = AdjustHedgeQuantityWithAvailableBalance(
		m.session.GetAccount(), m.market, side, quantity, price,
	)

	if m.config != nil {
		if m.config.MaxOrderQuantity.Sign() > 0 {
			quantity = fixedpoint.Max(quantity, m.config.MaxOrderQuantity)
		}
	}

	if m.market.IsDustQuantity(quantity, price) {
		m.logger.Infof("skip dust quantity: %s @ price %f", quantity.String(), price.Float64())
		return nil
	}

	hedgeOrder, err := m.submitOrder(ctx, types.SubmitOrder{
		Symbol:   m.Symbol,
		Market:   m.market,
		Side:     side,
		Type:     types.OrderTypeMarket,
		Quantity: quantity,
	})
	if err != nil {
		return err
	}

	m.positionExposure.Cover(uncoveredPosition)

	m.logger.Infof("hedge order created: %+v", hedgeOrder)
	return nil
}

type CounterpartyHedgeExecutorConfig struct {
	PriceLevel int `json:"priceLevel"`
}

type CounterpartyHedgeExecutor struct {
	*HedgeMarket

	config     *CounterpartyHedgeExecutorConfig
	hedgeOrder *types.Order
}

func newCounterpartyHedgeExecutor(
	market *HedgeMarket,
	config *CounterpartyHedgeExecutorConfig,
) *CounterpartyHedgeExecutor {
	return &CounterpartyHedgeExecutor{
		HedgeMarket: market,
		config:      config,
	}
}

func (m *CounterpartyHedgeExecutor) hedge(
	ctx context.Context,
	uncoveredPosition, hedgeDelta, quantity fixedpoint.Value,
	side types.SideType,
) error {
	if m.hedgeOrder != nil {
		if err := m.session.Exchange.CancelOrders(ctx, *m.hedgeOrder); err != nil {
			m.logger.WithError(err).Errorf("failed to cancel order: %+v", m.hedgeOrder)
		}

		hedgeOrder, err := retry.QueryOrderUntilCanceled(ctx, m.session.Exchange.(types.ExchangeOrderQueryService), m.hedgeOrder.AsQuery())
		if err != nil {
			m.logger.WithError(err).Errorf("failed to query order after cancel: %+v", m.hedgeOrder)
		} else {
			m.logger.Infof("hedge order canceled: %+v, returning covered position...", hedgeOrder)

			// return covered position from the canceled order
			m.positionExposure.Cover(quantityToDelta(hedgeOrder.GetRemainingQuantity(), hedgeOrder.Side))
		}

		m.hedgeOrder = nil
	}

	if uncoveredPosition.IsZero() {
		return nil
	}

	// use counterparty side book
	counterpartySide := side.Reverse()
	sideBook := m.book.SideBook(counterpartySide)

	if len(sideBook) == 0 {
		return fmt.Errorf("side book is empty for %s", m.Symbol)
	}

	priceLevel := m.config.PriceLevel
	offset := 0
	if m.config.PriceLevel < 0 {
		offset = m.config.PriceLevel
		priceLevel = 0
	} else {
		priceLevel--
	}

	if priceLevel > len(sideBook) {
		return fmt.Errorf("invalid price level %d for %s", m.config.PriceLevel, m.Symbol)
	}

	price := sideBook[priceLevel].Price
	if offset > 0 {
		ticks := m.market.TickSize.Mul(fixedpoint.NewFromInt(int64(offset)))
		switch counterpartySide {
		case types.SideTypeBuy:
			price = price.Add(ticks)
		case types.SideTypeSell:
			price = price.Sub(ticks)
		}
	}

	quantity = AdjustHedgeQuantityWithAvailableBalance(
		m.session.GetAccount(), m.market, side, quantity, price,
	)

	if m.market.IsDustQuantity(quantity, price) {
		m.logger.Infof("skip dust quantity: %s @ price %f", quantity.String(), price.Float64())
		return nil
	}

	hedgeOrder, err := m.submitOrder(ctx, types.SubmitOrder{
		Type:     types.OrderTypeLimit,
		Symbol:   m.Symbol,
		Market:   m.market,
		Side:     side,
		Price:    price,
		Quantity: quantity,
	})

	if err != nil {
		return err
	}

	m.hedgeOrder = hedgeOrder
	m.positionExposure.Cover(uncoveredPosition)
	m.logger.Infof("hedge order created: %+v", hedgeOrder)
	return nil

}

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

	hedgeExecutor HedgeExecutor

	hedgedC chan struct{}
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

		hedgedC: make(chan struct{}, 1),
		logger:  logger,
	}

	switch m.HedgeMethod {
	case HedgeMethodMarket:
		m.hedgeExecutor = newMarketOrderHedgeExecutor(m, m.HedgeMethodMarket)
	case HedgeMethodCounterparty:
		m.hedgeExecutor = newCounterpartyHedgeExecutor(m, m.HedgeMethodCounterparty)
	default:
		m.hedgeExecutor = newMarketOrderHedgeExecutor(m, m.HedgeMethodMarket)
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
	hedgeDelta := uncoveredPosition.Neg()
	quantity := hedgeDelta.Abs()
	side := deltaToSide(hedgeDelta)
	err := m.hedgeExecutor.hedge(ctx, uncoveredPosition, hedgeDelta, quantity, side)

	// emit the hedgedC signal to notify that a hedge has been attempted
	select {
	case m.hedgedC <- struct{}{}:
	default:
	}

	return err
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

		case <-ticker.C:
			if m.positionExposure.IsClosed() {
				continue
			}

			uncoveredPosition := m.positionExposure.GetUncovered()
			if err := m.hedge(ctx, uncoveredPosition); err != nil {
				m.logger.WithError(err).Errorf("hedge failed")
			}

		case delta := <-m.positionDeltaC:
			m.positionExposure.net.Add(delta)
		}
	}
}

func quantityToDelta(quantity fixedpoint.Value, side types.SideType) fixedpoint.Value {
	return quantity.Mul(sideToFixedPointValue(side))
}

func sideToFixedPointValue(side types.SideType) fixedpoint.Value {
	if side == types.SideTypeBuy {
		return fixedpoint.One
	}

	return fixedpoint.NegOne
}

func sideTakerPrice(bid, ask fixedpoint.Value, side types.SideType) fixedpoint.Value {
	if side == types.SideTypeBuy {
		return ask
	}

	return bid
}

func deltaToSide(delta fixedpoint.Value) types.SideType {
	side := types.SideTypeBuy
	if delta.IsZero() {
		side = types.SideTypeNone
	}

	if delta.Sign() < 0 {
		side = types.SideTypeSell
	}

	return side

}
