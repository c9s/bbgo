package bbgo

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type PositionUpdater -interface
type PositionUpdater struct {
	Position *types.Position

	// Callbacks
	updateBaseCallbacks        []func()
	updateQuoteCallbacks       []func()
	updateAverageCostCallbacks []func()
}

func (s *PositionUpdater) UpdateBase(qty float64) error {
	s.Position.Base = fixedpoint.NewFromFloat(qty)

	s.EmitUpdateBase()

	return nil
}

func (s *PositionUpdater) UpdateQuote(qty float64) error {
	s.Position.Quote = fixedpoint.NewFromFloat(qty)

	s.EmitUpdateQuote()

	return nil
}

func (s *PositionUpdater) UpdateAverageCost(price float64) error {
	s.Position.AverageCost = fixedpoint.NewFromFloat(price)

	s.EmitUpdateAverageCost()

	return nil
}

type StrategyPositionUpdater interface {
	UpdateBase(qty float64) error
	UpdateQuote(qty float64) error
	UpdateAverageCost(price float64) error
}
