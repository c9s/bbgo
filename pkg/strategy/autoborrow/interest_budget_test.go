package autoborrow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/accounting/margininterest"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

var budgetGauges = []string{
	"autoborrow_interest_budget_in_usdt",
	"autoborrow_interest_budget_remaining_in_usdt",
	"autoborrow_interest_budget_used_ratio",
	"autoborrow_interest_budget_exceeded",
	"autoborrow_interest_budget_projected_exceeded",
	"autoborrow_interest_budget_status",
}

// newBudgetWorker is a worker with 1000 USDT borrowed at 0.000001 per hour: at
// testNow 373 hourly charges remain, so 0.373 USDT is projected to the month end
func newBudgetWorker(t *testing.T, budget string, history *fakeHistoryStore) *interestEstimateWorker {
	t.Helper()

	account := newTestAccount(Number(10), debtBalance("USDT", fixedpoint.Zero, Number(1000)))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001"}}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)
	if budget != "" {
		w.config.Budget = &InterestBudgetConfig{MonthlyUSDT: Number(budget)}
	}
	return w
}

func hasSessionSeries(t *testing.T, w *interestEstimateWorker, name string) bool {
	t.Helper()
	for _, series := range gathered(t, name) {
		if series["session"] == w.session.Name {
			return true
		}
	}
	return false
}

// budgetStatusSeries returns the value of each status series of the worker's session
func budgetStatusSeries(t *testing.T, w *interestEstimateWorker) map[string]float64 {
	t.Helper()
	out := map[string]float64{}
	for _, series := range gathered(t, "autoborrow_interest_budget_status") {
		if series["session"] == w.session.Name {
			out[series["status"].(string)] = series["value"].(float64)
		}
	}
	return out
}

func oneHot(status margininterest.BudgetStatus) map[string]float64 {
	out := map[string]float64{}
	for _, s := range margininterest.BudgetStatuses {
		out[string(s)] = 0
	}
	out[string(status)] = 1
	return out
}

// budgetLogs returns the messages logged at level that are about the budget status of a session
func budgetLogs(hook *logrustest.Hook, level logrus.Level) []string {
	var out []string
	for _, e := range hook.AllEntries() {
		if e.Level == level && strings.HasPrefix(e.Message, "interest budget: session ") {
			out = append(out, e.Message)
		}
	}
	return out
}

func TestInterestBudget_WithinTheBudget(t *testing.T) {
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		usdtRow(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), "0.1"),
	}}
	w := newBudgetWorker(t, "10", history)

	est := w.tick(context.Background())

	// 0.1 realized + 1000 x 373 x 0.000001 = 0.373 projected
	require.NotNil(t, est.Budget)
	assert.Equal(t, margininterest.BudgetStatusOK, est.Budget.Status())
	assert.InDelta(t, 0.1, est.Budget.RealizedUSDT.Float64(), 1e-8)
	assert.InDelta(t, 0.473, est.Budget.MonthUSDT.Float64(), 1e-8)
	assert.InDelta(t, est.MonthTotalUSDT.Float64(), est.Budget.MonthUSDT.Float64(), 1e-12)

	assert.Equal(t, 10.0, sessionGauge(t, w, "autoborrow_interest_budget_in_usdt"))
	assert.InDelta(t, 9.527, sessionGauge(t, w, "autoborrow_interest_budget_remaining_in_usdt"), 1e-8)
	// non-dnum Div truncates to 8 decimals
	assert.InDelta(t, 0.0473, sessionGauge(t, w, "autoborrow_interest_budget_used_ratio"), 1e-7)
	assert.Equal(t, 0.0, sessionGauge(t, w, "autoborrow_interest_budget_exceeded"))
	assert.Equal(t, 0.0, sessionGauge(t, w, "autoborrow_interest_budget_projected_exceeded"))
	assert.Equal(t, oneHot(margininterest.BudgetStatusOK), budgetStatusSeries(t, w))
}

func TestInterestBudget_NoBudgetSetsNoGauge(t *testing.T) {
	for name, budget := range map[string]*InterestBudgetConfig{
		"not set": nil,
		"zero":    {MonthlyUSDT: fixedpoint.Zero},
	} {
		t.Run(name, func(t *testing.T) {
			w := newBudgetWorker(t, "", &fakeHistoryStore{})
			w.config.Budget = budget

			est := w.tick(context.Background())

			assert.Nil(t, est.Budget)
			for _, gauge := range budgetGauges {
				assert.False(t, hasSessionSeries(t, w, gauge), gauge)
			}
			// the estimate itself is still published
			assert.True(t, hasSessionSeries(t, w, "autoborrow_interest_month_total_in_usdt"))
		})
	}
}

