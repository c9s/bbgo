package xdepthmaker

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func (s *Strategy) handleMakerBookUpdate(_ types.SliceOrderBook) {
	bestBid, bestAsk, hasPrice := s.makerBook.BestBidAndAsk()
	if hasPrice {
		updateSpreadRatioMetrics(
			bestBid.Price,
			bestAsk.Price,
			ID,
			s.InstanceID(),
			string(s.makerSession.ExchangeName),
			s.Symbol,
		)

		midPrice := bestBid.Price.Add(bestAsk.Price).Div(fixedpoint.Two)
		for _, side := range []types.SideType{types.SideTypeBuy, types.SideTypeSell} {
			updateOpenOrderMetrics(
				s.makerBook.SideBook(side),
				side,
				midPrice,
				prometheusPriceRange,
				ID,
				s.InstanceID(),
				string(s.makerSession.ExchangeName),
				s.Symbol,
			)
			updateMarketDepthInUsd(
				s.makerBook.SideBook(side),
				side,
				midPrice,
				prometheusPriceRange,
				ID,
				s.InstanceID(),
				string(s.makerSession.ExchangeName),
				s.Symbol,
			)
		}
	}
}
