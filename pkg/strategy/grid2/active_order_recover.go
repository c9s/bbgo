package grid2

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

func (s *Strategy) recoverActiveOrdersPeriodically(ctx context.Context) {
	// every time we activeOrdersRecoverCh receive signal, do active orders recover
	s.activeOrdersRecoverCh = make(chan struct{}, 1)

	// make ticker's interval random in 25 min ~ 35 min
	interval := util.MillisecondsJitter(25*time.Minute, 10*60*1000)
	s.logger.Infof("[ActiveOrderRecover] interval: %s", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {

		case <-ctx.Done():
			return

		case <-ticker.C:
			if err := s.syncActiveOrders(ctx); err != nil {
				log.WithError(err).Errorf("unable to sync active orders")
			}

		case <-s.activeOrdersRecoverCh:
			if err := s.syncActiveOrders(ctx); err != nil {
				log.WithError(err).Errorf("unable to sync active orders")
			}

		}
	}
}

func (s *Strategy) syncActiveOrders(ctx context.Context) error {
	s.logger.Infof("[ActiveOrderRecover] syncActiveOrders")

	recovered := atomic.LoadInt32(&s.recovered)
	if recovered == 0 {
		s.logger.Infof("[ActiveOrderRecover] skip recovering active orders because recover not ready")
		return nil
	}

	if s.getGrid() == nil {
		return nil
	}

	openOrders, err := retry.QueryOpenOrdersUntilSuccessful(ctx, s.session.Exchange, s.Symbol)
	if err != nil {
		s.logger.WithError(err).Error("[ActiveOrderRecover] failed to query open orders, skip this time")
		return err
	}

	metricsNumOfOpenOrders.With(s.newPrometheusLabels()).Set(float64(len(openOrders)))

	// open orders query time + 1 min means this open orders may be changed !
	openOrdersExpiredTime := time.Now().Add(1 * time.Minute)

	s.mu.Lock()
	defer s.mu.Unlock()

	activeOrderBook := s.orderExecutor.ActiveMakerOrders()
	activeOrders := activeOrderBook.Orders()

	openOrdersMap := make(map[uint64]types.Order)
	for _, openOrder := range openOrders {
		openOrdersMap[openOrder.OrderID] = openOrder
	}

	// update active orders not in open orders
	var openOrdersExpired bool = false
	for _, activeOrder := range activeOrders {
		if _, exist := openOrdersMap[activeOrder.OrderID]; exist {
			if openOrdersExpired || time.Now().After(openOrdersExpiredTime) {
				openOrdersExpired = true
				s.logger.Infof("active order #%d is in the open orders but the update_at is over 1 min, updating...", activeOrder.OrderID)
				if err := s.syncActiveOrder(ctx, activeOrderBook, activeOrder.OrderID); err != nil {
					s.logger.WithError(err).Errorf("[ActiveOrderRecover] unable to query order #%d", activeOrder.OrderID)
					continue
				}
			}

			delete(openOrdersMap, activeOrder.OrderID)
		} else {
			s.logger.Infof("found active order #%d is not in the open orders, updating...", activeOrder.OrderID)

			if err := s.syncActiveOrder(ctx, activeOrderBook, activeOrder.OrderID); err != nil {
				s.logger.WithError(err).Errorf("[ActiveOrderRecover] unable to query order #%d", activeOrder.OrderID)
				continue
			}
		}
	}

	// update open orders not in active orders
	for _, openOrder := range openOrdersMap {
		activeOrderBook.Update(openOrder)
	}

	return nil
}

func (s *Strategy) syncActiveOrder(ctx context.Context, activeOrderBook *bbgo.ActiveOrderBook, orderID uint64) error {
	updatedOrder, err := retry.QueryOrderUntilSuccessful(ctx, s.orderQueryService, types.OrderQuery{
		Symbol:  s.Symbol,
		OrderID: strconv.FormatUint(orderID, 10),
	})

	if err != nil {
		return err
	}

	activeOrderBook.Update(*updatedOrder)

	return nil
}
