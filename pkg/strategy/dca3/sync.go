package dca3

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/util/timejitter"
)

func (s *Strategy) syncPeriodically(ctx context.Context, validateStateC chan struct{}) {
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

	// state validation ticker
	validateStateTicker := time.NewTicker(10 * time.Second)
	defer validateStateTicker.Stop()

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
		case <-validateStateTicker.C:
			s.validateState(ctx)
		case <-validateStateC:
			s.validateState(ctx)
		}
	}
}

func (s *Strategy) validateState(ctx context.Context) {
	if s.stateMachine == nil {
		s.logger.Warn("state machine is not initialized, skipping state validation")
		return
	}

	switch s.stateMachine.GetState() {
	case StateIdleWaiting:
		if time.Now().After(s.startTimeOfNextRound) {
			s.stateMachine.EmitNextState(StateOpenPositionReady)
		}
	case StateOpenPositionReady:
		s.stateMachine.EmitNextState(StateOpenPositionMOQReached)
	case StateTakeProfitOrderReset:
		if time.Since(s.lastTradeReceivedAt) > 10*time.Second {
			s.stateMachine.EmitNextState(StateOpenPositionMOQReached)
		}
	}
}

// syncActiveOrders syncs the active orders (orders in ActiveMakerOrders) with the open orders by QueryOpenOrders API
func (s *Strategy) syncActiveOrders(ctx context.Context) error {
	s.logger.Info("sync active orders...")
	updatedOrders, err := s.OrderExecutor.ActiveMakerOrders().SyncOrders(ctx, s.ExchangeSession.Exchange, 3*time.Minute)
	if err != nil {
		return err
	}

	for _, order := range updatedOrders {
		if s.OrderExecutor.OrderStore().Exists(order.OrderID) {
			s.OrderExecutor.OrderStore().Update(order)
		} else {
			s.OrderExecutor.OrderStore().Add(order)
		}
	}

	return nil
}
