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

type PositionExposure struct {
	symbol string

	// net = net position
	// pending = covered position
	net, pending fixedpoint.MutexValue
}

func newPositionExposure(symbol string) *PositionExposure {
	return &PositionExposure{
		symbol: symbol,
	}
}

func (m *PositionExposure) Open(delta fixedpoint.Value) {
	m.net.Add(delta)

	log.Infof(
		"%s opened:%f netPosition:%f coveredPosition: %f",
		m.symbol,
		delta.Float64(),
		m.net.Get().Float64(),
		m.pending.Get().Float64(),
	)
}

func (m *PositionExposure) Cover(delta fixedpoint.Value) {
	m.pending.Add(delta)

	log.Infof(
		"%s covered:%f netPosition:%f coveredPosition: %f",
		m.symbol,
		delta.Float64(),
		m.net.Get().Float64(),
		m.pending.Get().Float64(),
	)
}

func (m *PositionExposure) Close(delta fixedpoint.Value) {
	m.pending.Add(delta)
	m.net.Add(delta)

	log.Infof(
		"%s closed:%f netPosition:%f coveredPosition: %f",
		m.symbol,
		delta.Float64(),
		m.net.Get().Float64(),
		m.pending.Get().Float64(),
	)
}

func (m *PositionExposure) GetUncovered() fixedpoint.Value {
	netPosition := m.net.Get()
	coveredPosition := m.pending.Get()
	uncoverPosition := netPosition.Sub(coveredPosition)

	log.Infof(
		"%s netPosition:%v coveredPosition: %v uncoverPosition: %v",
		m.symbol,
		netPosition,
		coveredPosition,
		uncoverPosition,
	)

	return uncoverPosition
}

