package xmaker

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type HedgeMarket struct {
	symbol    string
	session   *bbgo.ExchangeSession
	market    types.Market
	stream    types.Stream
	book      *types.StreamOrderBook
	depthBook *types.DepthBook

	quotingPrice *types.Ticker

	curPosition, coveredPosition fixedpoint.MutexValue

	positionDeltaC chan fixedpoint.Value // channel to receive position delta updates

	position          *types.Position
	orderStore        *core.OrderStore
	tradeCollector    *core.TradeCollector
	activeMakerOrders *bbgo.ActiveOrderBook

	mu sync.Mutex
}

func newHedgeMarket(
	session *bbgo.ExchangeSession,
	market types.Market,
	depth fixedpoint.Value,
) *HedgeMarket {
	symbol := market.Symbol
	stream := session.Exchange.NewStream()
	stream.SetPublicOnly()
	stream.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{Depth: types.DepthLevelFull})

	book := types.NewStreamBook(symbol, session.Exchange.Name())
	book.BindStream(stream)

	depthBook := types.NewDepthBook(book, depth)

	position := types.NewPositionFromMarket(market)

	orderStore := core.NewOrderStore(symbol)
	orderStore.BindStream(session.UserDataStream)

	tradeCollector := core.NewTradeCollector(symbol, position, orderStore)
	tradeCollector.BindStream(session.UserDataStream)

	activeMakerOrders := bbgo.NewActiveOrderBook(symbol)
	activeMakerOrders.BindStream(session.UserDataStream)

	m := &HedgeMarket{
		session:   session,
		market:    market,
		symbol:    market.Symbol,
		stream:    stream,
		book:      book,
		depthBook: depthBook,

		position:          position,
		orderStore:        orderStore,
		tradeCollector:    tradeCollector,
		activeMakerOrders: activeMakerOrders,
	}

	tradeCollector.OnTrade(func(trade types.Trade, _, _ fixedpoint.Value) {
		delta := trade.PositionDelta()
		m.coveredPosition.Add(delta)

		log.Infof("received trade: %+v, position delta: %f, covered position: %f",
			trade, delta.Float64(), m.coveredPosition.Get().Float64())

		// TODO: pass Environment to HedgeMarket
		/*
			if profit.Compare(fixedpoint.Zero) == 0 {
				s.Environment.RecordPosition(s.Position, trade, nil)
			}
		*/
	})

	return m
}

func (m *HedgeMarket) submitOrder(ctx context.Context, submitOrder types.SubmitOrder) (*types.Order, error) {
	submitOrder.Market = m.market
	submitOrder.Symbol = m.symbol

	submitOrders := []types.SubmitOrder{
		submitOrder,
	}

	formattedOrders, err := m.session.FormatOrders(submitOrders)
	if err != nil {
		return nil, fmt.Errorf("unable to format hedge orders: %w", err)
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
		ctx, m.session.Exchange, orderCreateCallback, formattedOrders...,
	)

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
	bid, ask = m.depthBook.BestBidAndAskAtQuoteDepth()

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
	m.mu.Lock()
	if m.quotingPrice != nil {
		bid = m.quotingPrice.Buy
		ask = m.quotingPrice.Sell
	} else {
		bid, ask = m.depthBook.BestBidAndAskAtQuoteDepth()
	}
	m.mu.Unlock()

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
		log.Infof("skip dust quantity: %s @ price %f", quantity.String(), price.Float64())
		return nil
	}

	hedgeOrder, err := m.submitOrder(ctx, types.SubmitOrder{
		Symbol: m.symbol,
		Market: m.market,
		Side:   side,
		Type:   types.OrderTypeMarket,
		// Price: price,
		Quantity: quantity,
	})
	if err != nil {
		return err
	}

	m.coveredPosition.Add(hedgeDelta.Neg())

	log.Infof("hedge order created: %+v, covered position: %f", hedgeOrder, m.coveredPosition.Get().Float64())
	return nil
}

