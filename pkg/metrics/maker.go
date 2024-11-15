package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var OpenOrderBidExposureInUsdMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_maker_open_order_bid_exposure_in_usd",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var OpenOrderAskExposureInUsdMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_maker_open_order_ask_exposure_in_usd",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var OpenOrderBidOrderCountMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_maker_open_order_bid_count",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var OpenOrderAskOrderCountMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_maker_open_order_ask_count",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var MakerBestBidPriceMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_maker_best_bid_price",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var MakerBestAskPriceMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_maker_best_ask_price",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

func UpdateOpenOrderMetrics(strategyType, strategyId, exchangeName, symbol string, submitOrders []types.SubmitOrder) {
	bidOrderCount := 0
	askOrderCount := 0
	bidExposureQuoteAmount := fixedpoint.Zero
	askExposureQuoteAmount := fixedpoint.Zero
	for _, submitOrder := range submitOrders {
		quoteAmount := submitOrder.Quantity.Mul(submitOrder.Price)

		switch submitOrder.Side {
		case types.SideTypeSell:
			askExposureQuoteAmount = askExposureQuoteAmount.Add(quoteAmount)
			askOrderCount++

		case types.SideTypeBuy:
			bidExposureQuoteAmount = bidExposureQuoteAmount.Add(quoteAmount)
			bidOrderCount++

		}
	}

	labels := prometheus.Labels{
		"strategy_type": strategyType,
		"strategy_id":   strategyId,
		"exchange":      exchangeName,
		"symbol":        symbol,
	}

	OpenOrderBidExposureInUsdMetrics.With(labels).Set(bidExposureQuoteAmount.Float64())
	OpenOrderAskExposureInUsdMetrics.With(labels).Set(askExposureQuoteAmount.Float64())
	OpenOrderBidOrderCountMetrics.With(labels).Set(float64(bidOrderCount))
	OpenOrderAskOrderCountMetrics.With(labels).Set(float64(askOrderCount))
}

func init() {
	prometheus.MustRegister(
		OpenOrderAskExposureInUsdMetrics,
		OpenOrderBidExposureInUsdMetrics,
		MakerBestAskPriceMetrics,
		MakerBestBidPriceMetrics,
		OpenOrderAskOrderCountMetrics,
		OpenOrderBidOrderCountMetrics,
	)
}
