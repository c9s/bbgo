package metrics

import "github.com/prometheus/client_golang/prometheus"

var AccountMarginLevelMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_account_margin_level",
		Help: "account margin level metrics",
	}, []string{"exchange", "account_type", "isolated_symbol"})

var AccountMarginRatioMetrics = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "bbgo_account_margin_ratio",
		Help: "account margin ratio metrics",
	}, []string{"exchange", "account_type", "isolated_symbol"})

func init() {
	prometheus.MustRegister(AccountMarginLevelMetrics, AccountMarginRatioMetrics)
}
