package bbgo

import "context"

type SignalProvider interface {
	ID() string

	Subscribe(session *ExchangeSession, symbol string)

	Bind(ctx context.Context, session *ExchangeSession, symbol string) error

	CalculateSignal(ctx context.Context) (float64, error)
}
