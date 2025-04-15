package dca2

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/util/timejitter"
)

func (s *Strategy) syncPeriodically(ctx context.Context) {
	s.logger.Info("sync periodically")

	// sync persistence
	syncPersistenceTicker := time.NewTicker(1 * time.Hour)
	defer syncPersistenceTicker.Stop()

	// sync active orders
	syncActiveOrdersTicker := time.NewTicker(timejitter.Milliseconds(10*time.Minute, 5*60*1000))
	defer syncActiveOrdersTicker.Stop()

	// sync markets info
	syncMarketsTicker := time.NewTicker(4 * time.Hour)
	defer syncMarketsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-syncMarketsTicker.C:
			if err := s.ExchangeSession.UpdateMarkets(ctx); err != nil {
				s.logger.WithError(err).Warn("failed to update markets")
			}
		case <-syncPersistenceTicker.C:
			bbgo.Sync(ctx, s)
		case <-syncActiveOrdersTicker.C:
			if err := s.syncActiveOrders(ctx); err != nil {
				s.logger.WithError(err).Warn(err, "failed to sync active orders")
			}
		}
	}
}

// syncActiveOrders syncs the active orders (orders in ActiveMakerOrders) with the open orders by QueryOpenOrders API
func (s *Strategy) syncActiveOrders(ctx context.Context) error {
	s.logger.Info("sync active orders...")
	openOrders, err := retry.QueryOpenOrdersUntilSuccessfulLite(ctx, s.ExchangeSession.Exchange, s.Symbol)
	if err != nil {
		s.logger.WithError(err).Warn("failed to query open orders")
		return err
	}

	activeOrders := s.OrderExecutor.ActiveMakerOrders()

	if len(openOrders) != activeOrders.NumOfOrders() {
		s.logger.Warnf("num of open orders (%d) and active orders (%d) is different before active orders recovery, please check it.", len(openOrders), activeOrders.NumOfOrders())
	}

	opts := common.SyncActiveOrdersOpts{
		Logger:            s.logger,
		Exchange:          s.ExchangeSession.Exchange,
		OrderQueryService: s.collector.queryService,
		ActiveOrderBook:   s.OrderExecutor.ActiveMakerOrders(),
		OrderStore:        s.OrderExecutor.OrderStore(),
		OpenOrders:        openOrders,
	}

	return common.SyncActiveOrders(ctx, opts)
}