func TestInterestBudget_ProjectedExceeded(t *testing.T) {
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		usdtRow(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), "0.1"),
	}}
	// 0.1 realized is within 0.4, the month total 0.473 is not
	w := newBudgetWorker(t, "0.4", history)

	est := w.tick(context.Background())

	assert.Equal(t, margininterest.BudgetStatusProjectedExceeded, est.Budget.Status())
	assert.Equal(t, 0.0, sessionGauge(t, w, "autoborrow_interest_budget_exceeded"))
	assert.Equal(t, 1.0, sessionGauge(t, w, "autoborrow_interest_budget_projected_exceeded"))
	assert.InDelta(t, -0.073, sessionGauge(t, w, "autoborrow_interest_budget_remaining_in_usdt"), 1e-8)
	assert.InDelta(t, 1.1825, sessionGauge(t, w, "autoborrow_interest_budget_used_ratio"), 1e-7)
	assert.Equal(t, oneHot(margininterest.BudgetStatusProjectedExceeded), budgetStatusSeries(t, w))
}

func TestInterestBudget_Exceeded(t *testing.T) {
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		usdtRow(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), "0.1"),
	}}
	w := newBudgetWorker(t, "0.05", history)

	est := w.tick(context.Background())

	assert.Equal(t, margininterest.BudgetStatusExceeded, est.Budget.Status())
	assert.Equal(t, 1.0, sessionGauge(t, w, "autoborrow_interest_budget_exceeded"))
	assert.Equal(t, 1.0, sessionGauge(t, w, "autoborrow_interest_budget_projected_exceeded"))
	assert.Equal(t, oneHot(margininterest.BudgetStatusExceeded), budgetStatusSeries(t, w))
}

// An incomplete estimate is a lower bound: under the budget is unknown, never ok
func TestInterestBudget_IncompleteEstimateIsUnknownNotOK(t *testing.T) {
	row := usdtRow(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), "0.1")

	cases := map[string]func(w *interestEstimateWorker, history *fakeHistoryStore){
		"the fee asset sync failed": func(_ *interestEstimateWorker, history *fakeHistoryStore) {
			history.syncErr = map[string]error{"BNB": errors.New("timeout")}
		},
		"an asset sync failed": func(_ *interestEstimateWorker, history *fakeHistoryStore) {
			history.syncErr = map[string]error{"USDT": errors.New("timeout")}
		},
		"the query failed": func(_ *interestEstimateWorker, history *fakeHistoryStore) {
			history.queryErr = errors.New("db down")
		},
		"no rate API": func(w *interestEstimateWorker, _ *fakeHistoryStore) {
			w.interestQueryService = nil
		},
		"a missing rate": func(w *interestEstimateWorker, _ *fakeHistoryStore) {
			w.interestQueryService = &fakeRateService{}
		},
		"an unattributed row": func(_ *interestEstimateWorker, history *fakeHistoryStore) {
			unattributed := bnbPaidRow("", "0", "0.0000016", time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC))
			unattributed.BorrowedInterest = nil
			history.rows = append(history.rows, unattributed)
		},
	}

	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			history := &fakeHistoryStore{rows: []types.MarginInterestValuation{row}}
			w := newBudgetWorker(t, "10", history)
			breakIt(w, history)

			est := w.tick(context.Background())

			assert.False(t, est.Complete)
			assert.False(t, est.Budget.Complete)
			assert.Equal(t, margininterest.BudgetStatusUnknown, est.Budget.Status())
			assert.Equal(t, oneHot(margininterest.BudgetStatusUnknown), budgetStatusSeries(t, w))
			// both exceeded gauges read 0: only the status says it is not ok
			assert.Equal(t, 0.0, sessionGauge(t, w, "autoborrow_interest_budget_exceeded"))
			assert.Equal(t, 0.0, sessionGauge(t, w, "autoborrow_interest_budget_projected_exceeded"))
		})
	}
}

// Over the budget still counts when the estimate is incomplete: the true figure is at least as high
func TestInterestBudget_IncompleteEstimateOverTheBudgetStillCounts(t *testing.T) {
	history := &fakeHistoryStore{
		rows:    []types.MarginInterestValuation{usdtRow(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), "0.1")},
		syncErr: map[string]error{"BNB": errors.New("timeout")},
	}
	w := newBudgetWorker(t, "0.05", history)

	est := w.tick(context.Background())

	assert.False(t, est.Budget.Complete)
	assert.Equal(t, margininterest.BudgetStatusExceeded, est.Budget.Status())
}

