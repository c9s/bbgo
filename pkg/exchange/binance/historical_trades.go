package binance

import (
	"context"

	"github.com/c9s/bbgo/pkg/types"
)

func (e *Exchange) QueryHistoricalTrades(ctx context.Context, symbol string, limit uint64) ([]types.Trade, error) {
	req := e.client2.NewGetHistoricalTradesRequest()
	req.Symbol(symbol)
	req.Limit(limit)
	trades, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var result []types.Trade
	for _, t := range trades {
		localTrade, err := toGlobalTrade(t, e.IsMargin)
		if err != nil {
			log.WithError(err).Errorf("cannot convert binance trade: %+v", t)
			continue
		}
		result = append(result, *localTrade)
	}
	result = types.SortTradesAscending(result)
	return result, nil
}
