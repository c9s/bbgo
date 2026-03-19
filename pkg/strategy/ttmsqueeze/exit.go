package ttmsqueeze

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// triggerExit triggers an exit, closing the entire position via TWAP
// The TWAP completes in 2 LTF intervals with at most NumExitOrders orders
func (s *Strategy) triggerExit(ctx context.Context) {
	base := s.Strategy.Position.GetBase()
	// Only exit if we have a positive base (long position)
	if base.Sign() <= 0 {
		return
	}

	quantity := base

	// dust check
	isDust := false
	if currentPrice, ok := s.session.LastPrice(s.Symbol); ok {
		isDust = s.Position.IsDust(currentPrice)
	} else {
		isDust = s.Position.IsDust()
	}

	if isDust {
		s.logger.Infof("exit quantity %s is dust, skipping exit", quantity.String())
		return
	}

	numOrders, sliceQuantity := s.calculateOrderParams(quantity, s.ExitConfig.NumOrders)

	// Calculate TWAP parameters for exit:
	// - Total time = 2 * LTF interval
	// - Update interval = total time / numOrders
	ltfDuration := s.LtfInterval.Duration()
	totalExitTime := 2 * ltfDuration
	updateInterval := totalExitTime / time.Duration(numOrders)

	s.logger.Infof("triggering exit: quantity=%s, numOrders=%d, interval=%s, sliceQty=%s, deadline=%s",
		quantity.String(), numOrders, updateInterval, sliceQuantity.String(), totalExitTime)

	if err := s.stateMachine.TransitionToExiting(ctx, quantity, sliceQuantity, updateInterval, totalExitTime); err != nil {
		s.logger.WithError(err).Error("failed to transition to Exiting state")
	}
}

// triggerHardExit performs an immediate exit using market orders
// This is used for max loss exits where speed is critical
func (s *Strategy) triggerHardExit(ctx context.Context) {
	base := s.Strategy.Position.GetBase()
	// Only exit if we have a positive base (long position)
	if base.Sign() <= 0 {
		return
	}

	quantity := base

	// dust check
	isDust := false
	if currentPrice, ok := s.session.LastPrice(s.Symbol); ok {
		isDust = s.Position.IsDust(currentPrice)
	} else {
		isDust = s.Position.IsDust()
	}

	// Check if total quantity is dust
	if isDust {
		s.logger.Infof("hard exit quantity %s is dust, skipping", quantity.String())
		return
	}

	// Cancel any running TWAP and reset state
	s.stateMachine.CancelAndReset(ctx)

	numOrders, sliceQuantity := s.calculateOrderParams(quantity, s.ExitConfig.NumOrders)

	s.logger.Warnf("executing hard exit with %d market orders, total=%s, slice=%s",
		numOrders, quantity.String(), sliceQuantity.String())

	// Submit market sell orders
	remaining := quantity
	for i := 0; i < numOrders && remaining.Sign() > 0; i++ {
		orderQty := sliceQuantity
		// Last order takes the remainder
		if i == numOrders-1 || remaining.Compare(sliceQuantity) <= 0 {
			orderQty = s.Market.TruncateQuantity(remaining)
		}

		if orderQty.Compare(s.Market.MinQuantity) < 0 {
			break
		}

		order := types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     types.SideTypeSell,
			Type:     types.OrderTypeMarket,
			Quantity: orderQty,
		}

		_, err := s.Strategy.OrderExecutor.SubmitOrders(ctx, order)
		if err != nil {
			s.logger.WithError(err).Errorf("failed to submit hard exit market order %d/%d", i+1, numOrders)
		} else {
			s.logger.Infof("hard exit order %d/%d submitted: %s", i+1, numOrders, orderQty.String())
		}

		remaining = remaining.Sub(orderQty)
	}
}

// ExitConfig contains exit-related settings
type ExitConfig struct {
	// MaxLossRatio is the maximum unrealized loss ratio before hard exit (e.g., 0.05 = 5%)
	// Set to 0 to disable loss-based exit
	MaxLossRatio fixedpoint.Value `json:"maxLossRatio"`

	// ConsecutiveExit is the number of consecutive momentum weakening signals required for exit (default: 3)
	ConsecutiveExit int `json:"consecutiveExit"`

	// NumOrders is the number of orders to execute for exiting a position (default: 10)
	// The exit TWAP will complete in 2 LTF intervals using this many orders
	NumOrders int `json:"numOrders"`

	// HardExitCoolDown is the cooldown duration after a hard exit (max loss) before trading resumes
	// During cooldown, no new entries will be made
	HardExitCoolDown types.Duration `json:"hardExitCoolDown"`

	// HardExitCheckInterval is the interval for checking max loss ratio (default: 5m)
	HardExitCheckInterval types.Duration `json:"hardExitCheckInterval"`

	// ClearPositionOnShutdown closes any open position with market orders when the strategy shuts down
	ClearPositionOnShutdown bool `json:"clearPositionOnShutdown"`
}

func (c *ExitConfig) Defaults() {
	if c.MaxLossRatio.IsZero() {
		c.MaxLossRatio = fixedpoint.NewFromFloat(0.1) // default 10% max loss
	}
	if c.ConsecutiveExit == 0 {
		c.ConsecutiveExit = 3
	}
	if c.NumOrders == 0 {
		c.NumOrders = 10
	}
	if c.HardExitCoolDown == 0 {
		c.HardExitCoolDown = types.Duration(time.Hour) // default 1 hour cooldown
	}
	if c.HardExitCheckInterval == 0 {
		c.HardExitCheckInterval = types.Duration(5 * time.Minute) // default 5 minutes
	}
}
