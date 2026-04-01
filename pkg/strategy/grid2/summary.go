package grid2

import (
	"encoding/json"

	"github.com/c9s/bbgo/pkg/strategy/common"
)

var _ common.StrategySummarizer = (*Strategy)(nil)

func (s *Strategy) SummaryStats() (map[string]common.TabularStats, map[string]json.Marshaler) {
	tabularStats := map[string]common.TabularStats{
		"grid_profit_stats": s.GridProfitStats,
	}
	return tabularStats, nil
}
