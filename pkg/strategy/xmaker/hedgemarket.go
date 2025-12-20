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
	"github.com/c9s/bbgo/pkg/bbgo/sessionworker"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/strategy/xmaker/pricer"
	"github.com/c9s/bbgo/pkg/tradeid"
	"github.com/c9s/bbgo/pkg/types"
)

var defaultMinMarginLevel = fixedpoint.NewFromFloat(1.3)

var defaultMaxLeverage = fixedpoint.NewFromFloat(2.0) // 2x leverage

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

// FeeMode controls which fee rate is applied when computing hedge quote prices.
// - taker: use session.TakerFeeRate (default, crossing the spread)
// - maker: use session.MakerFeeRate (posting liquidity)
// - none:  do not apply any fee to the computed prices
// Note: This affects only pricing used by HedgeMarket when evaluating hedge decisions.
type FeeMode string

const (
	FeeModeTaker FeeMode = "taker"
	FeeModeMaker FeeMode = "maker"
	FeeModeNone  FeeMode = "none"
)

type HedgeMarketConfig struct {
	SymbolSelector string         `json:"symbolSelector"`
	HedgeMethod    HedgeMethod    `json:"hedgeMethod"`
	HedgeInterval  types.Duration `json:"hedgeInterval"`

	MinMarginLevel fixedpoint.Value `json:"minMarginLevel"`
	MaxLeverage    fixedpoint.Value `json:"maxLeverage"`

	HedgeMethodMarket       *MarketOrderHedgeExecutorConfig  `json:"hedgeMethodMarket,omitempty"`       // for backward compatibility, this is the default hedge method
	HedgeMethodCounterparty *CounterpartyHedgeExecutorConfig `json:"hedgeMethodCounterparty,omitempty"` // for backward compatibility, this is the default hedge method

	HedgeMethodQueue *struct {
		PriceLevel int `json:"priceLevel"`
	} `json:"hedgeMethodQueue,omitempty"` // for backward compatibility, this is the default hedge method

	QuotingDepth        fixedpoint.Value `json:"quotingDepth"`
	QuotingDepthInQuote fixedpoint.Value `json:"quotingDepthInQuote"`
}

func (c *HedgeMarketConfig) Defaults() error {
	if c.HedgeMethod == "" {
		c.HedgeMethod = HedgeMethodMarket
	}

	if c.HedgeInterval.Duration() == 0 {
		c.HedgeInterval = types.Duration(defaultHedgeInterval)
	}

	if c.HedgeMethodMarket == nil {
		c.HedgeMethodMarket = &MarketOrderHedgeExecutorConfig{
			BaseHedgeExecutorConfig: BaseHedgeExecutorConfig{},
			MaxOrderQuantity:        fixedpoint.Zero,
		}
	}

	if c.MinMarginLevel.IsZero() {
		c.MinMarginLevel = defaultMinMarginLevel
	}

	if c.MaxLeverage.IsZero() {
		c.MaxLeverage = defaultMaxLeverage
	}

	if c.QuotingDepthInQuote.IsZero() {
		c.QuotingDepthInQuote = fixedpoint.NewFromFloat(1000) // default to $1000
	}

	return nil
}

func InitializeHedgeMarketFromConfig(
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

	if err := c.Defaults(); err != nil {
		return nil, fmt.Errorf("failed to set defaults for hedge market %s: %w", c.SymbolSelector, err)
	}

	return NewHedgeMarket(c, session, market), nil
}

// HedgeMarket represents a market used for hedging the main strategy's position.
// It manages the connectivity, order book, position exposure, and executes hedge orders
// When the main strategy's position changes, it updates the position exposure
// and triggers the hedge logic to maintain a neutral position.
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

	// priceFeeMode controls which fee rate will be applied to compute quote prices.
	// Defaults to FeeModeTaker.
	priceFeeMode FeeMode

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

	redispatchCallback func(position fixedpoint.Value)
}

