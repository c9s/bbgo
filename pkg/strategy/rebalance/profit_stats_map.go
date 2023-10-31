package rebalance

import (
	"github.com/c9s/bbgo/pkg/types"
)

type ProfitStatsMap map[string]*types.ProfitStats

func (m ProfitStatsMap) CreateProfitStats(markets map[string]types.Market) ProfitStatsMap {
	for symbol, market := range markets {
		if _, ok := m[symbol]; ok {
			continue
		}

		log.Infof("creating profit stats for symbol %s", symbol)
		m[symbol] = types.NewProfitStats(market)
	}
	return m
}
