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

var openOrderMaxPriceSpacingMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_xdepthmaker_open_order_max_price_spacing",
		Help: "the max price spacing of open orders on the market",
	},
	[]string{"strategy_type", "strategy_id", "exchange", "side", "symbol"},
)

var openOrdersCountMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_xdepthmaker_market_open_order_count",
		Help: "the number of in-range open orders on the market",
	},
	[]string{"strategy_type", "strategy_id", "exchange", "side", "price_range", "symbol"},
)

var marketDepthInUsdMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_xdepthmaker_depth_in_usd",
		Help: "the market depth in USD",
	},
	[]string{"strategy_type", "strategy_id", "exchange", "side", "price_range", "symbol"},
)

func init() {
	prometheus.MustRegister(
		spreadRatioMetrics,
		openOrdersCountMetrics,
		openOrderMaxPriceSpacingMetrics,
		marketDepthInUsdMetrics,
	)
}

func updateSpreadRatioMetrics(bestBidPrice, bestAskPrice fixedpoint.Value, strategyType, strategyID, exchangeName, symbol string) {
	spreadRatio := bestAskPrice.Sub(bestBidPrice).Div(bestBidPrice)
	spreadRatioMetrics.With(prometheus.Labels{
		"strategy_type": strategyType,
		"strategy_id":   strategyID,
		"exchange":      exchangeName,
		"symbol":        symbol,
	}).Set(spreadRatio.Float64())
}

func updateOpenOrderMetrics(
	book types.PriceVolumeSlice,
	side types.SideType,
	midPrice, priceRange fixedpoint.Value,
	strategyType, strategyID, exchangeName, symbol string,
) {
	inRangeOrderCount := 0
	maxPriceSpacing := fixedpoint.Zero

	for idx, priceVolume := range book {
		price := priceVolume.Price
		priceRatio := price.Sub(midPrice).Div(midPrice).Abs().Sub(fixedpoint.One).Abs()
		if priceRatio.Compare(priceRange) < 0 {
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
	sharedLabels := prometheus.Labels{
		"strategy_type": strategyType,
		"strategy_id":   strategyID,
		"exchange":      exchangeName,
		"symbol":        symbol,
		"side":          string(side),
	}
	openOrderMaxPriceSpacingMetrics.
		With(sharedLabels).
		Set(maxPriceSpacing.Float64())
	openOrdersCountMetrics.
		MustCurryWith(sharedLabels).
		With(
			prometheus.Labels{
				"price_range": priceRange.String(),
			},
		).
		Set(float64(inRangeOrderCount))
}

func updateMarketDepthInUsd(
	book types.PriceVolumeSlice,
	side types.SideType,
	midPrice, priceRange fixedpoint.Value,
	strategyType, strategyID, exchangeName, symbol string,
) {
	depthInUsd := fixedpoint.Zero
	for _, priceVolume := range book {
		price := priceVolume.Price
		priceRatio := price.Sub(midPrice).Div(midPrice).Abs()
		if priceRatio.Compare(priceRange) < 0 {
			depthInUsd = depthInUsd.Add(priceVolume.InQuote())
		}
	}
	marketDepthInUsdMetrics.
		With(
			prometheus.Labels{
				"strategy_type": strategyType,
				"strategy_id":   strategyID,
				"exchange":      exchangeName,
				"symbol":        symbol,
				"side":          string(side),
				"price_range":   priceRange.String(),
			},
		).
		Set(depthInUsd.Float64())
}