func (m *HedgeMarket) getUncoveredPosition() fixedpoint.Value {
	position := m.curPosition.Get()
	coveredPosition := m.coveredPosition.Get()
	uncoverPosition := position.Sub(coveredPosition)

	log.Infof(
		"%s base position %v coveredPosition: %v uncoverPosition: %v",
		m.symbol,
		position,
		coveredPosition,
		uncoverPosition,
	)

	return uncoverPosition
}

func (m *HedgeMarket) start(ctx context.Context) error {
	if err := m.stream.Connect(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case now := <-ticker.C:
			_ = now
			uncoveredPosition := m.getUncoveredPosition()
			if err := m.hedge(ctx, uncoveredPosition); err != nil {
				log.WithError(err).Errorf("hedge failed")
			}

		case delta := <-m.positionDeltaC:
			if delta.IsZero() {
				m.curPosition.Add(delta)
			}
		}
	}
}

// SyntheticHedge is a strategy that uses synthetic hedging to manage risk
// SourceSymbol could be something like binance.BTCUSDT
// FiatSymbol could be something like max.USDTTWD
type SyntheticHedge struct {
	// SyntheticHedge is a strategy that uses synthetic hedging to manage risk
	Enabled bool `json:"enabled"`

	SourceSymbol       string           `json:"sourceSymbol"`
	SourceDepthInQuote fixedpoint.Value `json:"sourceDepthInQuote"`

	FiatSymbol       string           `json:"fiatSymbol"`
	FiatDepthInQuote fixedpoint.Value `json:"fiatDepthInQuote"`

	sourceMarket, fiatMarket *HedgeMarket

	sourceQuotingPrice, fiatQuotingPrice *types.Ticker

	mu sync.Mutex
}

func (s *SyntheticHedge) InitializeAndBind(ctx context.Context, sessions map[string]*bbgo.ExchangeSession) error {
	if !s.Enabled {
		return nil
	}

	// Initialize the synthetic quote
	if s.SourceSymbol == "" || s.FiatSymbol == "" {
		return fmt.Errorf("sourceSymbol and fiatSymbol must be set")
	}

	var err error

	sourceSession, sourceMarket, err := parseSymbolSelector(s.SourceSymbol, sessions)
	if err != nil {
		return err
	}

	fiatSession, fiatMarket, err := parseSymbolSelector(s.FiatSymbol, sessions)
	if err != nil {
		return err
	}

	s.sourceMarket = newHedgeMarket(sourceSession, sourceMarket, s.SourceDepthInQuote)
	s.fiatMarket = newHedgeMarket(fiatSession, fiatMarket, s.FiatDepthInQuote)

	// when receiving trades from the source session,
	// mock a trade with the quote amount and add to the fiat position
	s.sourceMarket.tradeCollector.OnTrade(func(trade types.Trade, _, _ fixedpoint.Value) {
		var price, bid, ask fixedpoint.Value
		var quantity = trade.QuoteQuantity.Abs()
		var side = trade.Side.Reverse()

		// get the fiat book price from the snapshot when possible
		s.mu.Lock()
		if s.fiatQuotingPrice != nil {
			bid = s.fiatQuotingPrice.Buy
			ask = s.fiatQuotingPrice.Sell
		} else {
			bid, ask = s.fiatMarket.depthBook.BestBidAndAskAtQuoteDepth()
		}
		s.mu.Unlock()

		switch trade.Side {
		case types.SideTypeBuy:
			price = ask
		case types.SideTypeSell:
			price = bid
		}

		fiatTrade := types.Trade{
			// TODO: set the order id and trade id from ???
			// ID:            trade.ID,
			// OrderID:       trade.OrderID,
			Exchange:      s.fiatMarket.session.ExchangeName,
			Price:         price,
			Quantity:      quantity,
			QuoteQuantity: quantity.Mul(price),
			Symbol:        s.fiatMarket.symbol,
			Side:          side,
			IsBuyer:       side == types.SideTypeBuy,
			IsMaker:       true,
			Time:          trade.Time,
			Fee:           fixedpoint.Zero,
			FeeCurrency:   "",
			StrategyID: sql.NullString{
				String: ID,
				Valid:  true,
			},
		}

		if profit, netProfit, madeProfit := s.fiatMarket.position.AddTrade(fiatTrade); madeProfit {
			// TODO: record the profits in somewhere?
			_ = profit
			_ = netProfit
		}
	})

	return nil
}

