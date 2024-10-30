package liquiditymaker

import "github.com/prometheus/client_golang/prometheus"

var generalLabels = []string{"strategy_type", "strategy_id", "exchange", "symbol"}

var spreadMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_spread",
		Help: "",
	}, generalLabels)

var tickerBidMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_ticker_bid",
		Help: "",
	}, generalLabels)

var tickerAskMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_ticker_ask",
		Help: "",
	}, generalLabels)

var openOrderBidExposureInUsdMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_open_order_bid_exposure_in_usd",
		Help: "",
	}, generalLabels)

var openOrderAskExposureInUsdMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_open_order_ask_exposure_in_usd",
		Help: "",
	}, generalLabels)

var askLiquidityAmountMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_ask_liquidity_amount",
		Help: "",
	}, generalLabels)

var bidLiquidityAmountMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_bid_liquidity_amount",
		Help: "",
	}, generalLabels)

var askLiquidityPriceHighMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_ask_liquidity_price_high",
		Help: "",
	}, generalLabels)

var askLiquidityPriceLowMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_ask_liquidity_price_low",
		Help: "",
	}, generalLabels)

var bidLiquidityPriceHighMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_bid_liquidity_price_high",
		Help: "",
	}, generalLabels)

var bidLiquidityPriceLowMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_bid_liquidity_price_low",
		Help: "",
	}, generalLabels)

var midPriceMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_mid_price",
		Help: "",
	}, generalLabels)

var orderPlacementStatusMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_order_placement_status",
		Help: "",
	}, append(generalLabels, "side"))

var liquidityPriceRangeMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "liqmaker_liquidity_price_range",
		Help: "",
	}, generalLabels)

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

		tickerBidMetrics,
		tickerAskMetrics,

		spreadMetrics,
	)
}