// Interest paid in BNB is compared at what was charged, not at the nominal amount
func TestInterestBudget_ComparesTheChargedValueOfBNBPaidInterest(t *testing.T) {
	account := newTestAccount(Number(10))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001"}}
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		// nominal 1 USDT, paid 0.0016 BNB at 600 = 0.96 USDT
		bnbPaidRow("USDT", "1", "0.0016", time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)),
	}}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)
	w.config.Budget = &InterestBudgetConfig{MonthlyUSDT: Number("0.98")}

	est := w.tick(context.Background())

	assert.InDelta(t, 1, est.Realized.Assets["USDT"].Interest.Float64(), 1e-8)
	assert.InDelta(t, 0.96, est.Budget.RealizedUSDT.Float64(), 1e-8)
	assert.Equal(t, margininterest.BudgetStatusOK, est.Budget.Status())
}

// In the current price mode the estimate revalues what was paid, the budget does not
func TestInterestBudget_AlwaysComparesTheChargedPrice(t *testing.T) {
	account := newTestAccount(Number(10))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001"}}
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		// paid 0.0016 BNB at 600 = 0.96 USDT; at the current 900 it is 1.44
		bnbPaidRow("USDT", "1", "0.0016", time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)),
	}}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)
	w.config.RealizedPriceMode = margininterest.PriceModeCurrent
	w.config.Budget = &InterestBudgetConfig{MonthlyUSDT: Number(1)}
	w.pricer = testPrices(map[string]string{"BNB": "900"})

	est := w.tick(context.Background())

	assert.Equal(t, margininterest.PriceModeCurrent, est.Realized.Mode)
	assert.InDelta(t, 1.44, est.Realized.TotalUSDT.Float64(), 1e-8)
	assert.InDelta(t, 0.96, est.Budget.RealizedUSDT.Float64(), 1e-8)
	assert.InDelta(t, 0.96, est.Budget.MonthUSDT.Float64(), 1e-8)
	assert.Equal(t, margininterest.BudgetStatusOK, est.Budget.Status())
	assert.True(t, est.Budget.Complete)
}

// In the current price mode, a paid asset without a current price does not make
// the budget incomplete: the budget only needs the price when charged
func TestInterestBudget_CurrentModeMissingPriceDoesNotAffectTheBudget(t *testing.T) {
	account := newTestAccount(Number(10))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001"}}
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		bnbPaidRow("USDT", "1", "0.0016", time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)),
	}}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)
	w.config.RealizedPriceMode = margininterest.PriceModeCurrent
	w.config.Budget = &InterestBudgetConfig{MonthlyUSDT: Number(1)}
	w.pricer = testPrices(nil)

	est := w.tick(context.Background())

	assert.False(t, est.Complete)
	assert.True(t, est.Budget.Complete)
	assert.Equal(t, margininterest.BudgetStatusOK, est.Budget.Status())
}

// A row that was not priced when charged is left out of the budget, which is then incomplete
func TestInterestBudget_UnpricedRowIsUnknown(t *testing.T) {
	account := newTestAccount(Number(10))
	rates := &fakeRateService{rates: map[string]string{"ETH": "0.000001"}}
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		valuedRow("ETH", "0.001", "", time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)),
	}}
	w := newTestInterestWorker(t, account, rates, history, configured("ETH")...)
	w.config.Budget = &InterestBudgetConfig{MonthlyUSDT: Number(10)}

	est := w.tick(context.Background())

	assert.False(t, est.Budget.Complete)
	assert.Equal(t, margininterest.BudgetStatusUnknown, est.Budget.Status())
}

// The month rolls over by itself, and a status change is logged once, not on every tick
func TestInterestBudget_MonthRolloverAndStatusChangeLog(t *testing.T) {
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		usdtRow(time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), "5"),
	}}
	w := newBudgetWorker(t, "1", history)
	logger, hook := logrustest.NewNullLogger()
	w.logger = logger

	now := time.Date(2026, 11, 30, 23, 59, 0, 0, time.UTC)
	w.now = func() time.Time { return now }

	// the last hour of November: 5 realized, nothing left to project
	est := w.tick(context.Background())
	assert.Equal(t, 0, est.RemainingHours)
	assert.Equal(t, margininterest.BudgetStatusExceeded, est.Budget.Status())
	require.Len(t, budgetLogs(hook, logrus.WarnLevel), 1)
	assert.Contains(t, budgetLogs(hook, logrus.WarnLevel)[0], "is exceeded")

	// the same status again is not logged again
	w.tick(context.Background())
	assert.Len(t, budgetLogs(hook, logrus.WarnLevel), 1)
	assert.Empty(t, budgetLogs(hook, logrus.InfoLevel))

	// December starts: realized is 0 again, 1000 x 743 x 0.000001 = 0.743 is projected
	now = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	est = w.tick(context.Background())
	assert.Equal(t, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), est.Month)
	assert.True(t, est.Budget.RealizedUSDT.IsZero())
	assert.InDelta(t, 0.743, est.Budget.MonthUSDT.Float64(), 1e-8)
	assert.Equal(t, margininterest.BudgetStatusOK, est.Budget.Status())
	assert.Len(t, budgetLogs(hook, logrus.WarnLevel), 1)
	require.Len(t, budgetLogs(hook, logrus.InfoLevel), 1)
	assert.Contains(t, budgetLogs(hook, logrus.InfoLevel)[0], "is ok again")
	assert.Equal(t, oneHot(margininterest.BudgetStatusOK), budgetStatusSeries(t, w))
}

