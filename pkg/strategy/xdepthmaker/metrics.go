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

var orderBookInRangePriceLevelCountMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_xdepthmaker_price_level_count",
		Help: "the number of in-range price level on the market",
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
		orderBookInRangePriceLevelCountMetrics,
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
			sideBook := book.SideBook(side)
			updateOrderBookMetrics(
				sideBook,
				side,
				midPrice,
				priceRange,
				labels,
			)
			updateMarketDepthInUsd(
				sideBook,
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

func priceLbdUbd(refPrice, priceRange fixedpoint.Value) (lbd fixedpoint.Value, ubd fixedpoint.Value) {
	lbd = refPrice.Mul(fixedpoint.One.Sub(priceRange))
	ubd = refPrice.Mul(fixedpoint.One.Add(priceRange))
	return lbd, ubd
}

func updateOrderBookMetrics(
	book types.PriceVolumeSlice,
	side types.SideType,
	midPrice, priceRange fixedpoint.Value,
	labels prometheus.Labels,
) {
	inRangePriceLevelCount := 0

	lbd, ubd := priceLbdUbd(midPrice, priceRange)
	for _, priceVolume := range book {
		price := priceVolume.Price
		if price.Compare(lbd) >= 0 && price.Compare(ubd) <= 0 {
			inRangePriceLevelCount++
		}
	}
	orderBookInRangePriceLevelCountMetrics.
		MustCurryWith(labels).
		With(
			prometheus.Labels{
				"side":        string(side),
				"price_range": priceRange.String(),
			},
		).
		Set(float64(inRangePriceLevelCount))
}

func updateMarketDepthInUsd(
	book types.PriceVolumeSlice,
	side types.SideType,
	midPrice, priceRange fixedpoint.Value,
	labels prometheus.Labels,
) {
	depthInUsd := fixedpoint.Zero
	lbd, ubd := priceLbdUbd(midPrice, priceRange)
	for _, priceVolume := range book {
		price := priceVolume.Price
		if price.Compare(lbd) >= 0 && price.Compare(ubd) <= 0 {
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
