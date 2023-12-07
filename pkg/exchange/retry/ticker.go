package retry

import (
	"context"

	"github.com/c9s/bbgo/pkg/types"
)

func QueryTickerUntilSuccessful(ctx context.Context, ex types.Exchange, symbol string) (ticker *types.Ticker, err error) {
	var op = func() (err2 error) {
		ticker, err2 = ex.QueryTicker(ctx, symbol)
		return err2
	}

	err = GeneralBackoff(ctx, op)
	return ticker, err
}
