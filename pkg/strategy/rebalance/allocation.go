package rebalance

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type WeightType string

const (
	CustomWeight    WeightType = "custom"
	EqualWeight     WeightType = "equal"
	MarketCapWeight WeightType = "market"
)

type Allocation struct {
	Currencies    []string           `json:"currencies"`
	CustomWeights []fixedpoint.Value `json:"customWeights"`
	WeightType    WeightType         `json:"weightType"`
}

func (a *Allocation) Weights() (o types.Float64Slice) {
	switch a.WeightType {
	case CustomWeight:
		for _, w := range a.CustomWeights {
			o = append(o, w.Float64())
		}
		return o
	case EqualWeight:
		return types.NewOneFloats(len(a.Currencies)).Normalize()
	default:
		panic(fmt.Errorf("unknown weight type: %v", a.WeightType))
	}
}