func NewHedgeMarket(
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

	// default logger
	logFields := logrus.Fields{
		"session":      session.Name,
		"exchange":     session.ExchangeName,
		"hedge_market": market.Symbol,
	}
	logger := log.WithFields(logFields)

	m := &HedgeMarket{
		HedgeMarketConfig: config,
		session:           session,
		market:            market,
		stream:            stream,
		book:              book,
		depthBook:         depthBook,

		connectivity: connectivity,

		positionExposure: NewPositionExposure(symbol),

		// default to taker fee mode for backward compatibility
		priceFeeMode: FeeModeTaker,

		positionDeltaC:    make(chan fixedpoint.Value, 100), // this depends on the number of trades
		Position:          position,
		orderStore:        orderStore,
		tradeCollector:    tradeCollector,
		activeMakerOrders: activeMakerOrders,

		hedgedC: make(chan struct{}, 2),
		doneC:   make(chan struct{}),

		logger: logger,
	}

	m.bidPricer = pricer.FromBestPrice(types.SideTypeBuy, m.book)

	m.askPricer = pricer.FromBestPrice(types.SideTypeSell, m.book)

	switch m.HedgeMethod {
	case HedgeMethodMarket:
		m.hedgeExecutor = NewMarketOrderHedgeExecutor(m, m.HedgeMethodMarket)
	case HedgeMethodCounterparty:
		m.hedgeExecutor = newCounterpartyHedgeExecutor(m, m.HedgeMethodCounterparty)
	default:
		m.hedgeExecutor = NewMarketOrderHedgeExecutor(m, m.HedgeMethodMarket)
	}

	tradeCollector.OnTrade(func(trade types.Trade, _, _ fixedpoint.Value) {
		delta := trade.PositionDelta()
		m.positionExposure.Close(delta)
		m.logger.Infof("trade collector received trade: %+v, position delta: %f, position exposure: %s",
			trade, delta.Float64(), m.positionExposure.String())
	})

	return m
}

func (m *HedgeMarket) SetLogger(logger logrus.FieldLogger) {
	m.logger = logger.WithFields(logrus.Fields{
		"session":      m.session.Name,
		"exchange":     m.session.ExchangeName,
		"hedge_market": m.market.Symbol,
	})
}

// SetPriceFeeMode sets the fee mode used when computing hedge quote prices.
// This method is safe to call at runtime.
func (m *HedgeMarket) SetPriceFeeMode(mode FeeMode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.priceFeeMode = mode
}

