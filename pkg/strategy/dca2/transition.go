package dca2

import (
	"context"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
)

// openPosition will place the open-position orders
// if nextRoundPaused is set to true, it will not place the open-position orders
// if startTimeOfNextRound is not reached, it will not place the open-position orders
// if it place open-position orders successfully, it will update the state to OpenPositionReady and return true to trigger the next state immediately
func (s *Strategy) openPosition(ctx context.Context) error {
	if s.nextRoundPaused {
		return fmt.Errorf("nextRoundPaused is set to true, not placing open-position orders")
	}

	if time.Now().Before(s.startTimeOfNextRound) {
		return fmt.Errorf("startTimeOfNextRound (%s) is not reached yet, not placing open-position orders", s.startTimeOfNextRound.String())
	}

	// validate the stage by orders
	currentRound, err := s.collector.CollectCurrentRound(ctx, recoverSinceLimit)
	if err != nil {
		return fmt.Errorf("failed to collect current round: %w", err)
	}

	if len(currentRound.OpenPositionOrders) > 0 && len(currentRound.TakeProfitOrders) == 0 {
		return fmt.Errorf("it's already in open-position stage, please check it and manually fix it")
	}

	s.logger.Info("[State] openPosition - place open-position orders")
	if err := s.placeOpenPositionOrders(ctx); err != nil {
		return fmt.Errorf("failed to place open-position orders: %w", err)
	}

	return nil
}

// readyToFinishOpenPositionStage will update the state to OpenPositionOrderFilled if there is at least one open-position order filled
// it will not trigger the next state immediately because OpenPositionOrderFilled state only trigger by kline to move to the next state
func (s *Strategy) readyToFinishOpenPositionStage(_ context.Context) error {
	return nil
}

// finishOpenPositionStage will update the state to OpenPositionFinish and return true to trigger the next state immediately
func (s *Strategy) finishOpenPositionStage(_ context.Context) error {
	s.stateMachine.EmitNextState(TakeProfitReady)
	return nil
}

// cancelOpenPositionOrdersAndPlaceTakeProfitOrder will cancel the open-position orders and place the take-profit orders
func (s *Strategy) cancelOpenPositionOrdersAndPlaceTakeProfitOrder(ctx context.Context) error {
	currentRound, err := s.collector.CollectCurrentRound(ctx, recoverSinceLimit)
	if err != nil {
		return fmt.Errorf("failed to collect current round: %w", err)
	}

	if len(currentRound.TakeProfitOrders) > 0 {
		return fmt.Errorf("there is a take-profit order, already in take-profit stage, please check it and manually fix it")
	}

	// cancel open-position orders
	if err := s.OrderExecutor.GracefulCancel(ctx); err != nil {
		return fmt.Errorf("failed to cancel open-position orders: %w", err)
	}

	// place the take-profit order
	if err := s.placeTakeProfitOrder(ctx, currentRound); err != nil {
		return fmt.Errorf("failed to place take-profit order: %w", err)
	}

	return nil
}

// finishTakeProfitStage will update the profit stats and reset the position, then wait the next round
func (s *Strategy) finishTakeProfitStage(ctx context.Context) error {
	// wait 3 seconds to avoid position not update
	time.Sleep(3 * time.Second)

	s.logger.Info("[State] finishTakeProfitStage - start resetting position and calculate quote investment for next round")

	// update profit stats
	if err := s.UpdateProfitStatsUntilSuccessful(ctx); err != nil {
		s.logger.WithError(err).Warn("failed to calculate and emit profit")
	}

	// reset position and open new round for profit stats before position opening
	s.Position.Reset()

	// emit position
	s.OrderExecutor.TradeCollector().EmitPositionUpdate(s.Position)

	// store into redis
	bbgo.Sync(ctx, s)

	// set the start time of the next round
	s.startTimeOfNextRound = time.Now().Add(s.CoolDownInterval.Duration())

	return nil
}