// A budget that starts ok logs no status change
func TestInterestBudget_FirstOKStatusIsNotLoggedAsAChange(t *testing.T) {
	w := newBudgetWorker(t, "10", &fakeHistoryStore{})
	logger, hook := logrustest.NewNullLogger()
	w.logger = logger

	w.tick(context.Background())

	assert.Empty(t, budgetLogs(hook, logrus.WarnLevel))
	assert.Empty(t, budgetLogs(hook, logrus.InfoLevel))

	// the per-tick line is still there
	var lines int
	for _, e := range hook.AllEntries() {
		if strings.HasPrefix(e.Message, "interest budget: session=") {
			lines++
			assert.Contains(t, e.Message, "status=ok")
			assert.Contains(t, e.Message, "budget_usdt=10")
		}
	}
	assert.Equal(t, 1, lines)
}

// Syncing the same rows again does not change the figures
func TestInterestBudget_ResyncDoesNotChangeTheStatus(t *testing.T) {
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		usdtRow(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), "0.1"),
	}}
	w := newBudgetWorker(t, "0.4", history)
	now := testNow
	w.now = func() time.Time { return now }

	first := w.tick(context.Background())

	// one sync interval later, inside the same hour range
	now = now.Add(30 * time.Minute)
	w.lastSync = time.Time{}
	second := w.tick(context.Background())

	assert.Greater(t, len(history.syncs), len(w.syncAssets))
	assert.Equal(t, first.Budget.Status(), second.Budget.Status())
	assert.Equal(t, first.Budget.RealizedUSDT, second.Budget.RealizedUSDT)
	assert.Equal(t, first.Budget.RemainingUSDT, second.Budget.RemainingUSDT)
}

func TestInterestBudgetConfig_Validate(t *testing.T) {
	interval := types.Duration(5 * time.Minute)
	budget := func(v string) *InterestBudgetConfig { return &InterestBudgetConfig{MonthlyUSDT: Number(v)} }

	assert.NoError(t, (&InterestEstimateConfig{Interval: interval, Budget: budget("1000")}).Validate())
	assert.True(t, (&InterestEstimateConfig{Interval: interval, Budget: budget("1000")}).hasBudget())

	// 0 or not set: no budget
	assert.NoError(t, (&InterestEstimateConfig{Interval: interval, Budget: budget("0")}).Validate())
	assert.False(t, (&InterestEstimateConfig{Interval: interval, Budget: budget("0")}).hasBudget())
	assert.False(t, (&InterestEstimateConfig{Interval: interval}).hasBudget())
	assert.NoError(t, (&InterestEstimateConfig{Budget: budget("0")}).Validate())

	assert.Error(t, (&InterestEstimateConfig{Interval: interval, Budget: budget("-1")}).Validate())

	// a budget with the estimate off is an error, not a silent no-op
	err := (&InterestEstimateConfig{Budget: budget("1000")}).Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interestEstimate.interval")
	assert.Error(t, (&Strategy{InterestEstimate: &InterestEstimateConfig{Budget: budget("1000")}}).Validate())
}

func TestInterestBudget_OneBudgetPerSession(t *testing.T) {
	a := newBudgetWorker(t, "10", &fakeHistoryStore{})
	// another autoborrow instance on the same session
	b := newBudgetWorker(t, "20", &fakeHistoryStore{})
	b.session.Name = a.session.Name

	release, err := a.claimBudget()
	require.NoError(t, err)

	_, err = b.claimBudget()
	require.Error(t, err)
	assert.Contains(t, err.Error(), a.session.Name)
	assert.Contains(t, err.Error(), "at most one")

	// another session has its own budget
	other := newBudgetWorker(t, "10", &fakeHistoryStore{})
	releaseOther, err := other.claimBudget()
	require.NoError(t, err)
	releaseOther()

	// the same name on another exchange is another session
	c := newBudgetWorker(t, "10", &fakeHistoryStore{})
	c.session.Name = a.session.Name
	c.session.ExchangeName = types.ExchangeMax
	releaseC, err := c.claimBudget()
	require.NoError(t, err)
	releaseC()

	// once the first instance stops, the session can be budgeted again
	release()
	release, err = b.claimBudget()
	require.NoError(t, err)
	release()
}

