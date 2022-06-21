// Code generated by "callbackgen -type TradeCollector"; DO NOT EDIT.

package bbgo

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func (c *TradeCollector) OnRecover(cb func(trade types.Trade)) {
	c.recoverCallbacks = append(c.recoverCallbacks, cb)
}

func (c *TradeCollector) EmitRecover(trade types.Trade) {
	for _, cb := range c.recoverCallbacks {
		cb(trade)
	}
}

func (c *TradeCollector) OnTrade(cb func(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value)) {
	c.tradeCallbacks = append(c.tradeCallbacks, cb)
}

func (c *TradeCollector) EmitTrade(trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value) {
	for _, cb := range c.tradeCallbacks {
		cb(trade, profit, netProfit)
	}
}

func (c *TradeCollector) OnPositionUpdate(cb func(position *types.Position)) {
	c.positionUpdateCallbacks = append(c.positionUpdateCallbacks, cb)
}

func (c *TradeCollector) EmitPositionUpdate(position *types.Position) {
	for _, cb := range c.positionUpdateCallbacks {
		cb(position)
	}
}

func (c *TradeCollector) OnProfit(cb func(trade types.Trade, profit *types.Profit)) {
	c.profitCallbacks = append(c.profitCallbacks, cb)
}

func (c *TradeCollector) EmitProfit(trade types.Trade, profit *types.Profit) {
	for _, cb := range c.profitCallbacks {
		cb(trade, profit)
	}
}
