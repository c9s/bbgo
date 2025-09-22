package xmaker

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type FastCancel struct {
	Enabled bool `json:"enabled"`

	SourceSession string `json:"session"`
	Symbol        string `json:"symbol"`

	TriggerSpread fixedpoint.Value `json:"triggerSpread"`

	MarketTradeC chan types.Trade

	strategy *Strategy

	// makerExchange is the maker makerExchange
	makerExchange types.Exchange

	// symbol is the maker symbol
	makerSymbol string

	// logger is the logger instance
	logger logrus.FieldLogger

	activeMakerOrders *bbgo.ActiveOrderBook

	marketTradeStream types.Stream
}

func (c *FastCancel) Validate() error {
	if c.TriggerSpread.IsZero() || c.TriggerSpread.Sign() < 0 {
		return fmt.Errorf("triggerSpread must be positive")
	}

	return nil
}

func (c *FastCancel) getSourceSession(sessions map[string]*bbgo.ExchangeSession) (*bbgo.ExchangeSession, error) {
	sourceSession := c.SourceSession
	session, ok := sessions[sourceSession]
	if !ok {
		return nil, fmt.Errorf("source session %s not found", sourceSession)
	}

	return session, nil
}

func (c *FastCancel) handleTrade(trade types.Trade) {
	select {
	case c.MarketTradeC <- trade:
	default: // avoid blocking
	}
}

func (c *FastCancel) InitializeAndBind(sessions map[string]*bbgo.ExchangeSession, strategy *Strategy) error {
	if !c.Enabled {
		return nil
	}

	c.MarketTradeC = make(chan types.Trade, 100)

	c.strategy = strategy
	c.logger = strategy.logger.WithFields(logrus.Fields{
		"feature": "fastcancel",
	})

	c.makerExchange = strategy.makerSession.Exchange
	c.makerSymbol = strategy.MakerSymbol
	c.activeMakerOrders = strategy.activeMakerOrders
	c.marketTradeStream = c.strategy.marketTradeStream

	c.logger.Infof("setting up fast cancel for %s on session %s with trigger spread %s", c.Symbol, c.SourceSession, c.TriggerSpread.String())

	session, err := c.getSourceSession(sessions)
	if err != nil {
		return err
	}

	c.marketTradeStream = c.strategy.marketTradeStream
	if c.marketTradeStream == nil {
		c.logger.Warnf("market trade stream is not set on strategy, creating a new one for %s", c.Symbol)
		sourceSymbol := c.Symbol
		if sourceSymbol == "" {
			sourceSymbol = c.strategy.SourceSymbol
		}

		c.marketTradeStream = bbgo.NewMarketTradeStream(session, sourceSymbol)
		c.marketTradeStream.OnMarketTrade(c.handleTrade)
		if err := c.marketTradeStream.Connect(context.Background()); err != nil {
			return fmt.Errorf("unable to connect to the market trade stream: %w", err)
		}
	} else {
		c.marketTradeStream.OnMarketTrade(c.handleTrade)
	}

	return nil
}

func (c *FastCancel) shouldCancel(quote *Quote, marketTrade types.Trade) (types.SideType, bool) {
	// taker side
	switch marketTrade.Side {
	case types.SideTypeBuy:
		askSpread := marketTrade.Price.Sub(quote.BestAskPrice).
			Div(quote.BestAskPrice)
		if askSpread.Compare(c.TriggerSpread) > 0 {
			return types.SideTypeSell, true
		}

	case types.SideTypeSell:
		bidSpread := quote.BestBidPrice.Sub(marketTrade.Price).
			Div(quote.BestBidPrice)
		if bidSpread.Compare(c.TriggerSpread) > 0 {
			return types.SideTypeBuy, true
		}
	}

	return types.SideTypeNone, false
}

func (c *FastCancel) fastCancel(ctx context.Context, marketTrade types.Trade) {
	lastQuote := c.strategy.getLastQuote()
	if lastQuote == nil {
		return
	}

	if side, shouldCancel := c.shouldCancel(lastQuote, marketTrade); shouldCancel {
		if err := c.executeFastCancel(ctx, c.makerSymbol, side); err != nil {
			c.logger.WithError(err).Warnf("FastCancel: unable to cancel %s %s orders", c.makerSymbol, side.String())
		}
	}
}

func (c *FastCancel) executeFastCancel(ctx context.Context, symbol string, side types.SideType) (err error) {
	c.logger.Infof("FastCancel: cancelling %s %s", c.makerSymbol, side.String())

	if service, ok := c.makerExchange.(cancelOrdersBySymbolSide); ok {
		_, err = service.CancelOrdersBySymbolSide(ctx, symbol, side)
	} else {
		buyOrders, sellOrders := c.activeMakerOrders.Orders().SeparateBySide()
		switch side {
		case types.SideTypeBuy:
			if len(buyOrders) == 0 {
				return nil
			}

			err = c.activeMakerOrders.GracefulCancel(ctx, c.makerExchange, buyOrders...)
		case types.SideTypeSell:
			if len(sellOrders) == 0 {
				return nil
			}

			err = c.activeMakerOrders.GracefulCancel(ctx, c.makerExchange, sellOrders...)
		}
	}

	if err != nil {
		return err
	}

	// FIXME: we might cancelled only one side of the orders, but we clear for both sides here.
	// This is not accurate if we want to handle both sides.
	c.strategy.setLastQuote(nil)
	return nil
}
