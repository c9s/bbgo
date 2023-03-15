package rebalance

import "github.com/c9s/bbgo/pkg/types"

type ProfitStatsMap map[string]*types.ProfitStats

func (m ProfitStatsMap) CreateProfitStats(markets []types.Market) ProfitStatsMap {
	for _, market := range markets {
		if _, ok := m[market.Symbol]; ok {
			continue
		}

		log.Infof("creating profit stats for symbol %s", market.Symbol)
		m[market.Symbol] = types.NewProfitStats(market)
	}
	return m
}
