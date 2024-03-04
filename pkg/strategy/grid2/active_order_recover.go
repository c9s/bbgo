package grid2

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

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

	metricsLabel := s.newPrometheusLabels()

	orderQueryService, ok := s.session.Exchange.(types.ExchangeOrderQueryService)
	if !ok {
		s.logger.Errorf("exchange %s doesn't support ExchangeOrderQueryService, please check it", s.session.ExchangeName)
		return
	}

	opts := common.SyncActiveOrdersOpts{
		Logger:            s.logger,
		Exchange:          s.session.Exchange,
		OrderQueryService: orderQueryService,
		ActiveOrderBook:   s.orderExecutor.ActiveMakerOrders(),
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

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

			openOrders, err := retry.QueryOpenOrdersUntilSuccessfulLite(ctx, s.session.Exchange, s.Symbol)
			if err != nil {
				s.logger.WithError(err).Error("[ActiveOrderRecover] failed to query open orders, skip this time")
				continue
			}

			if metricsNumOfOpenOrders != nil {
				metricsNumOfOpenOrders.With(metricsLabel).Set(float64(len(openOrders)))
			}

			opts.OpenOrders = openOrders

			if err := common.SyncActiveOrders(ctx, opts); err != nil {
				log.WithError(err).Errorf("unable to sync active orders")
			} else {
				lastRecoverTime = time.Now()
			}
		}
	}
}
