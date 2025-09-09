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

type HedgeMethod string

const (
	// HedgeMethodMarket is the default hedge method that uses the market order to hedge
	HedgeMethodMarket HedgeMethod = "market"

	// HedgeMethodCounterparty is a hedge method that uses limit order at the specific counterparty price level to hedge
	HedgeMethodCounterparty HedgeMethod = "counterparty"

	// HedgeMethodQueue is a hedge method that uses limit order at the first price level in the queue to hedge
	HedgeMethodQueue HedgeMethod = "queue"
)

const defaultHedgeInterval = 200 * time.Millisecond

type HedgeMarketConfig struct {
	SymbolSelector string         `json:"symbolSelector"`
	HedgeMethod    HedgeMethod    `json:"hedgeMethod"`
	HedgeInterval  types.Duration `json:"hedgeInterval"`

	MinMarginLevel fixedpoint.Value `json:"minMarginLevel"`

	HedgeMethodMarket       *MarketOrderHedgeExecutorConfig  `json:"hedgeMethodMarket,omitempty"`       // for backward compatibility, this is the default hedge method
	HedgeMethodCounterparty *CounterpartyHedgeExecutorConfig `json:"hedgeMethodCounterparty,omitempty"` // for backward compatibility, this is the default hedge method

	HedgeMethodQueue *struct {
		PriceLevel int `json:"priceLevel"`
	} `json:"hedgeMethodQueue,omitempty"` // for backward compatibility, this is the default hedge method

	QuotingDepth        fixedpoint.Value `json:"quotingDepth"`
	QuotingDepthInQuote fixedpoint.Value `json:"quotingDepthInQuote"`
}

func initializeHedgeMarketFromConfig(
	c *HedgeMarketConfig,
	sessions map[string]*bbgo.ExchangeSession,
) (*HedgeMarket, error) {
	session, market, err := parseSymbolSelector(c.SymbolSelector, sessions)
	if err != nil {
		return nil, err
	}

	if c.QuotingDepth.IsZero() && c.QuotingDepthInQuote.IsZero() {
		return nil, fmt.Errorf("quotingDepth or quotingDepthInQuote must be set for hedge market %s", c.SymbolSelector)
	}

	hm := newHedgeMarket(c, session, market)
	return hm, nil
}

