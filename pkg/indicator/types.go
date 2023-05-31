package indicator

import "github.com/c9s/bbgo/pkg/types"

type Float64Calculator interface {
	Calculate(x float64) float64
}

type Float64Source interface {
	types.Series
	OnUpdate(f func(v float64))
}

type Float64Subscription interface {
	types.Series
	AddSubscriber(f func(v float64))
}
