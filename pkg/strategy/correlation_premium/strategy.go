package correlationpremium

import (
	"context"
	"errors"
	"strings"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/strategy/common"
)

const ID = "correlation-premium"

type Strategy struct {
	*common.Strategy

	SymbolA  string `json:"symbolA"`
	SymbolB  string `json:"symbolB"`
	SessionA string `json:"sessionA"`
	SessionB string `json:"sessionB"`
}

func (s *Strategy) ID() string {
	return ID
}
func (s *Strategy) InstanceID() string {
	return strings.Join([]string{ID, s.SessionA, s.SymbolA, s.SessionB, s.SymbolB}, "-")
}

func (s *Strategy) Validate() error {
	if len(s.SymbolA) == 0 {
		return errors.New("symbolA is required")
	}
	if len(s.SymbolB) == 0 {
		return errors.New("symbolB is required")
	}
	if len(s.SessionA) == 0 {
		return errors.New("sessionA is required")
	}
	if len(s.SessionB) == 0 {
		return errors.New("sessionB is required")
	}
	return nil
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {}

func (s *Strategy) CrossRun(ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	return nil
}