// An instance without a budget does not take the budget of the session
func TestInterestBudget_NoBudgetClaimsNothing(t *testing.T) {
	a := newBudgetWorker(t, "", &fakeHistoryStore{})
	b := newBudgetWorker(t, "10", &fakeHistoryStore{})
	b.session.Name = a.session.Name

	releaseA, err := a.claimBudget()
	require.NoError(t, err)
	defer releaseA()

	releaseB, err := b.claimBudget()
	require.NoError(t, err)
	releaseB()
}

// A budget on a session the estimate cannot run on is an error, not a silent no-op
func TestInterestBudget_BudgetOnADisabledEstimateFails(t *testing.T) {
	isolated := newBudgetWorker(t, "10", &fakeHistoryStore{})
	isolated.session.IsolatedMargin = true
	_, err := isolated.claimBudget()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "isolated margin")

	noAssets := newBudgetWorker(t, "10", &fakeHistoryStore{})
	noAssets.assets = nil
	_, err = noAssets.claimBudget()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no autoborrow assets")

	// without a budget the estimate is only disabled, as before
	plain := newBudgetWorker(t, "", &fakeHistoryStore{})
	plain.session.IsolatedMargin = true
	release, err := plain.claimBudget()
	require.NoError(t, err)
	release()
}

var assetBudgetGauges = []string{
	"autoborrow_interest_asset_budget_in_usdt",
	"autoborrow_interest_asset_budget_remaining_in_usdt",
	"autoborrow_interest_asset_budget_used_ratio",
	"autoborrow_interest_asset_budget_exceeded",
	"autoborrow_interest_asset_budget_projected_exceeded",
	"autoborrow_interest_asset_budget_status",
}

// newAssetBudgetWorker is a worker on USDT and ETH. At testNow 373 hourly charges remain:
//
//	USDT: 1000 borrowed at 0.000001 projects 0.373 USDT
//	ETH:  10 borrowed at 0.000001 projects 0.00373 ETH = 11.19 USDT at 3000
func newAssetBudgetWorker(
	t *testing.T, history *fakeHistoryStore, budgets map[string]string,
) *interestEstimateWorker {
	t.Helper()

	account := newTestAccount(Number(10),
		debtBalance("USDT", fixedpoint.Zero, Number(1000)),
		debtBalance("ETH", fixedpoint.Zero, Number(10)),
	)
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001", "ETH": "0.000001"}}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT", "ETH")...)

	w.config.Budget = &InterestBudgetConfig{Assets: map[string]fixedpoint.Value{}}
	for asset, budget := range budgets {
		w.config.Budget.Assets[asset] = Number(budget)
	}
	return w
}

// assetBudgetStatuses returns the status whose series is 1, per asset of the worker's session
func assetBudgetStatuses(t *testing.T, w *interestEstimateWorker) map[string]string {
	t.Helper()
	out := map[string]string{}
	count := map[string]int{}
	for _, series := range gathered(t, "autoborrow_interest_asset_budget_status") {
		if series["session"] != w.session.Name {
			continue
		}

		asset := series["asset"].(string)
		count[asset]++
		if series["value"].(float64) == 1 {
			assert.Empty(t, out[asset], "more than one status is set for %s", asset)
			out[asset] = series["status"].(string)
		}
	}

	// every status has a series, 1 on one of them
	for asset, n := range count {
		assert.Equal(t, len(margininterest.BudgetStatuses), n, asset)
	}
	return out
}

func assetBudgetLogs(hook *logrustest.Hook, level logrus.Level, asset string) []string {
	var out []string
	for _, e := range hook.AllEntries() {
		if e.Level == level && strings.HasPrefix(e.Message, "interest budget: "+asset+" of session ") {
			out = append(out, e.Message)
		}
	}
	return out
}

func TestInterestAssetBudget_EachAssetHasItsOwnStatus(t *testing.T) {
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		usdtRow(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), "0.1"),
		// 0.001 ETH at 3000 = 3 USDT
		valuedRow("ETH", "0.001", "3000", time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)),
	}}
	// USDT: 0.1 + 0.373 = 0.473 of 600; ETH: 3 + 11.19 = 14.19 of 10
	w := newAssetBudgetWorker(t, history, map[string]string{"USDT": "600", "ETH": "10"})

	est := w.tick(context.Background())

	// no session budget is set
	assert.Nil(t, est.Budget)
	for _, gauge := range budgetGauges {
		assert.False(t, hasSessionSeries(t, w, gauge), gauge)
	}

	require.Len(t, est.AssetBudgets, 2)
	usdt, eth := est.AssetBudgets["USDT"], est.AssetBudgets["ETH"]
	assert.Equal(t, margininterest.BudgetStatusOK, usdt.Status())
	assert.InDelta(t, 0.1, usdt.RealizedUSDT.Float64(), 1e-8)
	assert.InDelta(t, 0.473, usdt.MonthUSDT.Float64(), 1e-8)
	assert.Equal(t, margininterest.BudgetStatusProjectedExceeded, eth.Status())
	assert.InDelta(t, 3, eth.RealizedUSDT.Float64(), 1e-8)
	assert.InDelta(t, 14.19, eth.MonthUSDT.Float64(), 1e-6)

	assert.Equal(t, map[string]float64{"USDT": 600, "ETH": 10}, assetSeries(t, w, "autoborrow_interest_asset_budget_in_usdt"))
	remaining := assetSeries(t, w, "autoborrow_interest_asset_budget_remaining_in_usdt")
	assert.InDelta(t, 599.527, remaining["USDT"], 1e-8)
	assert.InDelta(t, -4.19, remaining["ETH"], 1e-6)
	assert.InDelta(t, 1.419, assetSeries(t, w, "autoborrow_interest_asset_budget_used_ratio")["ETH"], 1e-6)
	assert.Equal(t, map[string]float64{"USDT": 0, "ETH": 0}, assetSeries(t, w, "autoborrow_interest_asset_budget_exceeded"))
	assert.Equal(t, map[string]float64{"USDT": 0, "ETH": 1}, assetSeries(t, w, "autoborrow_interest_asset_budget_projected_exceeded"))
	assert.Equal(t, map[string]string{"USDT": "ok", "ETH": "projected_exceeded"}, assetBudgetStatuses(t, w))
}

