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
	Help: "nominal margin interest charged since the start of the UTC calendar month, in asset units; interest paid in another asset (e.g. BNB) counts at its nominal amount",
}, assetLabels)

// charged_asset is the asset the interest was paid in: the asset itself, or e.g. BNB
var chargedAssetLabels = []string{"session", "exchange", "asset", "charged_asset"}

var interestRealizedChargedAmountMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_realized_charged_amount",
	Help: "margin interest of the asset actually charged since the start of the UTC calendar month, in charged_asset units",
}, chargedAssetLabels)

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
	Help: "interest estimate errors by kind: account, rate, price, price_query, sync, value, query, unattributed",
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

var interestBudgetInUSDTMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_budget_in_usdt",
	Help: "margin interest budget of the session per UTC calendar month, in USDT; absent without a budget",
}, sessionLabels)

var interestBudgetRemainingInUSDTMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_budget_remaining_in_usdt",
	Help: "budget minus the realized plus projected margin interest of the UTC calendar month, in USDT; negative when over the budget",
}, sessionLabels)

var interestBudgetUsedRatioMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_budget_used_ratio",
	Help: "realized plus projected margin interest of the UTC calendar month divided by the budget",
}, sessionLabels)

var interestBudgetExceededMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_budget_exceeded",
	Help: "1 if the margin interest already charged this UTC calendar month is over the budget, otherwise 0",
}, sessionLabels)

var interestBudgetProjectedExceededMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_budget_projected_exceeded",
	Help: "1 if the realized plus projected margin interest of the UTC calendar month is over the budget, otherwise 0",
}, sessionLabels)

// status is exceeded, projected_exceeded, unknown or ok. 0 on both exceeded
// gauges is not ok by itself: with an incomplete estimate the status is unknown.
var interestBudgetStatusMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_budget_status",
	Help: "1 on the current budget status of the session, 0 on the others: exceeded, projected_exceeded, unknown (within the budget but the estimate is incomplete) or ok",
}, []string{"session", "exchange", "status"})

var interestAssetBudgetInUSDTMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_asset_budget_in_usdt",
	Help: "margin interest budget of the asset per UTC calendar month, in USDT; absent without a budget for the asset",
}, assetLabels)

var interestAssetBudgetRemainingInUSDTMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_asset_budget_remaining_in_usdt",
	Help: "asset budget minus the realized plus projected margin interest of the asset in the UTC calendar month, in USDT; negative when over the budget",
}, assetLabels)

var interestAssetBudgetUsedRatioMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_asset_budget_used_ratio",
	Help: "realized plus projected margin interest of the asset in the UTC calendar month divided by its budget",
}, assetLabels)

var interestAssetBudgetExceededMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_asset_budget_exceeded",
	Help: "1 if the margin interest of the asset already charged this UTC calendar month is over its budget, otherwise 0",
}, assetLabels)

var interestAssetBudgetProjectedExceededMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_asset_budget_projected_exceeded",
	Help: "1 if the realized plus projected margin interest of the asset in the UTC calendar month is over its budget, otherwise 0",
}, assetLabels)

var interestAssetBudgetStatusMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "autoborrow_interest_asset_budget_status",
	Help: "1 on the current budget status of the asset, 0 on the others: exceeded, projected_exceeded, unknown (within the budget but the estimate of the asset is incomplete) or ok",
}, []string{"session", "exchange", "asset", "status"})

// reason is why the borrow was reduced or refused: clamped (a part fits the
// budget), over_budget (nothing is left), unknown (the estimate is incomplete)
// or no_estimate (there is no current estimate)
var interestBudgetBorrowLimitedMetrics = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "autoborrow_interest_budget_borrow_limited_total",
	Help: "number of times the margin interest budgets reduced or refused a borrow, by reason; the borrow loop tries again on every run; only with budget.limitBorrow",
}, []string{"session", "exchange", "asset", "reason"})

func init() {
	prometheus.MustRegister(
		interestProjectedInUSDTMetrics,
		interestProjectedAmountMetrics,
		interestHourlyRateMetrics,
		interestRealizedInUSDTMetrics,
		interestRealizedAmountMetrics,
		interestRealizedChargedAmountMetrics,
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
		interestBudgetInUSDTMetrics,
		interestBudgetRemainingInUSDTMetrics,
		interestBudgetUsedRatioMetrics,
		interestBudgetExceededMetrics,
		interestBudgetProjectedExceededMetrics,
		interestBudgetStatusMetrics,
		interestAssetBudgetInUSDTMetrics,
		interestAssetBudgetRemainingInUSDTMetrics,
		interestAssetBudgetUsedRatioMetrics,
		interestAssetBudgetExceededMetrics,
		interestAssetBudgetProjectedExceededMetrics,
		interestAssetBudgetStatusMetrics,
		interestBudgetBorrowLimitedMetrics,
	)
}
