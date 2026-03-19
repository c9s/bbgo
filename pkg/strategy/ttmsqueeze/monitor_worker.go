package ttmsqueeze

import (
	"context"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// monitorWorker monitors the position and recovers from negative (short) positions
// This prevents overshooting when exiting positions
func (s *Strategy) monitorWorker(wg *sync.WaitGroup, ctx context.Context) {
	wg.Add(1)
	defer wg.Done()

	checkDuration := s.LtfInterval.Duration() / 2
	if checkDuration < 15*time.Minute {
		checkDuration = 15 * time.Minute
	}
	ticker := time.NewTicker(checkDuration)
	defer ticker.Stop()

	s.checkAndRecoverPosition(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.logger.Info(s.stateMachine.String())
			bbgo.Notify(s.ProfitStats)
			s.checkAndRecoverPosition(ctx)
		}
	}
}

// checkAndRecoverPosition checks if the position needs recovery and submits orders to restore to zero
// - If position is negative (short): buy to recover
func (s *Strategy) checkAndRecoverPosition(ctx context.Context) {
	base := s.Strategy.Position.GetBase()

	// dust check
	isDust := false
	if currentPrice, ok := s.session.LastPrice(s.Symbol); ok {
		isDust = s.Position.IsDust(currentPrice)
	} else {
		isDust = s.Position.IsDust()
	}

	// Position is negative (short), need to buy to recover
	if base.Sign() < 0 && !isDust {
		s.recoverNegativePosition(ctx, base)
	}
}

// recoverNegativePosition submits a buy order to recover from a negative (short) position
func (s *Strategy) recoverNegativePosition(ctx context.Context, base fixedpoint.Value) {
	if base.Sign() >= 0 {
		return
	}

	if s.recoveryInProgress.Load() {
		return // Already recovering
	}

	quantityNeeded := base.Abs()
	quantityNeeded = s.Market.TruncateQuantity(quantityNeeded)

	s.recoveryInProgress.Store(true)
	defer s.recoveryInProgress.Store(false)

	s.logger.Warnf("detected negative position (base=%s), submitting recovery buy order for %s",
		base.String(), quantityNeeded.String())

	order := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeMarket,
		Quantity: quantityNeeded,
	}

	_, err := s.Strategy.OrderExecutor.SubmitOrders(ctx, order)
	if err != nil {
		s.logger.WithError(err).Error("failed to submit recovery buy order")
	}
}
