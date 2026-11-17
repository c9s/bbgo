package autoborrow

import (
	"context"
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