func TestInterestAssetBudget_Exceeded(t *testing.T) {
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		valuedRow("ETH", "0.001", "3000", time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)),
	}}
	w := newAssetBudgetWorker(t, history, map[string]string{"ETH": "2"})

	est := w.tick(context.Background())

	assert.Equal(t, margininterest.BudgetStatusExceeded, est.AssetBudgets["ETH"].Status())
	assert.Equal(t, map[string]float64{"ETH": 1}, assetSeries(t, w, "autoborrow_interest_asset_budget_exceeded"))
	assert.Equal(t, map[string]string{"ETH": "exceeded"}, assetBudgetStatuses(t, w))
}

// Only the assets that have a budget are compared, and a zero budget is no budget
func TestInterestAssetBudget_OnlyBudgetedAssets(t *testing.T) {
	w := newAssetBudgetWorker(t, &fakeHistoryStore{}, map[string]string{"USDT": "600", "ETH": "0"})

	est := w.tick(context.Background())

	assert.Len(t, est.AssetBudgets, 1)
	assert.Contains(t, est.AssetBudgets, "USDT")
	assert.Equal(t, []string{"USDT"}, keys(assetSeries(t, w, "autoborrow_interest_asset_budget_in_usdt")))
	assert.Equal(t, map[string]string{"USDT": "ok"}, assetBudgetStatuses(t, w))
}

func TestInterestAssetBudget_NoAssetBudgetSetsNoGauge(t *testing.T) {
	w := newBudgetWorker(t, "10", &fakeHistoryStore{})

	est := w.tick(context.Background())

	assert.NotNil(t, est.Budget)
	assert.Empty(t, est.AssetBudgets)
	for _, gauge := range assetBudgetGauges {
		assert.False(t, hasSessionSeries(t, w, gauge), gauge)
	}
}

// The session budget and the asset budgets are independent
func TestInterestAssetBudget_WithASessionBudget(t *testing.T) {
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		valuedRow("ETH", "0.001", "3000", time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)),
	}}
	// the session total 3 + 11.19 + 0.373 = 14.563 is within 100, ETH alone is over 10
	w := newAssetBudgetWorker(t, history, map[string]string{"ETH": "10"})
	w.config.Budget.MonthlyUSDT = Number(100)

	est := w.tick(context.Background())

	assert.Equal(t, margininterest.BudgetStatusOK, est.Budget.Status())
	assert.InDelta(t, 14.563, est.Budget.MonthUSDT.Float64(), 1e-6)
	assert.Equal(t, margininterest.BudgetStatusProjectedExceeded, est.AssetBudgets["ETH"].Status())
	assert.Equal(t, oneHot(margininterest.BudgetStatusOK), budgetStatusSeries(t, w))
	assert.Equal(t, map[string]string{"ETH": "projected_exceeded"}, assetBudgetStatuses(t, w))
}

