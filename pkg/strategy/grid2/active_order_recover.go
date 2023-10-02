package grid2

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

func (s *Strategy) recoverActiveOrdersWithOpenOrdersPeriodically(ctx context.Context) {
	// sleep for a while to wait for recovered
	time.Sleep(util.MillisecondsJitter(5*time.Second, 1000*10))

	if openOrders, err := retry.QueryOpenOrdersUntilSuccessful(ctx, s.session.Exchange, s.Symbol); err != nil {
		s.logger.WithError(err).Error("[ActiveOrderRecover] failed to query open orders, skip this time")
	} else {
		if err := s.recoverActiveOrdersWithOpenOrders(ctx, openOrders); err != nil {
			s.logger.WithError(err).Error("[ActiveOrderRecover] failed to recover avtive orderbook")
		}
	}

	ticker := time.NewTicker(util.MillisecondsJitter(40*time.Minute, 30*60*1000))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if openOrders, err := retry.QueryOpenOrdersUntilSuccessful(ctx, s.session.Exchange, s.Symbol); err != nil {
				s.logger.WithError(err).Error("[ActiveOrderRecover] failed to query open orders, skip this time")
			} else {
				if err := s.recoverActiveOrdersWithOpenOrders(ctx, openOrders); err != nil {
					s.logger.WithError(err).Error("[ActiveOrderRecover] failed to recover avtive orderbook")
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Strategy) recoverActiveOrdersWithOpenOrders(ctx context.Context, openOrders []types.Order) error {
	recovered := atomic.LoadInt32(&s.recovered)
	if recovered == 0 {
		s.logger.Infof("[ActiveOrderRecover] skip recovering active orders because recover not ready")
		return nil
	}

	s.logger.Infof("[ActiveOrderRecover] recovering active orders with open orders")

	if s.getGrid() == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	activeOrderBook := s.orderExecutor.ActiveMakerOrders()
	activeOrders := activeOrderBook.Orders()

	openOrdersMap := make(map[uint64]types.Order)
	for _, openOrder := range openOrders {
		openOrders[openOrder.OrderID] = openOrder
	}

	// update active orders not in open orders
	for _, activeOrder := range activeOrders {
		if _, exist := openOrdersMap[activeOrder.OrderID]; !exist {
			s.logger.Infof("find active order (%d) not in open orders, updating...", activeOrder.OrderID)
			delete(openOrdersMap, activeOrder.OrderID)
			updatedOrder, err := retry.QueryOrderUntilSuccessful(ctx, s.orderQueryService, types.OrderQuery{
				Symbol:  activeOrder.Symbol,
				OrderID: strconv.FormatUint(activeOrder.OrderID, 10),
			})

			if err != nil {
				s.logger.WithError(err).Errorf("[ActiveOrderRecover] unable to query order (%d)", activeOrder.OrderID)
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
