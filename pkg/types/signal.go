package types

import "context"

type SignalProvider interface {
	CalculateSignal(ctx context.Context) (float64, error)
}
