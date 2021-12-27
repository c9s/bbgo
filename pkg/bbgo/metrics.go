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

	metricsLockedBalances = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_balances_locked",
			Help: "bbgo exchange locked balances",
		},
		[]string{
			"exchange", // exchange name
			"margin",   // margin of connection. 1 or 0
			"symbol",   // margin symbol of the connection.
			"currency",
		},
	)

	metricsAvailableBalances = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_balances_available",
			Help: "bbgo exchange available balances",
		},
		[]string{
			"exchange", // exchange name
			"margin",   // margin of connection. none, margin or isolated
			"symbol",   // margin symbol of the connection.
			"currency",
		},
	)

	metricsTotalBalances = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_balances_total",
			Help: "bbgo exchange session total balances",
		},
		[]string{
			"exchange", // exchange name
			"margin",   // margin of connection. none, margin or isolated
			"symbol",   // margin symbol of the connection.
			"currency",
		},
	)

	metricsTradesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bbgo_trades_total",
			Help: "bbgo exchange session trades",
		},
		[]string{
			"exchange",  // exchange name
			"margin",    // margin of connection. none, margin or isolated
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
			"margin",    // margin of connection. none, margin or isolated
			"symbol",    // margin symbol of the connection.
			"side",      // side: buy or sell
			"liquidity", // maker or taker
		},
	)

	metricsLastUpdateTimeBalance = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_last_update_time",
			Help: "bbgo last update time of different channel",
		},
		[]string{
			"exchange",  // exchange name
			"margin",    // margin of connection. none, margin or isolated
			"channel",   // channel: user, market
			"data_type", // type: balance, ticker, kline, orderbook, trade, order
			"symbol",    // for market data, trade and order
			"currency",  // for balance
		},
	)
)

func init() {
	prometheus.MustRegister(
		metricsConnectionStatus,
		metricsTotalBalances,
		metricsLockedBalances,
		metricsAvailableBalances,
		metricsTradesTotal,
		metricsTradingVolume,
		metricsLastUpdateTimeBalance,
	)
}