type HedgeMarket struct {
	symbol    string
	session   *bbgo.ExchangeSession
	market    types.Market
	stream    types.Stream
	book      *types.StreamOrderBook
	depthBook *types.DepthBook

	quotingPrice *types.Ticker

	positionExposure *PositionExposure

	positionDeltaC chan fixedpoint.Value // channel to receive position delta updates

	position          *types.Position
	orderStore        *core.OrderStore
	tradeCollector    *core.TradeCollector
	activeMakerOrders *bbgo.ActiveOrderBook

	mockTradeId uint64
	mu          sync.Mutex
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

		positionExposure: newPositionExposure(symbol),

		positionDeltaC:    make(chan fixedpoint.Value, 5),
		position:          position,
		orderStore:        orderStore,
		tradeCollector:    tradeCollector,
		activeMakerOrders: activeMakerOrders,
	}

	tradeCollector.OnTrade(func(trade types.Trade, _, _ fixedpoint.Value) {
		delta := trade.PositionDelta()
		m.positionExposure.pending.Add(delta)
		m.positionExposure.net.Add(delta)

		log.Infof("trade collector received trade: %+v, position delta: %f, covered position: %f",
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
		Symbol:        m.symbol,
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
	submitOrder.Symbol = m.symbol

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

	m.positionExposure.pending.Add(hedgeDelta.Neg())

	log.Infof("hedge order created: %+v, covered position: %f", hedgeOrder, m.positionExposure.pending.Get().Float64())
	return nil
}

func (m *HedgeMarket) start(ctx context.Context, hedgeInterval time.Duration) error {
	if err := m.stream.Connect(ctx); err != nil {
		return err
	}

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
				log.WithError(err).Errorf("hedge failed")
			}

		case delta := <-m.positionDeltaC:
			m.positionExposure.net.Add(delta)
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

func (s *SyntheticHedge) InitializeAndBind(
	ctx context.Context,
	sessions map[string]*bbgo.ExchangeSession,
	targetPosition *types.Position,
) error {
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
	//
	// hedge flow:
	//   maker trade -> update source market position delta
	//   source market position delta -> convert to fiat trade -> update fiat market position delta
	s.sourceMarket.tradeCollector.OnTrade(func(trade types.Trade, _, _ fixedpoint.Value) {
		price := fixedpoint.Zero

		// 1) get the fiat book price from the snapshot when possible
		bid, ask := s.fiatMarket.getQuotePrice()

		// 2) build up fiat position from the trade quote quantity
		fiatQuantity := trade.QuoteQuantity
		fiatDelta := fiatQuantity

		// reverse side because we assume source's quote currency is the fiat market's base currency here
		fiatSide := trade.Side.Reverse()

		switch trade.Side {
		case types.SideTypeBuy:
			price = ask
			fiatDelta = fiatQuantity.Neg()
		case types.SideTypeSell:
			price = bid
			fiatDelta = fiatQuantity
		default:
			return
		}

		// 3) convert source trade into fiat trade as the average cost of the fiat market position
		// This assumes source's quote currency is the fiat market's base currency
		fiatTrade := s.fiatMarket.newMockTrade(fiatSide, price, fiatQuantity, trade.Time.Time())
		if profit, netProfit, madeProfit := s.fiatMarket.position.AddTrade(fiatTrade); madeProfit {
			// TODO: record the profits in somewhere?
			_ = profit
			_ = netProfit
		}

		// add position delta to let the fiat market hedge the position
		s.fiatMarket.positionDeltaC <- fiatDelta
	})

	s.fiatMarket.tradeCollector.OnTrade(func(trade types.Trade, _, _ fixedpoint.Value) {
		avgCost := s.sourceMarket.position.AverageCost

		// convert the trade quantity to the source market's base currency quantity
		// calculate how much base quantity we can close the source position
		baseQuantity := fixedpoint.Zero
		if s.sourceMarket.market.QuoteCurrency == s.fiatMarket.market.BaseCurrency {
			baseQuantity = trade.Quantity.Div(avgCost)
		} else {
			// To handle the case where the source market's quote currency is not the fiat market's base currency:
			// Assume it's TWD/USDT, the exchange rate is 0.03125 when USDT/TWD is at 32.0,
			// and trade quantity is in TWD,
			// so we need to convert Quantity from TWD to USDT unit by div
			baseQuantity = avgCost.Div(trade.Quantity.Div(trade.Price))
		}

		// convert the fiat trade into source trade to close the source position
		// side is reversed because we are closing the source hedge position
		mockSourceTrade := s.sourceMarket.newMockTrade(trade.Side.Reverse(), avgCost, baseQuantity, trade.Time.Time())
		if profit, netProfit, madeProfit := s.sourceMarket.position.AddTrade(mockSourceTrade); madeProfit {
			_ = profit
			_ = netProfit
		}

		// close the maker position
		// create a mock trade for closing the maker position
		// This assumes the source market's quote currency is the fiat market's base currency
		price := fixedpoint.Zero
		if s.sourceMarket.market.QuoteCurrency == s.fiatMarket.market.BaseCurrency {
			price = avgCost.Mul(trade.Price)
		} else {
			price = avgCost.Div(trade.Price)
		}

		side := trade.Side
		mockTargetTrade := types.Trade{
			ID:            0,  // TODO: generate a unique ID
			OrderID:       0,  // TODO: like above
			Exchange:      "", // TODO: use the maker exchange name
			Symbol:        "", // TODO: use the maker symbol
			Price:         price,
			Quantity:      baseQuantity,
			QuoteQuantity: baseQuantity.Mul(price),
			Side:          side,
			IsBuyer:       side == types.SideTypeBuy,
			IsMaker:       false,
			Time:          trade.Time,
			Fee:           trade.Fee,         // apply trade fee when possible
			FeeCurrency:   trade.FeeCurrency, // apply trade fee when possible
		}

		// TODO: add to the maker position delta
		_ = mockTargetTrade
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

	c := s.sourceMarket.positionDeltaC
	c <- uncoveredPosition
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

	if err := s.sourceMarket.start(ctx, 3*time.Second); err != nil {
		return err
	}

	if err := s.fiatMarket.start(ctx, 3*time.Second); err != nil {
		return err
	}

	return nil
}
