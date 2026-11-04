package autoborrow

import "github.com/prometheus/client_golang/prometheus"

var sessionLabels = []string{"session", "exchange"}
var assetLabels = []string{"session", "exchange", "asset"}

// hours is the projection window in hourly charges, e.g. 720 for one rolling month of 30 days
var rollingLabels = []string{"session", "exchange", "hours"}
var rollingAssetLabels = []string{"session", "exchange", "asset", "hours"}

var interestProjectedInUSDTMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_projected_in_usdt",
	Help: "projected margin interest of the current principal until the end of the UTC calendar month, in USDT",
}, assetLabels)

var interestProjectedAmountMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_projected_amount",
	Help: "projected margin interest of the current principal until the end of the UTC calendar month, in asset units",
}, assetLabels)

var interestHourlyRateMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_hourly_rate",
	Help: "next hourly margin interest rate (simple, not annualized)",
}, assetLabels)

var interestRealizedInUSDTMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_realized_in_usdt",
	Help: "margin interest charged since the start of the UTC calendar month, in USDT",
}, assetLabels)

var interestRealizedAmountMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_realized_amount",
	Help: "margin interest charged since the start of the UTC calendar month, in asset units",
}, assetLabels)

var interestMonthAmountMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_month_amount",
	Help: "realized plus projected margin interest of the asset in the UTC calendar month, in asset units",
}, assetLabels)

var interestMonthInUSDTMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_month_in_usdt",
	Help: "realized plus projected margin interest of the asset in the UTC calendar month, in USDT; absent when a part has no price",
}, assetLabels)

var interestAssetEstimateCompleteMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_asset_estimate_complete",
	Help: "1 if the month interest of the asset had its rate, price and history, otherwise 0",
}, assetLabels)

var interestProjectedTotalInUSDTMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_projected_total_in_usdt",
	Help: "projected margin interest of all assets until the end of the UTC calendar month, in USDT",
}, sessionLabels)

var interestRealizedTotalInUSDTMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_realized_total_in_usdt",
	Help: "margin interest of all assets charged since the start of the UTC calendar month, in USDT",
}, sessionLabels)

var interestMonthTotalInUSDTMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_month_total_in_usdt",
	Help: "realized plus projected margin interest of the UTC calendar month, in USDT",
}, sessionLabels)

var interestRemainingHoursMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_remaining_hours",
	Help: "hourly interest charges left in the UTC calendar month",
}, sessionLabels)

var interestEstimateCompleteMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_estimate_complete",
	Help: "1 if the last interest estimate had every rate, price and history, otherwise 0",
}, sessionLabels)

var interestEstimateErrorsMetrics = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "autoborrow_interest_estimate_errors_total",
	Help: "interest estimate errors by kind: account, rate, price, sync, query",
}, []string{"session", "exchange", "kind"})

var interestRollingProjectedAmountMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_rolling_projected_amount",
	Help: "projected margin interest of the current principal over the next `hours` hourly charges, in asset units",
}, rollingAssetLabels)

var interestRollingProjectedInUSDTMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_rolling_projected_in_usdt",
	Help: "projected margin interest of the current principal over the next `hours` hourly charges, in USDT; absent when the asset has no price",
}, rollingAssetLabels)

var interestRollingProjectedTotalInUSDTMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_rolling_projected_total_in_usdt",
	Help: "projected margin interest of all assets over the next `hours` hourly charges, in USDT",
}, rollingLabels)

var interestRollingProjectionCompleteMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_rolling_projection_complete",
	Help: "1 if every debt asset had a rate and a price for the projection over the next `hours` hourly charges, otherwise 0",
}, rollingLabels)

func init() {
	prometheus.MustRegister(
		interestProjectedInUSDTMetrics,
		interestProjectedAmountMetrics,
		interestHourlyRateMetrics,
		interestRealizedInUSDTMetrics,
		interestRealizedAmountMetrics,
		interestMonthAmountMetrics,
		interestMonthInUSDTMetrics,
		interestAssetEstimateCompleteMetrics,
		interestProjectedTotalInUSDTMetrics,
		interestRealizedTotalInUSDTMetrics,
		interestMonthTotalInUSDTMetrics,
		interestRemainingHoursMetrics,
		interestEstimateCompleteMetrics,
		interestEstimateErrorsMetrics,
		interestRollingProjectedAmountMetrics,
		interestRollingProjectedInUSDTMetrics,
		interestRollingProjectedTotalInUSDTMetrics,
		interestRollingProjectionCompleteMetrics,
	)
}
