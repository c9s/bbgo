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

type TradingMarket struct {
	symbol    string
	session   *bbgo.ExchangeSession
	market    types.Market
	stream    types.Stream
	book      *types.StreamOrderBook
	depthBook *types.DepthBook

	position       *types.Position
	orderStore     *core.OrderStore
	tradeCollector *core.TradeCollector
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

	sourceMarket, fiatMarket *TradingMarket

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

	s.sourceMarket, err = initializeTradingMarket(sourceMarket.Symbol, sourceSession, sourceMarket, s.SourceDepthInQuote)
	if err != nil {
		return err
	}

	fiatSession, fiatMarket, err := parseSymbolSelector(s.FiatSymbol, sessions)
	if err != nil {
		return err
	}

	s.fiatMarket, err = initializeTradingMarket(fiatMarket.Symbol, fiatSession, fiatMarket, s.FiatDepthInQuote)
	if err != nil {
		return err
	}

	// when receiving trades from the source session,
	// mock a trade with the quote amount and add to the fiat position
	s.sourceMarket.tradeCollector.OnTrade(func(trade types.Trade, _, _ fixedpoint.Value) {
		// get the fiat book price from here?
		bid, ask := s.fiatMarket.depthBook.BestBidAndAskAtQuoteDepth()
		var price fixedpoint.Value
		var quantity = trade.QuoteQuantity.Abs()
		var side = trade.Side.Reverse()

		switch trade.Side {
		case types.SideTypeBuy:
			price = ask
		case types.SideTypeSell:
			price = bid
		}

		fiatTrade := types.Trade{
			// TODO: update the fields later
			// ID:            trade.ID,
			// Exchange:      trade.Exchange,
			// OrderID:       trade.OrderID,
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
			_ = profit
			_ = netProfit
		}
	})

	return nil
}

func (s *SyntheticHedge) Hedge(ctx context.Context, pos fixedpoint.Value) error {
	// record the fiat price from the fiat market
	fiatBidPrice, fiatAskPrice := s.fiatMarket.depthBook.BestBidAndAskAtQuoteDepth()
	_ = fiatBidPrice
	_ = fiatAskPrice

	// get the source price and submit the hedge order to the source exchange

	// query trades from of the hedge order

	// build up the source hedge position for the average cost

	// submit fiat hedge order to the fiat market to convert the quote

	// merge the positions

	return nil
}

func (s *SyntheticHedge) GetQuotePrices() (fixedpoint.Value, fixedpoint.Value, bool) {
	if s.sourceMarket == nil || s.fiatMarket == nil {
		return fixedpoint.Zero, fixedpoint.Zero, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	bid, ask := s.sourceMarket.depthBook.BestBidAndAskAtQuoteDepth()
	bid2, ask2 := s.fiatMarket.depthBook.BestBidAndAskAtQuoteDepth()

	s.sourceQuotingPrice = &types.Ticker{
		Buy:  bid,
		Sell: ask,
		Time: now,
	}

	s.fiatQuotingPrice = &types.Ticker{
		Buy:  bid2,
		Sell: ask2,
		Time: now,
	}

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

	if err := s.sourceMarket.stream.Connect(ctx); err != nil {
		return err
	}

	if err := s.fiatMarket.stream.Connect(ctx); err != nil {
		return err
	}

	return nil
}

func initializeTradingMarket(
	symbol string,
	session *bbgo.ExchangeSession,
	market types.Market,
	depth fixedpoint.Value,
) (*TradingMarket, error) {
	stream := session.Exchange.NewStream()
	stream.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{Depth: types.DepthLevelFull})

	book := types.NewStreamBook(symbol, session.Exchange.Name())
	book.BindStream(stream)

	depthBook := types.NewDepthBook(book, depth)

	position := types.NewPositionFromMarket(market)

	orderStore := core.NewOrderStore(symbol)
	tradeCollector := core.NewTradeCollector(symbol, position, orderStore)
	if err := tradeCollector.Initialize(); err != nil {
		return nil, err
	}

	tradeCollector.BindStream(session.UserDataStream)

	return &TradingMarket{
		symbol:    symbol,
		session:   session,
		market:    market,
		stream:    stream,
		book:      book,
		depthBook: depthBook,

		position:       position,
		orderStore:     orderStore,
		tradeCollector: tradeCollector,
	}, nil
}
