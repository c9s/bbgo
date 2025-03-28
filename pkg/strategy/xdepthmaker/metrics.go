package xdepthmaker

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/prometheus/client_golang/prometheus"
)

var spreadRatioMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_xdepthmaker_market_spread_ratio",
		Help: "the spread ratio of the market",
	},
	[]string{"strategy_type", "strategy_id", "exchange", "symbol"},
)

var orderBookMaxPriceSpacingMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_xdepthmaker_order_book_max_price_spacing",
		Help: "the max price spacing of open orders on the market",
	},
	[]string{"strategy_type", "strategy_id", "exchange", "symbol", "side"},
)

var orderBookInRangeCountMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_xdepthmaker_order_book_count",
		Help: "the number of in-range orders on the market",
	},
	[]string{"strategy_type", "strategy_id", "exchange", "symbol", "side", "price_range"},
)

var marketDepthInUsdMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_xdepthmaker_depth_in_usd",
		Help: "the market depth in USD",
	},
	[]string{"strategy_type", "strategy_id", "exchange", "symbol", "side", "price_range"},
)

func init() {
	prometheus.MustRegister(
		spreadRatioMetrics,
		orderBookInRangeCountMetrics,
		orderBookMaxPriceSpacingMetrics,
		marketDepthInUsdMetrics,
	)
}

func (s *Strategy) newUpdateMetrics(exchangeName, symbol string, priceRange fixedpoint.Value) func(*types.StreamOrderBook, types.SliceOrderBook) {
	return func(book *types.StreamOrderBook, _ types.SliceOrderBook) {
		bestBid, bestAsk, hasPrice := book.BestBidAndAsk()
		if !hasPrice {
			return
		}
		labels := prometheus.Labels{
			"strategy_type": ID,
			"strategy_id":   s.InstanceID(),
			"exchange":      exchangeName,
			"symbol":        symbol,
		}
		updateSpreadRatioMetrics(
			bestBid.Price,
			bestAsk.Price,
			labels,
		)

		midPrice := bestBid.Price.Add(bestAsk.Price).Div(fixedpoint.Two)
		for _, side := range []types.SideType{types.SideTypeBuy, types.SideTypeSell} {
			updateOrderBookMetrics(
				book.SideBook(side),
				side,
				midPrice,
				priceRange,
				labels,
			)
			updateMarketDepthInUsd(
				book.SideBook(side),
				side,
				midPrice,
				priceRange,
				labels,
			)
		}
	}
}

func updateSpreadRatioMetrics(bestBidPrice, bestAskPrice fixedpoint.Value, labels prometheus.Labels) {
	spreadRatio := bestAskPrice.Sub(bestBidPrice).Div(bestBidPrice)
	spreadRatioMetrics.With(labels).Set(spreadRatio.Float64())
}

func inRange(price, refPrice, priceRange fixedpoint.Value) bool {
	priceRatio := price.Div(refPrice).Sub(fixedpoint.One).Abs()
	return priceRatio.Compare(priceRange) < 0
}

func updateOrderBookMetrics(
	book types.PriceVolumeSlice,
	side types.SideType,
	midPrice, priceRange fixedpoint.Value,
	labels prometheus.Labels,
) {
	inRangeOrderCount := 0
	maxPriceSpacing := fixedpoint.Zero

	for idx, priceVolume := range book {
		price := priceVolume.Price
		if inRange(price, midPrice, priceRange) {
			inRangeOrderCount++
		}
		if idx > 0 {
			prevPrice := book[idx-1].Price
			priceSpacing := price.Sub(prevPrice).Abs().Div(price)
			if priceSpacing.Compare(maxPriceSpacing) > 0 {
				maxPriceSpacing = priceSpacing
			}
		}
	}
	orderBookMaxPriceSpacingMetrics.
		MustCurryWith(labels).
		With(prometheus.Labels{
			"side": string(side),
		}).
		Set(maxPriceSpacing.Float64())
	orderBookInRangeCountMetrics.
		MustCurryWith(labels).
		With(
			prometheus.Labels{
				"side":        string(side),
				"price_range": priceRange.String(),
			},
		).
		Set(float64(inRangeOrderCount))
}

func updateMarketDepthInUsd(
	book types.PriceVolumeSlice,
	side types.SideType,
	midPrice, priceRange fixedpoint.Value,
	labels prometheus.Labels,
) {
	depthInUsd := fixedpoint.Zero
	for _, priceVolume := range book {
		price := priceVolume.Price
		if inRange(price, midPrice, priceRange) {
			depthInUsd = depthInUsd.Add(priceVolume.InQuote())
		}
	}
	marketDepthInUsdMetrics.
		MustCurryWith(labels).
		With(
			prometheus.Labels{
				"side":        string(side),
				"price_range": priceRange.String(),
			},
		).
		Set(depthInUsd.Float64())
}
