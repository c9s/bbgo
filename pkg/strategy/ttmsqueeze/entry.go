package ttmsqueeze

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// EntryConfig contains entry-related settings
type EntryConfig struct {
	// MaxPosition is the maximum position size allowed (required)
	MaxPosition fixedpoint.Value `json:"maxPosition"`

	// Quantity is the max quantity per entry (required)
	Quantity fixedpoint.Value `json:"quantity"`

	// ConsecutiveEntries is the number of consecutive entry signals required before entering (default: 3)
	ConsecutiveEntries int `json:"consecutiveEntries"`

	// NumOrders is the number of orders to execute for entering a position (default: 10)
	NumOrders int `json:"numOrders"`
}

func (c *EntryConfig) Defaults() {
	if c.ConsecutiveEntries == 0 {
		c.ConsecutiveEntries = 3
	}
	if c.NumOrders == 0 {
		c.NumOrders = 10
	}
}

func (c *EntryConfig) Validate() error {
	if c.MaxPosition.IsZero() {
		return fmt.Errorf("entry.maxPosition is required")
	}
	if c.Quantity.IsZero() {
		return fmt.Errorf("entry.quantity is required")
	}
	if c.MaxPosition.Compare(c.Quantity) < 0 {
		return fmt.Errorf("entry.maxPosition must be greater than or equal to entry.quantity")
	}
	return nil
}

func (s *Strategy) openLongPosition(ctx context.Context) {
	quantity := s.calculateEntryQuantity()
	if quantity.IsZero() || quantity.Compare(s.Market.MinQuantity) < 0 {
		s.logger.Infof("calculated quantity %s is too small, skipping entry", quantity.String())
		return
	}

	numOrders, sliceQuantity := s.calculateOrderParams(quantity, s.EntryConfig.NumOrders)

	s.logger.Infof("opening long position with TWAP: quantity=%s, numOrders=%d, slice=%s",
		quantity.String(), numOrders, sliceQuantity.String())

	if err := s.stateMachine.TransitionToLong(ctx, quantity, sliceQuantity); err != nil {
		s.logger.WithError(err).Error("failed to transition to Long state")
	}
}

func (s *Strategy) calculateEntryQuantity() fixedpoint.Value {
	currentBase := s.Strategy.Position.GetBase()
	remaining := s.EntryConfig.MaxPosition.Sub(currentBase)
	if remaining.Sign() <= 0 {
		return fixedpoint.Zero
	}
	quantity := fixedpoint.Min(s.EntryConfig.Quantity, remaining)
	return s.Market.TruncateQuantity(quantity)
}
