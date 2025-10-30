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
			s.logger.Info("stopping periodic sync due to context done")
			return
		case <-syncMarketsTicker.C:
			s.logger.Info("updating markets info...")
			if err := s.ExchangeSession.UpdateMarkets(ctx); err != nil {
				s.logger.WithError(err).Warn("failed to update markets")
			}
			s.logger.Info("[DEBUG] markets info updated")
		case <-syncPersistenceTicker.C:
			s.logger.Info("syncing persistence...")
			bbgo.Sync(ctx, s)
			s.logger.Info("[DEBUG] persistence synced")
		case <-syncActiveOrdersTicker.C:
			s.logger.Info("sync active orders...")
			if err := s.syncActiveOrders(ctx); err != nil {
				s.logger.WithError(err).Warn("failed to sync active orders")
			}
			s.logger.Info("[DEBUG] active orders synced")
		case <-validateStateTicker.C:
			s.logger.Infof("validating state... (current state: %d)", s.stateMachine.state)
			s.validateState(ctx)
			s.logger.Info("[DEBUG] state validated")
		case <-validateStateC:
			s.logger.Infof("validating state triggered... (current state: %d)", s.stateMachine.state)
			s.validateState(ctx)
			s.logger.Info("[DEBUG] state validated")
		}
	}
}

func (s *Strategy) validateState(ctx context.Context) {
	if s.stateMachine == nil {
		s.logger.Warn("state machine is not initialized, skipping state validation")
		return
	}

	s.logger.Info("[DEBUG] validate state")
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
	case StateTakeProfitReached:
		if isStucked, err := s.isStuckedAtTakeProfitReached(ctx); err != nil {
			s.logger.WithError(err).Warn("failed to check if stuck at take profit reached state, skip and wait for next validation")
		} else if isStucked {
			s.logger.Info("detected stuck at take profit reached state, emitting finish take profit stage")
			s.stateMachine.EmitNextState(StateIdleWaiting)
		}

		s.logger.Info("[DEBUG] take profit reached state is valid, no action needed")
	}
}

// syncActiveOrders syncs the active orders (orders in ActiveMakerOrders) with the open orders by QueryOpenOrders API
func (s *Strategy) syncActiveOrders(ctx context.Context) error {
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

func (s *Strategy) isStuckedAtTakeProfitReached(ctx context.Context) (bool, error) {
	s.logger.Info("[DEBUG] checking active orders for TakeProfitReached state")
	if s.OrderExecutor.ActiveMakerOrders().NumOfOrders() > 0 {
		// there are still active orders, not stuck at TakeProfitReachedState
		return false, nil
	}

	s.logger.Info("[DEBUG] querying open orders from exchange")
	openOrders, err := s.ExchangeSession.Exchange.QueryOpenOrders(ctx, s.Symbol)
	if err != nil {
		return false, err
	}

	s.logger.Info("[DEBUG] checking open orders from exchange")
	if len(openOrders) > 0 {
		if s.DisableOrderGroupIDFilter {
			s.logger.Info("[DEBUG] order group ID filter is disabled")
			// if order group ID filter is disabled, it means we treat all open orders as active orders belonging to this strategy
			// so do nothing because there is still active orders
			return false, nil
		}

		for _, order := range openOrders {
			if order.GroupID == s.OrderGroupID {
				s.logger.Info("[DEBUG] found open orders belonging to this strategy")
				// there are still active orders belonging to this strategy, so do nothing
				return false, nil
			}
		}
	}

	s.logger.Info("[DEBUG] no active orders found, considered stuck at TakeProfitReached state")
	return true, nil
}
