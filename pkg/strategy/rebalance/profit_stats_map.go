package rebalance

import "github.com/c9s/bbgo/pkg/types"

type ProfitStatsMap map[string]*types.ProfitStats

func NewProfitStatsMap(markets []types.Market) ProfitStatsMap {
	m := make(ProfitStatsMap)

	for _, market := range markets {
		m[market.Symbol] = types.NewProfitStats(market)
	}

	return m
}
