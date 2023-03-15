package rebalance

import (
	"github.com/c9s/bbgo/pkg/types"
)

type PositionMap map[string]*types.Position

func (m PositionMap) CreatePositions(markets []types.Market) PositionMap {
	for _, market := range markets {
		if _, ok := m[market.Symbol]; ok {
			continue
		}

		log.Infof("creating position for symbol %s", market.Symbol)
		position := types.NewPositionFromMarket(market)
		position.Strategy = ID
		position.StrategyInstanceID = instanceID(market.Symbol)
		m[market.Symbol] = position
	}
	return m
}
