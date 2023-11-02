package csvsource

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func getMockCsvSourceConfig(equityType EquityType) CsvStreamConfig {
	cfg := CsvStreamConfig{
		Exchange:   types.ExchangeBybit,
		Interval:   types.Interval30m,
		RateLimit:  time.Millisecond * 100,
		CsvPath:    "./testdata",
		Symbol:     "FXSUSDT",
		BaseCoin:   "FXS",
		QuoteCoin:  "USDT",
		StrategyID: "BOLLMaker",
	}
	switch equityType {
	case Derivatives:
		cfg.TakerFeeRate = fixedpoint.NewFromFloat(0.055)
		cfg.MakerFeeRate = fixedpoint.NewFromFloat(0.02)
		return cfg
	}
	cfg.TakerFeeRate = fixedpoint.NewFromFloat(0.1)
	cfg.MakerFeeRate = fixedpoint.NewFromFloat(0.1)
	return cfg
}
