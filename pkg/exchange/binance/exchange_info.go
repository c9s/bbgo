package binance

import (
	"context"

	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/pkg/errors"
)

func (e *Exchange) QueryFuturesExchangeInfo(ctx context.Context) (*binanceapi.FuturesExchangeInfo, error) {
	if !e.IsFutures {
		return nil, errors.New("cannot query futures exchange info for a non-futures exchange")
	}

	req := e.futuresClient2.NewFuturesExchangeInfoRequest()
	info, err := req.Do(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query futures exchange info")
	}
	return info, nil
}