// What is incomplete for one asset does not make the other asset unknown
func TestInterestAssetBudget_IncompleteIsPerAsset(t *testing.T) {
	cases := map[string]func(w *interestEstimateWorker, history *fakeHistoryStore){
		"its sync failed": func(_ *interestEstimateWorker, history *fakeHistoryStore) {
			history.syncErr = map[string]error{"ETH": errors.New("timeout")}
		},
		"its rate is missing": func(w *interestEstimateWorker, _ *fakeHistoryStore) {
			w.interestQueryService = &fakeRateService{rates: map[string]string{"USDT": "0.000001"}}
		},
		"its current price is missing": func(w *interestEstimateWorker, _ *fakeHistoryStore) {
			w.pricer = testPrices(nil)
		},
		"a charge of it was not priced": func(_ *interestEstimateWorker, history *fakeHistoryStore) {
			history.rows = append(history.rows,
				valuedRow("ETH", "0.0001", "", time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)))
		},
	}

	for name, breakETH := range cases {
		t.Run(name, func(t *testing.T) {
			history := &fakeHistoryStore{}
			w := newAssetBudgetWorker(t, history, map[string]string{"USDT": "600", "ETH": "600"})
			breakETH(w, history)

			est := w.tick(context.Background())

			assert.Equal(t, margininterest.BudgetStatusOK, est.AssetBudgets["USDT"].Status())
			assert.Equal(t, margininterest.BudgetStatusUnknown, est.AssetBudgets["ETH"].Status())
			assert.Equal(t, map[string]string{"USDT": "ok", "ETH": "unknown"}, assetBudgetStatuses(t, w))
		})
	}
}

// What may hide a charge of any asset makes every asset unknown
func TestInterestAssetBudget_IncompleteForEveryAsset(t *testing.T) {
	cases := map[string]func(w *interestEstimateWorker, history *fakeHistoryStore){
		"the fee asset sync failed": func(_ *interestEstimateWorker, history *fakeHistoryStore) {
			history.syncErr = map[string]error{"BNB": errors.New("timeout")}
		},
		"the query failed": func(_ *interestEstimateWorker, history *fakeHistoryStore) {
			history.queryErr = errors.New("db down")
		},
		"no rate API": func(w *interestEstimateWorker, _ *fakeHistoryStore) {
			w.interestQueryService = nil
		},
		"an unattributed row": func(_ *interestEstimateWorker, history *fakeHistoryStore) {
			unattributed := bnbPaidRow("", "0", "0.0000016", time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC))
			unattributed.BorrowedInterest = nil
			history.rows = append(history.rows, unattributed)
		},
	}

	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			history := &fakeHistoryStore{}
			w := newAssetBudgetWorker(t, history, map[string]string{"USDT": "600", "ETH": "600"})
			breakIt(w, history)

			est := w.tick(context.Background())

			assert.Equal(t, margininterest.BudgetStatusUnknown, est.AssetBudgets["USDT"].Status())
			assert.Equal(t, margininterest.BudgetStatusUnknown, est.AssetBudgets["ETH"].Status())
		})
	}
}

// Interest paid in BNB counts toward the budget of the borrowed asset, at what was charged
func TestInterestAssetBudget_BNBPaidInterestCountsTowardTheBorrowedAsset(t *testing.T) {
	account := newTestAccount(Number(10))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001", "ETH": "0.000001"}}
	h := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		// nominal 1 USDT, paid 0.0016 BNB at 600 = 0.96 USDT
		bnbPaidRow("USDT", "1", "0.0016", h),
		// nominal 0.001 ETH, paid 0.004 BNB at 600 = 2.4 USDT
		bnbPaidRow("ETH", "0.001", "0.004", h),
	}}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT", "ETH")...)
	w.config.RealizedPriceMode = margininterest.PriceModeCurrent
	w.config.Budget = &InterestBudgetConfig{Assets: map[string]fixedpoint.Value{
		"USDT": Number("0.98"), "ETH": Number("2.5"),
	}}
	// the current BNB price is higher: the estimate revalues, the budgets do not
	w.pricer = testPrices(map[string]string{"BNB": "900", "ETH": "3000"})

	est := w.tick(context.Background())

	assert.InDelta(t, 0.96, est.AssetBudgets["USDT"].RealizedUSDT.Float64(), 1e-8)
	assert.InDelta(t, 2.4, est.AssetBudgets["ETH"].RealizedUSDT.Float64(), 1e-8)
	assert.Equal(t, margininterest.BudgetStatusOK, est.AssetBudgets["USDT"].Status())
	assert.Equal(t, margininterest.BudgetStatusOK, est.AssetBudgets["ETH"].Status())
	// nothing is budgeted or reported under BNB
	assert.NotContains(t, est.AssetBudgets, "BNB")
}

