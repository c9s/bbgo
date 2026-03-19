package ttmsqueeze

import (
	"context"
	"sync"
	"time"
)

// riskWorker periodically checks if unrealized loss exceeds max loss ratio
func (s *Strategy) riskWorker(wg *sync.WaitGroup, ctx context.Context) {
	wg.Add(1)
	defer wg.Done()

	// Skip if max loss ratio is disabled
	if s.ExitConfig.MaxLossRatio.IsZero() {
		return
	}

	ticker := time.NewTicker(s.ExitConfig.HardExitCheckInterval.Duration())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Skip if already idle
			if s.stateMachine.GetState() == StateIdle {
				continue
			}

			// Get current position
			position := s.Strategy.Position
			currentPrice, ok := s.session.LastPrice(s.Symbol)
			if !ok || currentPrice.IsZero() {
				s.logger.Warn("failed to get current price for hard exit check")
				continue
			}

			// Only check if we have a long position
			if position.IsDust(currentPrice) || !position.IsLong() {
				continue
			}

			// Calculate ROI - negative ROI means loss
			roi := position.ROI(currentPrice)

			// Check if loss exceeds max loss ratio (ROI < -MaxLossRatio)
			maxLossThreshold := s.ExitConfig.MaxLossRatio.Neg()
			if roi.Compare(maxLossThreshold) < 0 {
				s.logger.Warnf("HARD EXIT: ROI %.2f%% exceeds max loss %.2f%%, triggering immediate exit with cooldown",
					roi.Float64()*100, maxLossThreshold.Float64()*100)

				// Record hard exit time for cooldown
				now := time.Now()
				s.setHardExitTime(now)
				s.statsWorker.HardExitC <- now

				// Trigger immediate market order exit
				s.triggerHardExit(ctx)
			}
		}
	}
}
