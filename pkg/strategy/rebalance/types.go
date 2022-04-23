package rebalance

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Allocation []struct {
	Currency string           `json:"currency"`
	Weight   fixedpoint.Value `json:"weight"`
}

func (ws Allocation) Currencies() (o []string) {
	for _, w := range ws {
		o = append(o, w.Currency)
	}
	return o
}

func (ws Allocation) Weights() (o types.Float64Slice) {
	for _, w := range ws {
		o = append(o, w.Weight.Float64())
	}
	return o
}

type WeightType string

const (
	CustomWeight    WeightType = "custom"
	EqualWeight     WeightType = "equal"
	MarketCapWeight WeightType = "market"
)
