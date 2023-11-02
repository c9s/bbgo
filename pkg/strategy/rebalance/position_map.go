package rebalance

import (
	"github.com/c9s/bbgo/pkg/types"
)

type PositionMap map[string]*types.Position

func (m PositionMap) CreatePositions(markets map[string]types.Market) PositionMap {
	for symbol, market := range markets {
		if _, ok := m[symbol]; ok {
			continue
		}

		log.Infof("creating position for symbol %s", symbol)
		position := types.NewPositionFromMarket(market)
		position.Strategy = ID
		position.StrategyInstanceID = instanceID(symbol)
		m[symbol] = position
	}
	return m
}
