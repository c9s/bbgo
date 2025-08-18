package xgap

import "github.com/c9s/bbgo/pkg/types"

func getPositionSnapshot(
	position *types.Position,
) *types.Position {
	position.Lock()
	defer position.Unlock()

	snapShot := types.Position{
		Symbol:        position.Symbol,
		BaseCurrency:  position.BaseCurrency,
		QuoteCurrency: position.QuoteCurrency,

		Market: position.Market,

		Base:        position.Base,
		Quote:       position.Quote,
		AverageCost: position.AverageCost,

		FeeRate:          position.FeeRate,
		ExchangeFeeRates: position.ExchangeFeeRates,

		TotalFee:        position.TotalFee,
		FeeAverageCosts: position.FeeAverageCosts,

		OpenedAt:  position.OpenedAt,
		ChangedAt: position.ChangedAt,

		Strategy:           position.Strategy,
		StrategyInstanceID: position.StrategyInstanceID,

		AccumulatedProfit: position.AccumulatedProfit,
	}
	return &snapShot
}
