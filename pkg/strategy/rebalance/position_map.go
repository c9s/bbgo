package rebalance

import (
	"github.com/c9s/bbgo/pkg/types"
)

type PositionMap map[string]*types.Position

func NewPositionMap(markets []types.Market) PositionMap {
	m := make(PositionMap)

	for _, market := range markets {
		position := types.NewPositionFromMarket(market)
		position.Strategy = ID
		position.StrategyInstanceID = instanceID(market.Symbol)
		m[market.Symbol] = position
	}

	return m
}
