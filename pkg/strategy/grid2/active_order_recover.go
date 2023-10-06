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
	// every time we activeOrdersRecoverCh receive signal, do active orders recover
	s.activeOrdersRecoverCh = make(chan struct{}, 1)

	// make ticker's interval random in 40 min ~ 70 min
	interval := util.MillisecondsJitter(40*time.Minute, 30*60*1000)
	s.logger.Infof("[ActiveOrderRecover] interval: %s", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.queryOpenOrdersThenRecoverActiveOrders(ctx)
		case <-s.activeOrdersRecoverCh:
			s.queryOpenOrdersThenRecoverActiveOrders(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (s *Strategy) queryOpenOrdersThenRecoverActiveOrders(ctx context.Context) {
	if openOrders, err := retry.QueryOpenOrdersUntilSuccessful(ctx, s.session.Exchange, s.Symbol); err != nil {
		s.logger.WithError(err).Error("[ActiveOrderRecover] failed to query open orders, skip this time")
	} else {
		if err := s.recoverActiveOrdersWithOpenOrders(ctx, openOrders); err != nil {
			s.logger.WithError(err).Error("[ActiveOrderRecover] failed to recover avtive orderbook")
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
		openOrdersMap[openOrder.OrderID] = openOrder
	}

	// update active orders not in open orders
	for _, activeOrder := range activeOrders {
		if _, exist := openOrdersMap[activeOrder.OrderID]; !exist {
			s.logger.Infof("find active order (%d) not in open orders, updating...", activeOrder.OrderID)
			updatedOrder, err := retry.QueryOrderUntilSuccessful(ctx, s.orderQueryService, types.OrderQuery{
				Symbol:  activeOrder.Symbol,
				OrderID: strconv.FormatUint(activeOrder.OrderID, 10),
			})

			if err != nil {
				s.logger.WithError(err).Errorf("[ActiveOrderRecover] unable to query order (%d)", activeOrder.OrderID)
				continue
			}

			activeOrderBook.Update(*updatedOrder)
		} else {
			delete(openOrdersMap, activeOrder.OrderID)
		}
	}

	// TODO: should we add open orders back into active orderbook ?
	// update open orders not in active orders
	for _, openOrder := range openOrdersMap {
		activeOrderBook.Update(openOrder)
	}

	return nil
}