// priceFeeRate returns the fee rate to apply on prices depending on the current PriceFeeMode.
// If session is nil, it returns zero.
func (m *HedgeMarket) priceFeeRate() fixedpoint.Value {
	if m.session == nil {
		return fixedpoint.Zero
	}

	switch m.priceFeeMode {
	case FeeModeTaker:
		return m.session.TakerFeeRate
	case FeeModeMaker:
		return m.session.MakerFeeRate
	case FeeModeNone:
		return fixedpoint.Zero
	default:
		return m.session.TakerFeeRate
	}
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

// GetQuotePrice returns the current bid and ask prices for hedging,
// adjusted by the configured fee mode and quoting depth.
//
// Implementation detail:
// - If QuotingDepthInQuote is configured, delegate to GetQuotePriceByQuoteAmount.
// - Else if QuotingDepth is configured, delegate to GetQuotePriceByBaseAmount.
// - Else, fall back to best bid/ask with fee application.
func (m *HedgeMarket) GetQuotePrice() (bid, ask fixedpoint.Value) {
	// Prefer configured quoting depth helpers when available to avoid code duplication.
	if m.QuotingDepthInQuote.Sign() > 0 {
		return m.GetQuotePriceByQuoteAmount(m.QuotingDepthInQuote)
	}
	if m.QuotingDepth.Sign() > 0 {
		return m.GetQuotePriceByBaseAmount(m.QuotingDepth)
	}

	// Fallback path: use best bid/ask and apply fee.
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	rawBid := m.bidPricer(0, fixedpoint.Zero)
	rawAsk := m.askPricer(0, fixedpoint.Zero)

	// Apply fee according to PriceFeeMode
	feeRate := m.priceFeeRate()
	bid = pricer.ApplyFeeRate(types.SideTypeBuy, feeRate)(0, rawBid)
	ask = pricer.ApplyFeeRate(types.SideTypeSell, feeRate)(0, rawAsk)

	if bid.IsZero() || ask.IsZero() {
		bids := m.book.SideBook(types.SideTypeBuy)
		asks := m.book.SideBook(types.SideTypeSell)
		m.logger.Warnf("no valid bid/ask price found for %s, bids: %v, asks: %v", m.SymbolSelector, bids, asks)
	}

	// store prices as snapshot
	m.quotingPrice = &types.Ticker{Buy: bid, Sell: ask, Time: now}

	return bid, ask
}

// GetQuotePriceByQuoteAmount returns bid/ask prices averaged over the given quote amount,
// adjusted by taker fee (ask: +fee, bid: -fee). This reflects the effective prices when
// submitting taker orders crossing the spread.
func (m *HedgeMarket) GetQuotePriceByQuoteAmount(quoteAmount fixedpoint.Value) (bid, ask fixedpoint.Value) {
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	rawBid, rawAsk := m.depthBook.BestBidAndAskAtQuoteDepth(quoteAmount)

	// Apply fee according to PriceFeeMode
	feeRate := m.priceFeeRate()
	bid = pricer.ApplyFeeRate(types.SideTypeBuy, feeRate)(0, rawBid)
	ask = pricer.ApplyFeeRate(types.SideTypeSell, feeRate)(0, rawAsk)

	m.quotingPrice = &types.Ticker{Buy: bid, Sell: ask, Time: now}
	return bid, ask
}

// GetQuotePriceByBaseAmount returns bid/ask prices averaged over the given base amount,
// adjusted by taker fee (ask: +fee, bid: -fee).
func (m *HedgeMarket) GetQuotePriceByBaseAmount(baseAmount fixedpoint.Value) (bid, ask fixedpoint.Value) {
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	rawBid, rawAsk := m.depthBook.BestBidAndAskAtDepth(baseAmount)

	// Apply fee according to PriceFeeMode
	feeRate := m.priceFeeRate()
	bid = pricer.ApplyFeeRate(types.SideTypeBuy, feeRate)(0, rawBid)
	ask = pricer.ApplyFeeRate(types.SideTypeSell, feeRate)(0, rawAsk)

	m.quotingPrice = &types.Ticker{Buy: bid, Sell: ask, Time: now}
	return bid, ask
}

func (m *HedgeMarket) canHedge(
	ctx context.Context, uncoveredPosition fixedpoint.Value,
) (bool, fixedpoint.Value, error) {
	hedgeDelta := uncoveredPosition.Neg()
	quantity := hedgeDelta.Abs()
	side := deltaToSide(hedgeDelta)

	// get quote price
	bid, ask := m.GetQuotePrice()
	price := sideTakerPrice(bid, ask, side)

	if price.IsZero() {
		return false, fixedpoint.Zero, fmt.Errorf("canHedge: no valid price found for %s", m.SymbolSelector)
	}

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

		canHedge, quota := m.allowMarginHedge(m.session, minMarginLevel, m.MaxLeverage, side, price)
		if !canHedge {
			return false, fixedpoint.Zero, nil
		}

		return true, fixedpoint.Min(quota, quantity), nil
	}

	// spot mode
	if !hasBalance {
		m.logger.Warnf("canHedge: cannot find balance for currency: %s", currency)
		return false, fixedpoint.Zero, nil
	} else if available.IsZero() {
		m.logger.Warnf("canHedge: zero available balance for currency: %s", currency)
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
	price fixedpoint.Value,
) (bool, fixedpoint.Value) {
	zero := fixedpoint.Zero

	account := session.GetAccount()
	if account.MarginLevel.IsZero() || minMarginLevel.IsZero() {
		return false, zero
	}

	// if the margin level is lower than the minimal margin level,
	// we need to repay the debt first
	if account.MarginLevel.Sign() > 0 && account.MarginLevel.Compare(minMarginLevel) < 0 {
		// check if we can repay the debt via available balance (the reverse side)
		if tryToRepayDebts(context.Background(), session) {
			return m.allowMarginHedge(session, minMarginLevel, maxLeverage, side, price)
		}

		swapQty, canSwap := canSwapDebtOnSide(m.market, account.Balances(), side, price)
		if canSwap {
			return true, swapQty
		}
	}

	// if the margin level is higher than the minimal margin level,
	// we can hedge the position, but we need to check the debt quota
	// debtQuota is the quota with minimal margin level
	debtQuota := m.getDebtQuota()
	if debtQuota.Sign() <= 0 {
		return false, zero
	}

	if price.IsZero() {
		return false, zero
	}

	switch side {
	case types.SideTypeBuy:
		return true, debtQuota.Div(price)
	case types.SideTypeSell:
		return true, debtQuota.Div(price)
	default:
		return false, zero
	}
}

// margin level = totalValue / totalDebtValue * MMR (maintenance margin ratio)
// on binance:
// - MMR with 10x leverage = 5%
// - MMR with 5x leverage = 9%
// - MMR with 3x leverage = 10%
func (m *HedgeMarket) getDebtQuota() fixedpoint.Value {
	var debtQuota = fixedpoint.Zero
	if workerHandle := sessionworker.Get(m.session, "debt-quota"); workerHandle != nil {
		rst := workerHandle.Value().(*DebtQuotaResult)
		debtQuota = rst.AmountInQuote
	}

	return debtQuota
}

func (m *HedgeMarket) hedge(ctx context.Context) error {
	if err := m.hedgeExecutor.Clear(ctx); err != nil {
		return fmt.Errorf("failed to clear hedge executor: %w", err)
	}

	// update uncovered position after clear
	uncoveredPosition := m.positionExposure.GetUncovered()
	hedgeDelta := uncoveredToDelta(uncoveredPosition)
	quantity := hedgeDelta.Abs()
	side := deltaToSide(hedgeDelta)
	if m.market.MinQuantity.Compare(quantity) > 0 {
		// skip dust position
		return nil
	}

	// Before executing the hedge, verify that we have enough balance/credit to do so.
	can, maxQty, errCheck := m.canHedge(ctx, uncoveredPosition)
	if errCheck != nil {
		m.logger.WithError(errCheck).Errorf("canHedge check failed, skip this tick")
		return errCheck
	}

	if !can {
		m.logger.Infof("insufficient balance/credit to hedge on %s, redispatching position %s", m.InstanceID(), uncoveredPosition.String())
		// give the whole position back to parent SplitHedge for re-dispatching
		if err := m.RedispatchPosition(uncoveredPosition); err != nil {
			m.logger.WithError(err).Errorf("failed to redispatch position")
		}
		return nil
	}

	if maxQty.Compare(quantity) < 0 {
		// We can only hedge a portion; redispatch the exceeded quantity back to SplitHedge
		hedgeQty := m.market.TruncateQuantity(maxQty)
		if hedgeQty.IsZero() {
			m.logger.Infof("can hedge only %s < required %s on %s, but hedgeable qty is zero after truncation; redispatching full position", maxQty.String(), quantity.String(), m.InstanceID())
			if err := m.RedispatchPosition(uncoveredPosition); err != nil {
				m.logger.WithError(err).Errorf("failed to redispatch position")
			}
			return nil
		}

		// compute remainder and redispatch only the exceeded part
		remainderQty := quantity.Sub(hedgeQty)
		if remainderQty.Sign() > 0 {
			// build a position value with the same sign as uncoveredPosition
			var remainderPos fixedpoint.Value
			if uncoveredPosition.Sign() >= 0 {
				remainderPos = remainderQty
			} else {
				remainderPos = remainderQty.Neg()
			}

			m.logger.Infof("can hedge only %s < required %s on %s, redispatching exceeded %s and hedging %s",
				hedgeQty.String(), quantity.String(), m.InstanceID(), remainderPos.Abs().String(), hedgeQty.String())

			if err := m.RedispatchPosition(remainderPos); err != nil {
				m.logger.WithError(err).Errorf("failed to redispatch exceeded position")
			}

			// proceed to hedge the hedgeable quantity
			quantity = hedgeQty
		}
	}

	err := m.hedgeExecutor.Hedge(ctx, uncoveredPosition, hedgeDelta, quantity, side)

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
	}

	if m.session != nil && m.session.UserDataConnectivity != nil {
		select {
		case <-ctx.Done():
			return
		case <-m.session.UserDataConnectivity.AuthedC():
		}
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

	m.logger.Infof("HedgeMarket: restored position %s: %+v", m.SymbolSelector, m.Position)
	return nil
}

func (m *HedgeMarket) Sync(ctx context.Context, namespace string) {
	isolation := bbgo.GetIsolationFromContext(ctx)
	ps := isolation.GetPersistenceService()
	id := m.InstanceID()
	store := ps.NewStore(namespace, id)
	if err := store.Save(m.Position); err != nil {
		m.logger.WithError(err).Errorf("HedgeMarket: failed to save position %s", m.SymbolSelector)
	}
}

func (m *HedgeMarket) RedispatchPosition(uncoveredPosition fixedpoint.Value) error {
	if m.redispatchCallback == nil {
		return fmt.Errorf("HedgeMarket: redispatch callback is not set, can't redispatch position")
	}

	// This Close could trigger the strategy's position close callback,
	// which in turn could call RedispatchPosition again, so we need to be careful
	// to avoid deadlock or infinite recursion.
	// Here we assume that the position close callback will not call RedispatchPosition again.
	// If it does, it should be handled gracefully by the strategy.
	m.positionExposure.Close(uncoveredPosition.Neg())
	m.redispatchCallback(uncoveredPosition)
	return nil
}

func (m *HedgeMarket) OnRedispatchPosition(f func(position fixedpoint.Value)) {
	if m.redispatchCallback != nil {
		m.logger.Panicf("HedgeMarket: redispatch callback is already set")
	}

	m.redispatchCallback = f
}

func (m *HedgeMarket) hedgeWorker(ctx context.Context, hedgeInterval time.Duration) {
	defer func() {
		if err := m.hedgeExecutor.Clear(ctx); err != nil {
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

			if err := m.hedge(ctx); err != nil {
				m.logger.WithError(err).Errorf("hedge failed")
			}

		case delta, ok := <-m.positionDeltaC:
			if !ok {
				return
			}

			m.positionExposure.Open(delta)
			if err := m.hedge(ctx); err != nil {
				m.logger.WithError(err).Errorf("hedge failed")
			}
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

// uncoveredToDelta converts uncovered position to delta by negating it.
// delta is the amount needed to hedge the uncovered position.
// For example, if uncovered position is +10 (long 10 units), the delta to hedge it is -10 (sell 10 units).
func uncoveredToDelta(uncoveredPosition fixedpoint.Value) fixedpoint.Value {
	return uncoveredPosition.Neg()
}
