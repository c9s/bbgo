package signal

import (
	"context"

	"github.com/c9s/bbgo/pkg/bbgo"
)

type BaseProvider struct {
	Symbol string
}

func (p *BaseProvider) Subscribe(_ *bbgo.ExchangeSession, _ string) {}

func (p *BaseProvider) Initialize(ctx context.Context, config *DynamicConfig) error {
	p.Symbol = config.Symbol
	return nil
}
