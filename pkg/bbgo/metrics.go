package bbgo

import "github.com/prometheus/client_golang/prometheus"

var (
	metricsConnectionStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_connection_status",
			Help: "bbgo exchange session connection status",
		},
		[]string{
			"exchange", // exchange name
			"stream",   // user data stream, market data stream
			"margin",   // margin of connection. 1 or 0
			"symbol",   // margin symbol of the connection.
		},
	)

	metricsBalances = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_balances",
			Help: "bbgo exchange session balance",
		},
		[]string{
			"exchange", // exchange name
			"status",   // 1 -> ON, 0 -> OFF
			"currency",
			"margin", // margin of connection. 1 or 0
			"symbol", // margin symbol of the connection.
		},
	)

	metricsTradesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bbgo_trades_total",
			Help: "bbgo exchange session trades",
		},
		[]string{
			"exchange",  // exchange name
			"margin",    // margin of connection. 1 or 0
			"isolated",  // isolated or not
			"symbol",    // margin symbol of the connection.
			"side",      // side: buy or sell
			"liquidity", // maker or taker
		},
	)

	metricsTradingVolume = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_trading_volume",
			Help: "bbgo trading volume",
		},
		[]string{
			"exchange",  // exchange name
			"margin",    // margin of connection. 1 or 0
			"isolated",  // isolated or not
			"symbol",    // margin symbol of the connection.
			"side",      // side: buy or sell
			"liquidity", // maker or taker
		},
	)
)

func init() {
	prometheus.MustRegister(
		metricsConnectionStatus,
		metricsBalances,
		metricsTradesTotal,
		metricsTradingVolume,
	)
}
