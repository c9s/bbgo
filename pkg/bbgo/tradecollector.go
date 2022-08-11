package bbgo

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

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
	position   *types.Position
	orderStore *OrderStore
	doneTrades map[types.TradeKey]struct{}

	mu sync.Mutex

	recoverCallbacks []func(trade types.Trade)

	tradeCallbacks []func(trade types.Trade, profit, netProfit fixedpoint.Value)

	positionUpdateCallbacks []func(position *types.Position)
	profitCallbacks         []func(trade types.Trade, profit *types.Profit)
}

func NewTradeCollector(symbol string, position *types.Position, orderStore *OrderStore) *TradeCollector {
	return &TradeCollector{
		Symbol:   symbol,
		orderSig: sigchan.New(1),

		tradeC:     make(chan types.Trade, 100),
		tradeStore: NewTradeStore(),
		doneTrades: make(map[types.TradeKey]struct{}),
		position:   position,
		orderStore: orderStore,
	}
}

// OrderStore returns the order store used by the trade collector
func (c *TradeCollector) OrderStore() *OrderStore {
	return c.orderStore
}

// Position returns the position used by the trade collector
func (c *TradeCollector) Position() *types.Position {
	return c.position
}

func (c *TradeCollector) SetPosition(position *types.Position) {
	c.position = position
}

// QueueTrade sends the trade object to the trade channel,
// so that the goroutine can receive the trade and process in the background.
func (c *TradeCollector) QueueTrade(trade types.Trade) {
	c.tradeC <- trade
}

// BindStreamForBackground bind the stream callback for background processing
func (c *TradeCollector) BindStreamForBackground(stream types.Stream) {
	stream.OnTradeUpdate(c.QueueTrade)
}

func (c *TradeCollector) BindStream(stream types.Stream) {
	stream.OnTradeUpdate(func(trade types.Trade) {
		c.ProcessTrade(trade)
	})
}

// Emit triggers the trade processing (position update)
// If you sent order, and the order store is updated, you can call this method
// so that trades will be processed in the next round of the goroutine loop
func (c *TradeCollector) Emit() {
	c.orderSig.Emit()
}

func (c *TradeCollector) Recover(ctx context.Context, ex types.ExchangeTradeHistoryService, symbol string, from time.Time) error {
	trades, err := ex.QueryTrades(ctx, symbol, &types.TradeQueryOptions{
		StartTime: &from,
	})

	if err != nil {
		return err
	}

	for _, td := range trades {
		log.Debugf("processing trade: %s", td.String())
		if c.ProcessTrade(td) {
			log.Infof("recovered trade: %s", td.String())
			c.EmitRecover(td)
		}
	}
	return nil
}

func (c *TradeCollector) setDone(key types.TradeKey) {
	c.mu.Lock()
	c.doneTrades[key] = struct{}{}
	c.mu.Unlock()
}

// Process filters the received trades and see if there are orders matching the trades
// if we have the order in the order store, then the trade will be considered for the position.
// profit will also be calculated.
func (c *TradeCollector) Process() bool {
	positionChanged := false

	c.tradeStore.Filter(func(trade types.Trade) bool {
		key := trade.Key()

		c.mu.Lock()
		defer c.mu.Unlock()

		// if it's already done, remove the trade from the trade store
		if _, done := c.doneTrades[key]; done {
			return true
		}

		if c.orderStore.Exists(trade.OrderID) {
			if c.position != nil {
				profit, netProfit, madeProfit := c.position.AddTrade(trade)
				if madeProfit {
					p := c.position.NewProfit(trade, profit, netProfit)
					c.EmitTrade(trade, profit, netProfit)
					c.EmitProfit(trade, &p)
				} else {
					c.EmitTrade(trade, fixedpoint.Zero, fixedpoint.Zero)
					c.EmitProfit(trade, nil)
				}
				positionChanged = true
			} else {
				c.EmitTrade(trade, fixedpoint.Zero, fixedpoint.Zero)
			}

			c.doneTrades[key] = struct{}{}
			return true
		}
		return false
	})

	if positionChanged && c.position != nil {
		c.EmitPositionUpdate(c.position)
	}

	return positionChanged
}

// processTrade takes a trade and see if there is a matched order
// if the order is found, then we add the trade to the position
// return true when the given trade is added
// return false when the given trade is not added
func (c *TradeCollector) processTrade(trade types.Trade) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := trade.Key()

	// if it's already done, remove the trade from the trade store
	if _, done := c.doneTrades[key]; done {
		return false
	}

	if c.orderStore.Exists(trade.OrderID) {
		if c.position != nil {
			profit, netProfit, madeProfit := c.position.AddTrade(trade)
			if madeProfit {
				p := c.position.NewProfit(trade, profit, netProfit)
				c.EmitTrade(trade, profit, netProfit)
				c.EmitProfit(trade, &p)
			} else {
				c.EmitTrade(trade, fixedpoint.Zero, fixedpoint.Zero)
				c.EmitProfit(trade, nil)
			}
			c.EmitPositionUpdate(c.position)
		} else {
			c.EmitTrade(trade, fixedpoint.Zero, fixedpoint.Zero)
		}

		c.doneTrades[key] = struct{}{}
		return true
	}
	return false
}

// return true when the given trade is added
// return false when the given trade is not added
func (c *TradeCollector) ProcessTrade(trade types.Trade) bool {
	key := trade.Key()
	// if it's already done, remove the trade from the trade store
	if _, done := c.doneTrades[key]; done {
		return false
	}

	if c.processTrade(trade) {
		return true
	}

	c.tradeStore.Add(trade)
	return false
}

// Run is a goroutine executed in the background
// Do not use this function if you need back-testing
func (c *TradeCollector) Run(ctx context.Context) {
	var ticker = time.NewTicker(3 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			c.Process()

		case <-c.orderSig:
			c.Process()

		case trade := <-c.tradeC:
			c.ProcessTrade(trade)
		}
	}
}
