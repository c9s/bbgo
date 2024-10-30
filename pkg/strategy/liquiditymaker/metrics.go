package liquiditymaker

import "github.com/prometheus/client_golang/prometheus"

var openOrderBidExposureInUsdMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_open_order_bid_exposure_in_usd",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var openOrderAskExposureInUsdMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_open_order_ask_exposure_in_usd",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var askLiquidityAmountMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_ask_liquidity_amount",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var bidLiquidityAmountMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_bid_liquidity_amount",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var askLiquidityPriceHighMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_ask_liquidity_price_high",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var askLiquidityPriceLowMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_ask_liquidity_price_low",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var bidLiquidityPriceHighMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_bid_liquidity_price_high",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var bidLiquidityPriceLowMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_bid_liquidity_price_low",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var midPriceMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_mid_price",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var orderPlacementStatusMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_order_placement_status",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol", "side"})

var liquidityPriceRangeMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_liquidity_price_range",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

func init() {
	prometheus.MustRegister(
		openOrderBidExposureInUsdMetrics,
		openOrderAskExposureInUsdMetrics,

		midPriceMetrics,

		askLiquidityAmountMetrics,
		bidLiquidityAmountMetrics,
		liquidityPriceRangeMetrics,

		askLiquidityPriceHighMetrics,
		askLiquidityPriceLowMetrics,

		bidLiquidityPriceHighMetrics,
		bidLiquidityPriceLowMetrics,

		orderPlacementStatusMetrics,
	)
}
