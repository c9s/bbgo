package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var makerOpenOrderBidExposureInUsdMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_maker_open_order_bid_exposure_in_usd",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var makerOpenOrderAskExposureInUsdMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_maker_open_order_ask_exposure_in_usd",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var makerOpenOrderBidOrderCountMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_maker_open_order_bid_count",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var makerOpenOrderAskOrderCountMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_maker_open_order_ask_count",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var makerBestBidPriceMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_maker_best_bid_price",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

var makerBestAskPriceMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_maker_best_ask_price",
		Help: "",
	}, []string{"strategy_type", "strategy_id", "exchange", "symbol"})

// UpdateMakerOpenOrderMetrics updates the metrics for maker open orders.
// It assumes that the orders have been partially or all canceled.
// In the case of a partial cancel, the submitOrders should be the active orders.
func UpdateMakerOpenOrderMetrics(strategyType, strategyId, exchangeName, symbol string, submitOrders []types.SubmitOrder) {
	labels := prometheus.Labels{
		"strategy_type": strategyType,
		"strategy_id":   strategyId,
		"exchange":      exchangeName,
		"symbol":        symbol,
	}
	if len(submitOrders) == 0 {
		makerOpenOrderBidExposureInUsdMetrics.With(labels).Set(0.0)
		makerOpenOrderAskExposureInUsdMetrics.With(labels).Set(0.0)
		makerOpenOrderBidOrderCountMetrics.With(labels).Set(0.0)
		makerOpenOrderAskOrderCountMetrics.With(labels).Set(0.0)
		makerBestBidPriceMetrics.With(labels).Set(0.0)
		makerBestAskPriceMetrics.With(labels).Set(0.0)
		return
	}
	bidOrderCount := 0
	askOrderCount := 0
	bidExposureQuoteAmount := fixedpoint.Zero
	askExposureQuoteAmount := fixedpoint.Zero

	bestAskPrice := fixedpoint.Zero
	bestBidPrice := fixedpoint.Zero

	for _, submitOrder := range submitOrders {
		quoteAmount := submitOrder.Quantity.Mul(submitOrder.Price)

		switch submitOrder.Side {
		case types.SideTypeSell:
			askExposureQuoteAmount = askExposureQuoteAmount.Add(quoteAmount)
			askOrderCount++
			if bestAskPrice.IsZero() {
				bestAskPrice = submitOrder.Price
			} else {
				bestAskPrice = fixedpoint.Min(bestAskPrice, submitOrder.Price)
			}

		case types.SideTypeBuy:
			bidExposureQuoteAmount = bidExposureQuoteAmount.Add(quoteAmount)
			bidOrderCount++

			if bestBidPrice.IsZero() {
				bestBidPrice = submitOrder.Price
			} else {
				bestBidPrice = fixedpoint.Max(bestBidPrice, submitOrder.Price)
			}

		}
	}

	makerOpenOrderBidExposureInUsdMetrics.With(labels).Set(bidExposureQuoteAmount.Float64())
	makerOpenOrderAskExposureInUsdMetrics.With(labels).Set(askExposureQuoteAmount.Float64())
	makerOpenOrderBidOrderCountMetrics.With(labels).Set(float64(bidOrderCount))
	makerOpenOrderAskOrderCountMetrics.With(labels).Set(float64(askOrderCount))
	makerBestBidPriceMetrics.With(labels).Set(bestBidPrice.Float64())
	makerBestAskPriceMetrics.With(labels).Set(bestAskPrice.Float64())
}

func init() {
	prometheus.MustRegister(
		makerOpenOrderAskExposureInUsdMetrics,
		makerOpenOrderBidExposureInUsdMetrics,
		makerBestAskPriceMetrics,
		makerBestBidPriceMetrics,
		makerOpenOrderAskOrderCountMetrics,
		makerOpenOrderBidOrderCountMetrics,
	)
}