type HedgeMarket struct {
	*HedgeMarketConfig

	session *bbgo.ExchangeSession
	market  types.Market
	stream  types.Stream

	connectivity *types.Connectivity

	book      *types.StreamOrderBook
	depthBook *types.DepthBook

	quotingPrice         *types.Ticker
	bidPricer, askPricer pricer.Pricer

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

	debtQuotaCache *fixedpoint.ExpirableValue
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
		"session":      session.Name,
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

	takerFeeRate := fixedpoint.Zero
	if m.session != nil {
		takerFeeRate = m.session.TakerFeeRate
	}

	m.bidPricer = pricer.Compose(
		pricer.FromBestPrice(types.SideTypeBuy, m.book),
		pricer.ApplyFeeRate(types.SideTypeBuy, takerFeeRate),
	)

	m.askPricer = pricer.Compose(
		pricer.FromBestPrice(types.SideTypeSell, m.book),
		pricer.ApplyFeeRate(types.SideTypeSell, takerFeeRate),
	)

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
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	bid = m.bidPricer(0, fixedpoint.Zero)
	ask = m.askPricer(0, fixedpoint.Zero)

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

func (m *HedgeMarket) canHedge(
	ctx context.Context, uncoveredPosition fixedpoint.Value,
) (bool, fixedpoint.Value, error) {
	hedgeDelta := uncoveredPosition.Neg()
	quantity := hedgeDelta.Abs()
	side := deltaToSide(hedgeDelta)

	// get quote price
	bid, ask := m.getQuotePrice()
	price := sideTakerPrice(bid, ask, side)
	currency, required := determineRequiredCurrencyAndAmount(m.market, side, quantity, price)
	// required = required amount of quote, or base currency depending on the side
	_ = required

	account := m.session.GetAccount()
	available, hasBalance := getAvailableBalance(account, currency)
	maxQuantity := quantity
	amt := available
	if amt.IsZero() || m.session.Margin {
		amt = required
	}

	if !amt.IsZero() {
		if side == types.SideTypeBuy {
			// for buy, we need to check if we have enough quote currency
			maxQuantity = fixedpoint.Min(maxQuantity, amt.Div(price))
		} else {
			// for sell, we need to check if we have enough base currency
			maxQuantity = fixedpoint.Min(maxQuantity, amt)
		}
	}

	quantity = fixedpoint.Min(quantity, maxQuantity)

	if m.market.IsDustQuantity(quantity, price) {
		log.Warnf("canHedge: skip dust quantity: %s @ price %f", quantity.String(), price.Float64())
		return false, fixedpoint.Zero, nil
	}

	// for margin account, we need to check if the margin level is sufficient
	if m.session.Margin {
		// a simple check to ensure the account is not in danger of liquidation
		minMarginLevel := m.MinMarginLevel
		if minMarginLevel.IsZero() {
			// default to 150% margin level
			minMarginLevel = fixedpoint.NewFromFloat(1.1) // 110%
		}

		if account.MarginLevel.IsZero() || account.MarginLevel.Compare(minMarginLevel) < 0 {
			log.Warnf("canHedge: margin level too low to hedge: %s (less than %s)", account.MarginLevel.String(), minMarginLevel.String())
			return false, fixedpoint.Zero, nil
		}

		// TODO: calculate debt quota here

		return true, available, nil
	}

	// spot mode
	if !hasBalance {
		log.Warnf("canHedge: cannot find balance for currency: %s", currency)
		return false, fixedpoint.Zero, nil
	} else if available.IsZero() {
		log.Warnf("canHedge: zero available balance for currency: %s", currency)
		return false, fixedpoint.Zero, nil
	} else if m.market.IsDustQuantity(available, price) {
		return false, fixedpoint.Zero, nil
	}

	return true, quantity, nil
}

func (m *HedgeMarket) allowMarginHedge(
	session *bbgo.ExchangeSession,
	minMarginLevel, maxLeverage fixedpoint.Value,
	side types.SideType,
) (bool, fixedpoint.Value) {
	zero := fixedpoint.Zero

	account := session.GetAccount()
	if account.MarginLevel.IsZero() || minMarginLevel.IsZero() {
		return false, zero
	}

	bid, ask := m.getQuotePrice()
	price := sideTakerPrice(bid, ask, side)

	bufMinMarginLevel := minMarginLevel.Mul(fixedpoint.NewFromFloat(1.005))

	accountValueCalculator := session.GetAccountValueCalculator()
	marketValue := accountValueCalculator.MarketValue()
	debtValue := accountValueCalculator.DebtValue()
	netValueInUsd := accountValueCalculator.NetValue()

	m.logger.Infof(
		"hedge account net value in usd: %f, debt value in usd: %f, total value in usd: %f",
		netValueInUsd.Float64(),
		debtValue.Float64(),
		marketValue.Float64(),
	)

	// balances := account.Balances()
	quoteBal, _ := account.Balance(m.market.QuoteCurrency)
	baseBal, _ := account.Balance(m.market.BaseCurrency)

	// if the margin level is lower than the minimal margin level,
	// we need to repay the debt first
	if account.MarginLevel.Compare(minMarginLevel) < 0 {
		// check if we can repay the debt via available balance (the reverse side)
		if tryToRepayDebts(context.Background(), m.session) {
			return m.allowMarginHedge(session, minMarginLevel, maxLeverage, side)
		}

		swapQty, canSwap := canSwapDebtOnSide(m.market, account.Balances(), side, price)
		if canSwap {
			return true, swapQty
		}
	}

	// if the margin level is higher than the minimal margin level,
	// we can hedge the position, but we need to check the debt quota
	if account.MarginLevel.Compare(minMarginLevel) > 0 {
		// debtQuota is the quota with minimal margin level
		debtQuota := m.calculateDebtQuota(marketValue, debtValue, bufMinMarginLevel, maxLeverage)

		m.logger.Infof(
			"hedge account margin level %f > %f, debt quota: %f",
			account.MarginLevel.Float64(), minMarginLevel.Float64(), debtQuota.Float64(),
		)

		if debtQuota.Sign() <= 0 {
			return false, zero
		}

		// if MaxHedgeAccountLeverage is set, we need to calculate credit buffer
		if maxLeverage.Sign() > 0 {
			maximumValueInUsd := netValueInUsd.Mul(maxLeverage)
			leverageQuotaInUsd := maximumValueInUsd.Sub(debtValue)
			m.logger.Infof(
				"hedge account maximum leveraged value in usd: %f (%f x), quota in usd: %f",
				maximumValueInUsd.Float64(),
				maxLeverage.Float64(),
				leverageQuotaInUsd.Float64(),
			)

			debtQuota = fixedpoint.Min(debtQuota, leverageQuotaInUsd)
		}

		switch side {
		case types.SideTypeBuy:
			return true, debtQuota

		case types.SideTypeSell:
			if price.IsZero() {
				return false, zero
			}

			return true, debtQuota.Div(price)

		}
		return true, zero
	}

	switch side {
	case types.SideTypeBuy:
		if baseBal.Available.IsZero() {
			return false, zero
		}

		quota := baseBal.Available.Mul(price)

		// for buy orders, we need to check if we can repay the quoteBal asset via selling the base balance
		quoteDebt := quoteBal.Debt()
		if quoteDebt.Sign() > 0 {
			return true, fixedpoint.Min(quota, quoteDebt)
		}

		return false, zero

	case types.SideTypeSell:
		if quoteBal.Available.IsZero() {
			return false, zero
		}

		quota := quoteBal.Available.Div(price)

		baseDebt := baseBal.Debt()
		if baseDebt.Sign() > 0 {
			// return how much quote bal amount we can use to place the buy order
			return true, fixedpoint.Min(quota, baseDebt)
		}

		return false, zero
	}

	return false, zero
}

// margin level = totalValue / totalDebtValue * MMR (maintenance margin ratio)
// on binance:
// - MMR with 10x leverage = 5%
// - MMR with 5x leverage = 9%
// - MMR with 3x leverage = 10%
func (s *HedgeMarket) calculateDebtQuota(totalValue, debtValue, minMarginLevel, leverage fixedpoint.Value) fixedpoint.Value {
	now := time.Now()
	if s.debtQuotaCache != nil {
		if v, ok := s.debtQuotaCache.Get(now); ok {
			return v
		}
	}

	if minMarginLevel.IsZero() || totalValue.IsZero() {
		return fixedpoint.Zero
	}

	defaultMmr := fixedpoint.NewFromFloat(9.0 * 0.01)
	if leverage.Compare(fixedpoint.NewFromFloat(10.0)) >= 0 {
		defaultMmr = fixedpoint.NewFromFloat(5.0 * 0.01) // 5%
	} else if leverage.Compare(fixedpoint.NewFromFloat(5.0)) >= 0 {
		defaultMmr = fixedpoint.NewFromFloat(9.0 * 0.01) // 9%
	} else if leverage.Compare(fixedpoint.NewFromFloat(3.0)) >= 0 {
		defaultMmr = fixedpoint.NewFromFloat(10.0 * 0.01) // 10%
	}

	debtCap := totalValue.Div(minMarginLevel).Div(defaultMmr)
	marginLevel := totalValue.Div(debtValue).Div(defaultMmr)

	s.logger.Infof(
		"calculateDebtQuota: debtCap=%f, debtValue=%f currentMarginLevel=%f mmr=%f",
		debtCap.Float64(),
		debtValue.Float64(),
		marginLevel.Float64(),
		defaultMmr.Float64(),
	)

	debtQuota := debtCap.Sub(debtValue)
	if debtQuota.Sign() < 0 {
		return fixedpoint.Zero
	}

	if s.debtQuotaCache == nil {
		s.debtQuotaCache = fixedpoint.NewExpirable(debtQuota, now.Add(debtQuotaCacheDuration))
	} else {
		s.debtQuotaCache.Set(debtQuota, now.Add(debtQuotaCacheDuration))
	}

	return debtQuota
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

func canSwapDebtOnSide(
	market types.Market, balances types.BalanceMap, side types.SideType, price fixedpoint.Value,
) (fixedpoint.Value, bool) {
	var debtCurrency string
	var qty fixedpoint.Value
	switch side {
	case types.SideTypeSell:
		// check if we have debt in quote, and we have available base to sell
		debtCurrency = market.QuoteCurrency
		if baseBal, ok := balances[market.BaseCurrency]; ok {
			qty = baseBal.Available
		}

	case types.SideTypeBuy:
		// check if we have debt in base, and we have available quote to buy
		debtCurrency = market.BaseCurrency
		if quoteBal, ok := balances[market.QuoteCurrency]; ok {
			qty = quoteBal.Available.Div(price)
		}

	default:
		return fixedpoint.Zero, false
	}

	if market.IsDustQuantity(qty, price) {
		return fixedpoint.Zero, false
	}

	debtBal, ok := balances[debtCurrency]
	if !ok || debtBal.Debt().IsZero() {
		return fixedpoint.Zero, false
	}

	return qty, true
}

func positionToSide(pos fixedpoint.Value) types.SideType {
	side := types.SideTypeBuy
	if pos.Sign() < 0 {
		side = types.SideTypeSell
	}
	return side
}

// uncoveredToDelta converts uncovered position to delta by negating it.
// delta is the amount needed to hedge the uncovered position.
// For example, if uncovered position is +10 (long 10 units), the delta to hedge it is -10 (sell 10 units).
func uncoveredToDelta(uncoveredPosition fixedpoint.Value) fixedpoint.Value {
	return uncoveredPosition.Neg()
}
