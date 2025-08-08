package types

import "context"

type SignalProvider interface {
	ID() string
	CalculateSignal(ctx context.Context) (float64, error)
}
