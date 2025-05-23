package xmaker

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/bbgo"
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

	return nil
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
	return &TradingMarket{
		symbol:    symbol,
		session:   session,
		market:    market,
		stream:    stream,
		book:      book,
		depthBook: depthBook,
	}, nil
}