// A status change of an asset is logged once, under the asset
func TestInterestAssetBudget_StatusChangeLog(t *testing.T) {
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		valuedRow("ETH", "0.001", "3000", time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)),
	}}
	w := newAssetBudgetWorker(t, history, map[string]string{"USDT": "600", "ETH": "2"})
	logger, hook := logrustest.NewNullLogger()
	logger.SetLevel(logrus.DebugLevel)
	w.logger = logger

	now := time.Date(2026, 11, 30, 23, 59, 0, 0, time.UTC)
	w.now = func() time.Time { return now }

	w.tick(context.Background())
	w.tick(context.Background())

	require.Len(t, assetBudgetLogs(hook, logrus.WarnLevel, "ETH"), 1)
	assert.Contains(t, assetBudgetLogs(hook, logrus.WarnLevel, "ETH")[0], "is exceeded")
	assert.Contains(t, assetBudgetLogs(hook, logrus.WarnLevel, "ETH")[0], w.session.Name)
	// USDT started ok: no line
	assert.Empty(t, assetBudgetLogs(hook, logrus.WarnLevel, "USDT"))
	assert.Empty(t, assetBudgetLogs(hook, logrus.InfoLevel, "USDT"))

	// the per-tick line of each budgeted asset is at debug level
	var lines int
	for _, e := range hook.AllEntries() {
		if strings.HasPrefix(e.Message, "interest budget asset: ") {
			assert.Equal(t, logrus.DebugLevel, e.Level)
			lines++
		}
	}
	assert.Equal(t, 4, lines)

	// December: ETH realized is 0 again, 10 x 743 x 0.000001 x 3000 = 22.29 is projected
	now = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	est := w.tick(context.Background())
	assert.Equal(t, margininterest.BudgetStatusProjectedExceeded, est.AssetBudgets["ETH"].Status())
	require.Len(t, assetBudgetLogs(hook, logrus.WarnLevel, "ETH"), 2)
	assert.Contains(t, assetBudgetLogs(hook, logrus.WarnLevel, "ETH")[1], "is projected_exceeded (was exceeded)")
}

func TestInterestAssetBudgetConfig_Validate(t *testing.T) {
	interval := types.Duration(5 * time.Minute)
	assets := func(budgets map[string]string) *InterestBudgetConfig {
		c := &InterestBudgetConfig{Assets: map[string]fixedpoint.Value{}}
		for asset, budget := range budgets {
			c.Assets[asset] = Number(budget)
		}
		return c
	}

	ok := &InterestEstimateConfig{Interval: interval, Budget: assets(map[string]string{"USDT": "600", "ETH": "0"})}
	assert.NoError(t, ok.Validate())
	assert.True(t, ok.hasBudget())
	assert.Equal(t, []string{"USDT"}, sortedKeys(ok.assetBudgets()))
	_, session := ok.sessionBudget()
	assert.False(t, session)

	// only zero budgets: no budget
	none := &InterestEstimateConfig{Budget: assets(map[string]string{"USDT": "0"})}
	assert.NoError(t, none.Validate())
	assert.False(t, none.hasBudget())

	err := (&InterestEstimateConfig{Interval: interval, Budget: assets(map[string]string{"USDT": "-1"})}).Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "USDT")

	// an asset budget with the estimate off is an error, like a session budget
	assert.Error(t, (&InterestEstimateConfig{Budget: assets(map[string]string{"USDT": "600"})}).Validate())

	// a budgeted asset has to be an autoborrow asset: the estimate covers no other
	s := &Strategy{
		Assets:           configured("USDT"),
		InterestEstimate: &InterestEstimateConfig{Interval: interval, Budget: assets(map[string]string{"USDT": "600", "ETH": "10"})},
	}
	err = s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ETH")
	assert.Contains(t, err.Error(), "not in the autoborrow assets")

	s.Assets = configured("USDT", "ETH")
	assert.NoError(t, s.Validate())

	// a zero budget of an asset outside the list is no budget, so no error
	s.Assets = configured("USDT")
	s.InterestEstimate.Budget = assets(map[string]string{"USDT": "600", "ETH": "0"})
	assert.NoError(t, s.Validate())
}

// An instance with only asset budgets also takes the budget of the session
func TestInterestAssetBudget_ClaimsTheSession(t *testing.T) {
	a := newAssetBudgetWorker(t, &fakeHistoryStore{}, map[string]string{"USDT": "600"})
	b := newBudgetWorker(t, "10", &fakeHistoryStore{})
	b.session.Name = a.session.Name

	release, err := a.claimBudget()
	require.NoError(t, err)
	defer release()

	_, err = b.claimBudget()
	require.Error(t, err)
}

// The config reaches the strategy as JSON, converted from the YAML file
func TestInterestBudgetConfig_Unmarshal(t *testing.T) {
	var c InterestEstimateConfig
	require.NoError(t, json.Unmarshal([]byte(`{
		"interval": "5m",
		"budget": {"monthlyUSDT": 1000, "assets": {"USDT": 600, "ETH": "300.5"}}
	}`), &c))

	require.NoError(t, c.Validate())
	session, ok := c.sessionBudget()
	require.True(t, ok)
	assert.InDelta(t, 1000, session.Float64(), 1e-9)

	budgets := c.assetBudgets()
	require.Len(t, budgets, 2)
	assert.InDelta(t, 600, budgets["USDT"].Float64(), 1e-9)
	assert.InDelta(t, 300.5, budgets["ETH"].Float64(), 1e-9)
}
