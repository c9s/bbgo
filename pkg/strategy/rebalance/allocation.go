package rebalance

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/glassnode"
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

	Glassnode *glassnode.Glassnode
}

func (a *Allocation) Weights(ctx context.Context) (o types.Float64Slice) {
	switch a.WeightType {
	case CustomWeight:
		for _, w := range a.CustomWeights {
			o = append(o, w.Float64())
		}
		return o
	case EqualWeight:
		return types.NewOneFloats(len(a.Currencies)).Normalize()
	case MarketCapWeight:
		return a.getMarkcatCapWeights(ctx)
	default:
		panic(fmt.Errorf("unknown weight type: %v", a.WeightType))
	}
}

func (a *Allocation) getMarkcatCapWeights(ctx context.Context) types.Float64Slice {
	var weights types.Float64Slice

	// get market cap values
	for _, currency := range a.Currencies {
		marketCap, err := a.Glassnode.QueryMarketCapInUSD(ctx, currency)
		if err != nil {
			panic(fmt.Errorf("failed to get market cap for %s: %v", currency, err))
		}
		weights = append(weights, marketCap)
	}

	return weights.Normalize()
}
