package bbgo

import (
	"context"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/sigchan"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type TradeCollector
type TradeCollector struct {
	Symbol   string
	orderSig sigchan.Chan

	tradeStore *TradeStore
	tradeC     chan types.Trade
	position   *Position
	orderStore *OrderStore

	tradeCallbacks          []func(trade types.Trade)
	positionUpdateCallbacks []func(position *Position)
	profitCallbacks         []func(trade types.Trade, profit, netProfit fixedpoint.Value)
}

func NewTradeCollector(symbol string, position *Position, orderStore *OrderStore) *TradeCollector {
	return &TradeCollector{
		Symbol:   symbol,
		orderSig: sigchan.New(1),

		tradeC:     make(chan types.Trade, 100),
		tradeStore: NewTradeStore(symbol),
		position:   position,
		orderStore: orderStore,
	}
}

// handleTradeUpdateInBackground sends the trade object to the trade channel,
// so that the goroutine can receive the trade and process in the background.
func (c *TradeCollector) handleTradeUpdateInBackground(trade types.Trade) {
	c.tradeC <- trade
}

// BindStreamForBackground bind the stream callback for background processing
func (c *TradeCollector) BindStreamForBackground(stream types.Stream) {
	stream.OnTradeUpdate(c.handleTradeUpdateInBackground)
}

func (c *TradeCollector) BindStream(stream types.Stream) {
	stream.OnTradeUpdate(c.processTrade)
}

// Emit triggers the trade processing (position update)
// If you sent order, and the order store is updated, you can call this method
// so that trades will be processed in the next round of the goroutine loop
func (c *TradeCollector) Emit() {
	c.orderSig.Emit()
}

// Process filters the received trades and see if there are orders matching the trades
// if we have the order in the order store, then the trade will be considered for the position.
// profit will also be calculated.
func (c *TradeCollector) Process() bool {
	positionChanged := false
	c.tradeStore.Filter(func(trade types.Trade) bool {
		if c.orderStore.Exists(trade.OrderID) {
			c.EmitTrade(trade)
			if profit, netProfit, madeProfit := c.position.AddTrade(trade); madeProfit {
				c.EmitProfit(trade, profit, netProfit)
			}
			positionChanged = true
			return true
		}
		return false
	})
	if positionChanged {
		c.EmitPositionUpdate(c.position)
	}

	return positionChanged
}

func (c *TradeCollector) processTrade(trade types.Trade) {
	if c.orderStore.Exists(trade.OrderID) {
		c.EmitTrade(trade)
		if profit, netProfit, madeProfit := c.position.AddTrade(trade); madeProfit {
			c.EmitProfit(trade, profit, netProfit)
		}
		c.EmitPositionUpdate(c.position)
	} else {
		c.tradeStore.Add(trade)
	}
}

// Run is a goroutine executed in the background
func (c *TradeCollector) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-c.orderSig:
			c.Process()

		case trade := <-c.tradeC:
			c.processTrade(trade)
		}
	}
}
