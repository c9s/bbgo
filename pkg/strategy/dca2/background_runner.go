package dca2

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/util"
)

func (s *Strategy) runBackgroundTask(ctx context.Context) {
	s.logger.Info("run background task")

	// recover active orders
	recoverActiveOrdersInterval := util.MillisecondsJitter(10*time.Minute, 5*60*1000)
	recoverActiveOrdersTicker := time.NewTicker(recoverActiveOrdersInterval)
	defer recoverActiveOrdersTicker.Stop()

	// sync strategy
	syncPersistenceTicker := time.NewTicker(1 * time.Hour)
	defer syncPersistenceTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-syncPersistenceTicker.C:
			bbgo.Sync(ctx, s)
		case <-recoverActiveOrdersTicker.C:
			if err := s.recoverActiveOrders(ctx); err != nil {
				s.logger.WithError(err).Warn(err, "failed to recover active orders")
			}
		}
	}
}
