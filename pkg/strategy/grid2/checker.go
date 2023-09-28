package grid2

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/pkg/errors"
)

type ActiveOrderRecover struct {
	strategy *Strategy

	interval time.Duration
}

func NewActiveOrderRecover(strategy *Strategy, interval time.Duration) *ActiveOrderRecover {
	return &ActiveOrderRecover{
		strategy: strategy,
		interval: interval,
	}
}

func (c *ActiveOrderRecover) Run(ctx context.Context) {
	// sleep for a while to wait for recovered
	time.Sleep(util.MillisecondsJitter(5*time.Second, 1000*10))

	if err := c.recover(ctx); err != nil {
		c.strategy.logger.WithError(err).Error("[ActiveOrderRecover] failed to recover avtive orderbook")
	}

	ticker := time.NewTicker(util.MillisecondsJitter(c.interval, 30*60*1000))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.recover(ctx); err != nil {
				c.strategy.logger.WithError(err).Error("[ActiveOrderRecover] failed to recover avtive orderbook")
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *ActiveOrderRecover) recover(ctx context.Context) error {
	recovered := atomic.LoadInt32(&c.strategy.recovered)
	if recovered == 0 {
		c.strategy.logger.Infof("[ActiveOrderRecover] skip recovering active orders because recover not ready")
		return nil
	}

	c.strategy.logger.Infof("[ActiveOrderRecover] recovering active orders with open orders")

	if c.strategy.getGrid() == nil {
		return nil
	}

	c.strategy.mu.Lock()
	defer c.strategy.mu.Unlock()

	openOrders, err := c.strategy.session.Exchange.QueryOpenOrders(ctx, c.strategy.Symbol)
	if err != nil {
		return errors.Wrapf(err, "[ActiveOrderRecover] failed to query open orders")
	}

	activeOrderBook := c.strategy.orderExecutor.ActiveMakerOrders()
	activeOrders := activeOrderBook.Orders()

	openOrdersMap := make(map[uint64]types.Order)
	for _, openOrder := range openOrders {
		openOrders[openOrder.OrderID] = openOrder
	}

	// update active orders not in open orders
	for _, activeOrder := range activeOrders {
		if _, exist := openOrdersMap[activeOrder.OrderID]; !exist {
			c.strategy.logger.Infof("find active order (%d) not in open orders, updating...", activeOrder.OrderID)
			delete(openOrdersMap, activeOrder.OrderID)
			updatedOrder, err := retry.QueryOrderUntilSuccessful(ctx, c.strategy.orderQueryService, types.OrderQuery{
				Symbol:  activeOrder.Symbol,
				OrderID: strconv.FormatUint(activeOrder.OrderID, 10),
			})

			if err != nil {
				c.strategy.logger.WithError(err).Errorf("[ActiveOrderRecover] unable to query order (%d)", activeOrder.OrderID)
				continue
			}

			activeOrderBook.Update(*updatedOrder)
		}
	}

	// update open orders not in active orders
	for _, openOrders := range openOrdersMap {
		activeOrderBook.Update(openOrders)
	}

	return nil
}