// Hedge is the main function to perform the synthetic hedging:
// 1) use the snapshot price as the source average cost
// 2) submit the hedge order to the source exchange
// 3) query trades from of the hedge order.
// 4) build up the source hedge position for the average cost.
// 5) submit fiat hedge order to the fiat market to convert the quote.
// 6) merge the positions.
func (s *SyntheticHedge) Hedge(
	ctx context.Context, uncoveredPosition fixedpoint.Value,
) error {
	if uncoveredPosition.IsZero() {
		return nil
	}

	var bid, ask, price fixedpoint.Value
	s.mu.Lock()
	if s.sourceQuotingPrice != nil {
		bid = s.sourceQuotingPrice.Buy
		ask = s.sourceQuotingPrice.Sell
	} else {
		bid, ask = s.sourceMarket.depthBook.BestBidAndAskAtQuoteDepth()
	}
	s.mu.Unlock()

	hedgePosition := uncoveredPosition.Neg()
	quantity := hedgePosition.Abs()
	side := types.SideTypeBuy
	if hedgePosition.Sign() < 0 {
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
		s.sourceMarket.session.GetAccount(), s.sourceMarket.market, side, quantity, price,
	)

	// skip dust quantity
	if s.sourceMarket.market.IsDustQuantity(quantity, price) {
		log.Infof("skip dust quantity: %s @ price %f", quantity.String(), price.Float64())
		return nil
	}

	hedgeOrder, err := s.sourceMarket.submitOrder(ctx, types.SubmitOrder{
		Symbol: s.sourceMarket.symbol,
		Market: s.sourceMarket.market,
		Side:   side,
		Type:   types.OrderTypeMarket,
		// Price: price,
		Quantity: quantity,
	})
	if err != nil {
		return err
	}

	_ = hedgeOrder

	return nil
}

func (s *SyntheticHedge) GetQuotePrices() (fixedpoint.Value, fixedpoint.Value, bool) {
	if s.sourceMarket == nil || s.fiatMarket == nil {
		return fixedpoint.Zero, fixedpoint.Zero, false
	}

	bid, ask := s.sourceMarket.getQuotePrice()
	bid2, ask2 := s.fiatMarket.getQuotePrice()

	if s.sourceMarket.market.QuoteCurrency == s.fiatMarket.market.BaseCurrency {
		bid = bid.Mul(bid2)
		ask = ask.Mul(ask2)
		return bid, ask, bid.Sign() > 0 && ask.Sign() > 0
	}

	if s.sourceMarket.market.QuoteCurrency == s.fiatMarket.market.QuoteCurrency {
		bid = bid.Div(bid2)
		ask = ask.Div(ask2)
		return bid, ask, bid.Sign() > 0 && ask.Sign() > 0
	}

	return fixedpoint.Zero, fixedpoint.Zero, false
}

func (s *SyntheticHedge) Start(ctx context.Context) error {
	if !s.Enabled {
		return nil
	}

	if s.sourceMarket == nil || s.fiatMarket == nil {
		return fmt.Errorf("sourceMarket and fiatMarket must be initialized")
	}

	if err := s.sourceMarket.start(ctx); err != nil {
		return err
	}

	if err := s.fiatMarket.start(ctx); err != nil {
		return err
	}

	return nil
}
