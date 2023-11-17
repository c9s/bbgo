package grid2

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/max"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

type SyncActiveOrdersOpts struct {
	logger            *logrus.Entry
	metricsLabels     prometheus.Labels
	activeOrderBook   *bbgo.ActiveOrderBook
	orderQueryService types.ExchangeOrderQueryService
	exchange          types.Exchange
}

func (s *Strategy) initializeRecoverC() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	isInitialize := false

	if s.recoverC == nil {
		s.logger.Info("initializing recover channel")
		s.recoverC = make(chan struct{}, 1)
	} else {
		s.logger.Info("recover channel is already initialized, trigger active orders recover")
		isInitialize = true

		select {
		case s.recoverC <- struct{}{}:
			s.logger.Info("trigger active orders recover")
		default:
			s.logger.Info("activeOrdersRecoverC is full")
		}
	}

	return isInitialize
}

func (s *Strategy) recoverActiveOrdersPeriodically(ctx context.Context) {
	// every time we activeOrdersRecoverC receive signal, do active orders recover
	if isInitialize := s.initializeRecoverC(); isInitialize {
		return
	}

	// make ticker's interval random in 25 min ~ 35 min
	interval := util.MillisecondsJitter(25*time.Minute, 10*60*1000)
	s.logger.Infof("[ActiveOrderRecover] interval: %s", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	opts := SyncActiveOrdersOpts{
		logger:            s.logger,
		metricsLabels:     s.newPrometheusLabels(),
		activeOrderBook:   s.orderExecutor.ActiveMakerOrders(),
		orderQueryService: s.orderQueryService,
		exchange:          s.session.Exchange,
	}

	var lastRecoverTime time.Time

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			s.recoverC <- struct{}{}
		case <-s.recoverC:
			if !time.Now().After(lastRecoverTime.Add(10 * time.Minute)) {
				continue
			}

			if err := syncActiveOrders(ctx, opts); err != nil {
				log.WithError(err).Errorf("unable to sync active orders")
			} else {
				lastRecoverTime = time.Now()
			}
		}
	}
}

func isMaxExchange(ex interface{}) bool {
	_, yes := ex.(*max.Exchange)
	return yes
}

func syncActiveOrders(ctx context.Context, opts SyncActiveOrdersOpts) error {
	opts.logger.Infof("[ActiveOrderRecover] syncActiveOrders")

	// only sync orders which is updated over 3 min, because we may receive from websocket and handle it twice
	syncBefore := time.Now().Add(-3 * time.Minute)

	openOrders, err := retry.QueryOpenOrdersUntilSuccessfulLite(ctx, opts.exchange, opts.activeOrderBook.Symbol)
	if err != nil {
		opts.logger.WithError(err).Error("[ActiveOrderRecover] failed to query open orders, skip this time")
		return errors.Wrapf(err, "[ActiveOrderRecover] failed to query open orders, skip this time")
	}

	if metricsNumOfOpenOrders != nil {
		metricsNumOfOpenOrders.With(opts.metricsLabels).Set(float64(len(openOrders)))
	}

	activeOrders := opts.activeOrderBook.Orders()

	openOrdersMap := make(map[uint64]types.Order)
	for _, openOrder := range openOrders {
		openOrdersMap[openOrder.OrderID] = openOrder
	}

	var errs error
	// update active orders not in open orders
	for _, activeOrder := range activeOrders {

		if _, exist := openOrdersMap[activeOrder.OrderID]; exist {
			// no need to sync active order already in active orderbook, because we only need to know if it filled or not.
			delete(openOrdersMap, activeOrder.OrderID)
		} else {
			opts.logger.Infof("[ActiveOrderRecover] found active order #%d is not in the open orders, updating...", activeOrder.OrderID)

			isActiveOrderBookUpdated, err := syncActiveOrder(ctx, opts.activeOrderBook, opts.orderQueryService, activeOrder.OrderID, syncBefore)
			if err != nil {
				opts.logger.WithError(err).Errorf("[ActiveOrderRecover] unable to query order #%d", activeOrder.OrderID)
				errs = multierr.Append(errs, err)
				continue
			}

			if !isActiveOrderBookUpdated {
				opts.logger.Infof("[ActiveOrderRecover] active order #%d is updated in 3 min, skip updating...", activeOrder.OrderID)
			}
		}
	}

	// update open orders not in active orders
	for _, openOrder := range openOrdersMap {
		opts.logger.Infof("found open order #%d is not in active orderbook, updating...", openOrder.OrderID)
		// we don't add open orders into active orderbook if updated in 3 min, because we may receive message from websocket and add it twice.
		if openOrder.UpdateTime.After(syncBefore) {
			opts.logger.Infof("open order #%d is updated in 3 min, skip updating...", openOrder.OrderID)
			continue
		}

		opts.activeOrderBook.Add(openOrder)
		// opts.activeOrderBook.Update(openOrder)
	}

	return errs
}
