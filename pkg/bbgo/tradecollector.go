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
	profitCallbacks []func(trade types.Trade, profit, netProfit fixedpoint.Value)
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

func (c *TradeCollector) handleTradeUpdate(trade types.Trade) {
	c.tradeC <- trade
}

func (c *TradeCollector) BindStream(stream types.Stream) {
	stream.OnTradeUpdate(c.handleTradeUpdate)
}

func (c *TradeCollector) Emit() {
	c.orderSig.Emit()
}

func (c *TradeCollector) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-c.orderSig:
			trades := c.tradeStore.GetAndClear()
			for _, trade := range trades {
				if c.orderStore.Exists(trade.OrderID) {
					c.EmitTrade(trade)
					if profit, netProfit, madeProfit := c.position.AddTrade(trade) ; madeProfit {
						c.EmitProfit(trade, profit, netProfit)
					}
				}
			}
			c.EmitPositionUpdate(c.position)

		case trade := <-c.tradeC:
			c.tradeStore.Add(trade)
		}
	}
}
