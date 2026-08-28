package xfundingv2

import (
	"context"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

type AccountUpdater func(context.Context) (*types.Account, error)

type FuturesAccountUpdater struct {
	accountUpdater AccountUpdater

	lastUpdateTime time.Time
	updatePeriod   time.Duration
	C              chan time.Time

	logger    logrus.FieldLogger
	startOnce sync.Once
}

func NewFuturesAccountUpdater(
	accountUpdater AccountUpdater,
	logger logrus.FieldLogger,
	updatePeriod time.Duration,
) *FuturesAccountUpdater {
	return &FuturesAccountUpdater{
		accountUpdater: accountUpdater,
		updatePeriod:   updatePeriod,
		logger:         logger.WithField("component", "futuresAccountUpdater"),
		C:              make(chan time.Time, 5),
	}
}

func (u *FuturesAccountUpdater) Start(ctx context.Context) {
	go u.startOnce.Do(func() {
		defer func() {
			u.logger.Info("futures account updater stopped")
			close(u.C)
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case currentTime, ok := <-u.C:
				if !ok {
					return
				}
				if !u.lastUpdateTime.IsZero() && currentTime.Sub(u.lastUpdateTime) < u.updatePeriod {
					continue
				}
				u.lastUpdateTime = currentTime
				if _, err := u.accountUpdater(ctx); err != nil {
					u.logger.WithError(err).Warn("failed to update futures account")
					continue
				}
			}
		}
	})
}

func (s *Strategy) runFuturesAccountUpdater(ctx context.Context) {
	s.futuresAccountUpdater = NewFuturesAccountUpdater(
		s.futuresSession.UpdateAccount,
		s.logger,
		s.FuturesAccountUpdatePeriod.Duration(),
	)
	s.futuresAccountUpdater.Start(ctx)
}
