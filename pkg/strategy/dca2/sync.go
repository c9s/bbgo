package dca2

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/util"
)

func (s *Strategy) syncPeriodically(ctx context.Context) {
	s.logger.Info("sync periodically")

	// sync persistence
	syncPersistenceTicker := time.NewTicker(1 * time.Hour)
	defer syncPersistenceTicker.Stop()

	// sync active orders
	syncActiveOrdersTicker := time.NewTicker(util.MillisecondsJitter(10*time.Minute, 5*60*1000))
	defer syncActiveOrdersTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-syncPersistenceTicker.C:
			bbgo.Sync(ctx, s)
		case <-syncActiveOrdersTicker.C:
			if err := s.syncActiveOrders(ctx); err != nil {
				s.logger.WithError(err).Warn(err, "failed to sync active orders")
			}
		}
	}
}

func (s *Strategy) syncActiveOrders(ctx context.Context) error {
	s.logger.Info("recover active orders...")
	openOrders, err := retry.QueryOpenOrdersUntilSuccessfulLite(ctx, s.ExchangeSession.Exchange, s.Symbol)
	if err != nil {
		s.logger.WithError(err).Warn("failed to query open orders")
		return err
	}

	activeOrders := s.OrderExecutor.ActiveMakerOrders()

	// update num of open orders metrics
	if metricsNumOfOpenOrders != nil {
		metricsNumOfOpenOrders.With(baseLabels).Set(float64(len(openOrders)))
	}

	// update num of active orders metrics
	if metricsNumOfActiveOrders != nil {
		metricsNumOfActiveOrders.With(baseLabels).Set(float64(activeOrders.NumOfOrders()))
	}

	if len(openOrders) != activeOrders.NumOfOrders() {
		s.logger.Warnf("num of open orders (%d) and active orders (%d) is different before active orders recovery, please check it.", len(openOrders), activeOrders.NumOfOrders())
	}

	opts := common.SyncActiveOrdersOpts{
		Logger:            s.logger,
		Exchange:          s.ExchangeSession.Exchange,
		OrderQueryService: s.collector.queryService,
		ActiveOrderBook:   activeOrders,
		OpenOrders:        openOrders,
	}

	return common.SyncActiveOrders(ctx, opts)
}
