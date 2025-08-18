package xmaker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/strategy/xmaker/pricer"
	"github.com/c9s/bbgo/pkg/tradeid"
	"github.com/c9s/bbgo/pkg/types"
)

const defaultHedgeInterval = 200 * time.Millisecond

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

	// clear clears any pending orders or state related to hedging
	clear(ctx context.Context) error
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

	Position *types.Position

	orderStore        *core.OrderStore
	tradeCollector    *core.TradeCollector
	activeMakerOrders *bbgo.ActiveOrderBook

	logger logrus.FieldLogger

	mu sync.Mutex

	hedgeExecutor HedgeExecutor

	hedgedC chan struct{}
	doneC   chan struct{}

	tradingCtx    context.Context
	cancelTrading context.CancelFunc
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
	position.Strategy = ID

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

		positionDeltaC:    make(chan fixedpoint.Value, 100), // this depends on the number of trades
		Position:          position,
		orderStore:        orderStore,
		tradeCollector:    tradeCollector,
		activeMakerOrders: activeMakerOrders,

		hedgedC: make(chan struct{}, 1),
		doneC:   make(chan struct{}),

		logger: logger,
	}

	m.logger.Infof("%+v", m.HedgeMethodMarket)

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
	tradeId := tradeid.GlobalGenerator.Generate()

	return types.Trade{
		ID:            tradeId,
		OrderID:       tradeId,
		Exchange:      m.session.ExchangeName,
		Price:         price,
		Quantity:      quantity,
		QuoteQuantity: quantity.Mul(price),
		Symbol:        m.market.Symbol,
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
	submitOrder.Symbol = m.market.Symbol

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
		return nil, fmt.Errorf("failed to submit order: %w, order: %+v", err, submitOrder)
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

	takerFeeRate := fixedpoint.Zero
	if m.session != nil {
		takerFeeRate = m.session.TakerFeeRate
	}

	bidPricer := pricer.Compose(
		pricer.FromBestPrice(types.SideTypeBuy, m.book),
		pricer.ApplyFeeRate(types.SideTypeBuy, takerFeeRate),
	)

	askPricer := pricer.Compose(
		pricer.FromBestPrice(types.SideTypeSell, m.book),
		pricer.ApplyFeeRate(types.SideTypeSell, takerFeeRate),
	)

	bid = bidPricer(0, fixedpoint.Zero)
	ask = askPricer(0, fixedpoint.Zero)

	if bid.IsZero() || ask.IsZero() {
		bids := m.book.SideBook(types.SideTypeBuy)
		asks := m.book.SideBook(types.SideTypeSell)
		m.logger.Warnf("no valid bid/ask price found for %s, bids: %v, asks: %v", m.SymbolSelector, bids, asks)
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

	if err := m.hedgeExecutor.clear(ctx); err != nil {
		return fmt.Errorf("failed to clear hedge executor: %w", err)
	}

	err := m.hedgeExecutor.hedge(ctx, uncoveredPosition, hedgeDelta, quantity, side)

	// emit the hedgedC signal to notify that a hedge has been attempted
	select {
	case m.hedgedC <- struct{}{}:
	default:
	}

	return err
}

func (m *HedgeMarket) WaitForReady(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-m.connectivity.ConnectedC():
		return
	}
}

func (m *HedgeMarket) Start(ctx context.Context) error {
	interval := m.HedgeInterval.Duration()
	if interval == 0 {
		interval = defaultHedgeInterval
	}

	m.positionExposure.SetMetricsLabels(ID,
		m.InstanceID(),
		m.session.ExchangeName.String(),
		m.market.Symbol)

	if err := m.stream.Connect(ctx); err != nil {
		return err
	}

	m.tradingCtx, m.cancelTrading = context.WithCancel(ctx)

	m.logger.Infof("waiting for %s hedge market connectivity...", m.SymbolSelector)
	select {
	case <-ctx.Done():
	case <-m.connectivity.ConnectedC():
	case <-time.After(1 * time.Minute):
		return fmt.Errorf("hedge market %s connectivity timeout", m.SymbolSelector)
	}

	m.logger.Infof("%s hedge market is ready", m.SymbolSelector)

	go m.hedgeWorker(m.tradingCtx, interval)
	return nil
}

func (m *HedgeMarket) InstanceID() string {
	return strings.Join([]string{"hedgeMarket", m.session.Name, m.market.Symbol}, "-")
}

// Restore loads the position from persistence and restores it to the HedgeMarket.
func (m *HedgeMarket) Restore(ctx context.Context, namespace string) error {
	isolation := bbgo.GetIsolationFromContext(ctx)
	ps := isolation.GetPersistenceService()
	id := m.InstanceID()
	store := ps.NewStore(namespace, id)

	if err := store.Load(&m.Position); err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, service.ErrPersistenceNotExists) {
			return nil
		}

		return fmt.Errorf("failed to load position for hedge market %s: %w", m.SymbolSelector, err)
	}

	m.logger.Infof("restored position for hedge market %s: %+v", m.SymbolSelector, m.Position)
	return nil
}

func (m *HedgeMarket) Sync(ctx context.Context, namespace string) {
	isolation := bbgo.GetIsolationFromContext(ctx)
	ps := isolation.GetPersistenceService()
	id := m.InstanceID()
	store := ps.NewStore(namespace, id)
	if err := store.Save(m.Position); err != nil {
		m.logger.WithError(err).Errorf("failed to save position for hedge market %s", m.SymbolSelector)
	}
}

func (m *HedgeMarket) hedgeWorker(ctx context.Context, hedgeInterval time.Duration) {
	defer func() {
		if err := m.hedgeExecutor.clear(ctx); err != nil {
			m.logger.WithError(err).Errorf("failed to clear hedge executor")
		}

		close(m.doneC)
		close(m.hedgedC)
	}()

	ticker := time.NewTicker(hedgeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			if m.positionExposure.IsClosed() {
				continue
			}

			uncoveredPosition := m.positionExposure.GetUncovered()
			if err := m.hedge(ctx, uncoveredPosition); err != nil {
				m.logger.WithError(err).Errorf("hedge failed")
			}

		case delta, ok := <-m.positionDeltaC:
			if !ok {
				return
			}

			m.positionExposure.Open(delta)
		}
	}
}

func (m *HedgeMarket) Stop(shutdownCtx context.Context) {
	m.logger.Infof("stopping hedge market %s", m.SymbolSelector)

	// cancel the context to stop the hedge worker
	if m.cancelTrading != nil {
		m.cancelTrading()
	}

	close(m.positionDeltaC)

	// Wait for the worker goroutine to finish
	select {
	case <-shutdownCtx.Done():
	case <-m.doneC:
	case <-time.After(1 * time.Minute):
		m.logger.Warnf("hedge market %s worker did not finish in time", m.SymbolSelector)
	}

	m.logger.Infof("hedge market %s stopped", m.SymbolSelector)
}

// quantityToDelta converts side to fixedpoint.Value based on the side type,
// and multiplies it with the quantity to get the delta value.
func quantityToDelta(quantity fixedpoint.Value, side types.SideType) fixedpoint.Value {
	return quantity.Mul(sideToFixedPointValue(side))
}

// sideToFixedPointValue converts a side type to a fixedpoint.Value
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
