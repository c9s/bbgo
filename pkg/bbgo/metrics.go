package bbgo

import "github.com/prometheus/client_golang/prometheus"

var (
	metricsConnectionStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_connection_status",
			Help: "bbgo exchange session connection status",
		},
		[]string{
			"session",
			"exchange",    // exchange name
			"channel",     // channel: user or market
			"margin_type", // margin type: none, margin or isolated
			"symbol",      // margin symbol of the connection.
		},
	)

	metricsBalanceLockedMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_balances_locked",
			Help: "bbgo exchange locked balances",
		},
		[]string{
			"session",
			"exchange",    // exchange name
			"margin_type", // margin of connection. 1 or 0
			"symbol",      // margin symbol of the connection.
			"currency",
		},
	)

	metricsBalanceAvailableMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_balances_available",
			Help: "bbgo exchange available balances",
		},
		[]string{
			"session",
			"exchange",    // exchange name
			"margin_type", // margin of connection. none, margin or isolated
			"symbol",      // margin symbol of the connection.
			"currency",
		},
	)

	metricsBalanceDebtMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_balances_debt",
			Help: "bbgo exchange balance debt",
		},
		[]string{
			"session",
			"exchange",    // exchange name
			"margin_type", // margin of connection. none, margin or isolated
			"symbol",      // margin symbol of the connection.
			"currency",
		},
	)

	metricsBalanceBorrowedMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_balances_borrowed",
			Help: "bbgo exchange balance borrowed",
		},
		[]string{
			"session",
			"exchange",    // exchange name
			"margin_type", // margin of connection. none, margin or isolated
			"symbol",      // margin symbol of the connection.
			"currency",
		},
	)

	metricsBalanceInterestMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_balances_interest",
			Help: "bbgo exchange balance interest",
		},
		[]string{
			"session",
			"exchange",    // exchange name
			"margin_type", // margin of connection. none, margin or isolated
			"symbol",      // margin symbol of the connection.
			"currency",
		},
	)

	metricsBalanceNetMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_balances_net",
			Help: "bbgo exchange session total net balances",
		},
		[]string{
			"session",
			"exchange",    // exchange name
			"margin_type", // margin of connection. none, margin or isolated
			"symbol",      // margin symbol of the connection.
			"currency",
		},
	)

	metricsTotalBalances = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_balances_total",
			Help: "bbgo exchange session total balances",
		},
		[]string{
			"session",
			"exchange",    // exchange name
			"margin_type", // margin of connection. none, margin or isolated
			"symbol",      // margin symbol of the connection.
			"currency",
		},
	)

	metricsTradesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bbgo_trades_total",
			Help: "bbgo exchange session trades",
		},
		[]string{
			"session",
			"exchange",    // exchange name
			"margin_type", // margin of connection. none, margin or isolated
			"symbol",      // margin symbol of the connection.
			"side",        // side: buy or sell
			"liquidity",   // maker or taker
		},
	)

	metricsTradingVolume = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_trading_volume",
			Help: "bbgo trading volume",
		},
		[]string{
			"session",
			"exchange",    // exchange name
			"margin_type", // margin of connection. none, margin or isolated
			"symbol",      // margin symbol of the connection.
			"side",        // side: buy or sell
			"liquidity",   // maker or taker
		},
	)

	metricsLastUpdateTimeMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bbgo_last_update_time",
			Help: "bbgo last update time of different channel",
		},
		[]string{
			"session",
			"exchange",    // exchange name
			"margin_type", // margin of connection. none, margin or isolated
			"channel",     // channel: user, market
			"data_type",   // type: balance, ticker, kline, orderbook, trade, order
			"symbol",      // for market data, trade and order
			"currency",    // for balance
		},
	)
)

func init() {
	prometheus.MustRegister(
		metricsConnectionStatus,
		metricsTotalBalances,
		metricsBalanceNetMetrics,
		metricsBalanceLockedMetrics,
		metricsBalanceAvailableMetrics,
		metricsBalanceDebtMetrics,
		metricsBalanceBorrowedMetrics,
		metricsBalanceInterestMetrics,
		metricsTradesTotal,
		metricsTradingVolume,
		metricsLastUpdateTimeMetrics,
	)
}
