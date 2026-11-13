package xalign

import (
	"context"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type DepthQueryService interface {
	QueryDepth(ctx context.Context, symbol string) (types.SliceOrderBook, int64, error)
}

type DepthQueryServiceWithLimit interface {
	QueryDepth(ctx context.Context, symbol string, limit int) (types.SliceOrderBook, int64, error)
}

func (s *Strategy) queryDepthPrice(ctx context.Context, session *bbgo.ExchangeSession, symbol string, side types.SideType) (fixedpoint.Value, bool) {
	var book types.SliceOrderBook
	var err error

	if svc, ok := session.Exchange.(DepthQueryService); ok {
		book, _, err = svc.QueryDepth(ctx, symbol)
	} else if svc, ok := session.Exchange.(DepthQueryServiceWithLimit); ok {
		book, _, err = svc.QueryDepth(ctx, symbol, 0)
	} else {
		return fixedpoint.Zero, false
	}

	if err != nil {
		log.WithError(err).Warnf("unable to query depth for %s on %s", symbol, session.Name)
		return fixedpoint.Zero, false
	}

	var price fixedpoint.Value
	switch side {
	case types.SideTypeBuy:
		price = book.Asks.AverageDepthPriceByQuote(s.DepthInQuote, 0)
	case types.SideTypeSell:
		price = book.Bids.AverageDepthPriceByQuote(s.DepthInQuote, 0)
	}

	if price.IsZero() {
		return fixedpoint.Zero, false
	}
	return price, true
}
