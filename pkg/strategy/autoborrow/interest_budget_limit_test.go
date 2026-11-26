package autoborrow

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/accounting/margininterest"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

// limitTol is the tolerance of a limit computed by hand in a test comment
const limitTol = 1e-4

// newTestLimiter is a limiter at testNow with ETH at 3000 and BTC at 60000
func newTestLimiter(budget *InterestBudgetConfig) *interestBudgetLimiter {
	l := newInterestBudgetLimiter(
		&InterestEstimateConfig{Interval: types.Duration(5 * time.Minute), Budget: budget},
		testPrices(map[string]string{"ETH": "3000", "BTC": "60000"}),
	)
	l.now = func() time.Time { return testNow }
	return l
}

func sessionBudgetOf(usdt string) *InterestBudgetConfig {
	return &InterestBudgetConfig{MonthlyUSDT: Number(usdt), LimitBorrow: true}
}

func assetBudgetsOf(budgets map[string]string) *InterestBudgetConfig {
	c := &InterestBudgetConfig{Assets: map[string]fixedpoint.Value{}, LimitBorrow: true}
	for asset, budget := range budgets {
		c.Assets[asset] = Number(budget)
	}
	return c
}

func testRates(assets ...string) types.MarginNextHourlyInterestRateMap {
	rates := types.MarginNextHourlyInterestRateMap{}
	for _, asset := range assets {
		rates[asset] = &types.MarginNextHourlyInterestRate{Asset: asset, HourlyRate: Number("0.000001")}
	}
	return rates
}

// testBasis is the basis of a tick at testNow: 373 hourly charges remain, every
// rate is 0.000001 per hour, and the session was charged sessionCharged.
//
// One ETH borrowed now then costs 374 x 0.000001 x 3000 = 1.122 USDT to the
// month end, and one ETH of debt is charged 373 x 0.000001 x 3000 = 1.119 USDT more.
func testBasis(sessionCharged string) *interestBudgetBasis {
	return &interestBudgetBasis{
		Month:      margininterest.MonthStart(testNow),
		Hours:      373,
		Rates:      testRates("ETH", "BTC", "USDT"),
		StalePrice: map[string]struct{}{},
		Session:    &chargedInterest{USDT: Number(sessionCharged), Complete: true},
		Assets:     map[string]chargedInterest{},
	}
}

func debtsOf(balances ...types.Balance) types.BalanceMap {
	m := types.BalanceMap{}
	for _, b := range balances {
		m[b.Currency] = b
	}
	return m
}

func ethDebt(borrowed string) types.BalanceMap {
	return debtsOf(debtBalance("ETH", fixedpoint.Zero, Number(borrowed)))
}

func TestInterestBudgetLimiter_FitsTheBudget(t *testing.T) {
	l := newTestLimiter(sessionBudgetOf("100"))
	l.set(testBasis("10"))

	// 10 ETH of debt projects 11.19 USDT to the month end, so 78.81 USDT are
	// left. 5 more ETH cost 5.61.
	allowed, limit := l.limit("ETH", Number(5), ethDebt("10"))

	assert.Equal(t, Number(5), allowed)
	assert.False(t, limit.limited())
}

func TestInterestBudgetLimiter_ClampsToWhatIsLeftOfTheMonth(t *testing.T) {
	l := newTestLimiter(sessionBudgetOf("100"))
	l.set(testBasis("80"))

	// 20 USDT are left of the month: 20 / 1.122 = 17.8253 ETH
	allowed, limit := l.limit("ETH", Number(100), types.BalanceMap{})

	assert.InDelta(t, 17.8253, allowed.Float64(), limitTol)
	assert.Equal(t, borrowLimit{Reason: borrowLimitClamped, Budget: sessionBudgetName}, limit)
}

// What was not spent earlier in the month is there to borrow with later
func TestInterestBudgetLimiter_AMonthChargedLessLeavesMoreToBorrow(t *testing.T) {
	l := newTestLimiter(sessionBudgetOf("100"))
	l.set(testBasis("10"))

	// 90 USDT are left of the month: 90 / 1.122 = 80.2139 ETH, where a month that
	// was charged 80 leaves 17.8253
	allowed, limit := l.limit("ETH", Number(100), types.BalanceMap{})

	assert.InDelta(t, 80.2139, allowed.Float64(), limitTol)
	assert.Equal(t, borrowLimit{Reason: borrowLimitClamped, Budget: sessionBudgetName}, limit)
}

func TestInterestBudgetLimiter_CountsTheDebtsToTheMonthEnd(t *testing.T) {
	l := newTestLimiter(sessionBudgetOf("100"))
	l.set(testBasis("10"))

	// 10 ETH of debt will be charged 11.19 USDT more: 78.81 / 1.122 = 70.2406 ETH
	allowed, limit := l.limit("ETH", Number(100), ethDebt("10"))

	assert.InDelta(t, 70.2406, allowed.Float64(), limitTol)
	assert.Equal(t, borrowLimit{Reason: borrowLimitClamped, Budget: sessionBudgetName}, limit)
}

// The limit is about this month only. In its last hour a borrow pays one charge
// of it, so what is left of the budget buys a large debt. The next month starts
// with that debt, and nothing here repays it.
func TestInterestBudgetLimiter_TheLastHourOfTheMonth(t *testing.T) {
	l := newTestLimiter(sessionBudgetOf("100"))
	basis := testBasis("0")
	basis.Hours = 0
	l.set(basis)

	// one ETH costs one charge, 0.003 USDT: 1000 ETH cost 3 USDT
	allowed, limit := l.limit("ETH", Number(1000), types.BalanceMap{})
	assert.Equal(t, Number(1000), allowed)
	assert.False(t, limit.limited())

	// 100 / 0.003 = 33333.33 ETH
	allowed, limit = l.limit("ETH", Number(100000), types.BalanceMap{})
	assert.InDelta(t, 33333.33, allowed.Float64(), 0.01)
	assert.Equal(t, borrowLimitClamped, limit.Reason)
}

// A borrow is charged once at the moment it is made. The interest history
// shows that charge only after a sync, so until then the limiter counts it:
// otherwise every borrow of the last hour would see the same budget left.
func TestInterestBudgetLimiter_CountsTheChargeOfABorrowUntilItIsSynced(t *testing.T) {
	l := newTestLimiter(sessionBudgetOf("6"))
	lastHour := func(charged string, syncedAt time.Time) *interestBudgetBasis {
		basis := testBasis(charged)
		basis.Hours = 0
		basis.SyncedAt = syncedAt
		return basis
	}
	l.set(lastHour("0", testNow))

	// one ETH costs one charge, 0.003 USDT: 1000 ETH cost 3 of the 6 USDT
	allowed, limit := l.limit("ETH", Number(1000), types.BalanceMap{})
	require.Equal(t, Number(1000), allowed)
	require.False(t, limit.limited())
	l.borrowed("ETH", allowed)

	// the debt pays nothing more this month and the history still says 0 USDT
	// were charged, but 3 USDT are left, not 6
	debts := ethDebt("1000")
	allowed, limit = l.limit("ETH", Number(2000), debts)
	assert.InDelta(t, 1000, allowed.Float64(), limitTol)
	assert.Equal(t, borrowLimit{Reason: borrowLimitClamped, Budget: sessionBudgetName}, limit)

	// a tick without a new sync changes nothing
	l.set(lastHour("0", testNow))
	allowed, _ = l.limit("ETH", Number(2000), debts)
	assert.InDelta(t, 1000, allowed.Float64(), limitTol)

	// a sync that started less than syncedOverlap after the borrow may or may not
	// hold the charge: it is still counted, which is the safe side when it does
	l.now = func() time.Time { return testNow.Add(30 * time.Minute) }
	l.set(lastHour("3", testNow.Add(30*time.Minute)))
	allowed, limit = l.limit("ETH", Number(2000), debts)
	assert.True(t, allowed.IsZero())
	assert.Equal(t, borrowLimitOverBudget, limit.Reason)

	// a sync that started syncedOverlap after the borrow holds it
	l.now = func() time.Time { return testNow.Add(time.Hour) }
	l.set(lastHour("3", testNow.Add(time.Hour)))
	allowed, limit = l.limit("ETH", Number(2000), debts)
	assert.InDelta(t, 1000, allowed.Float64(), limitTol)
	assert.Equal(t, borrowLimitClamped, limit.Reason)
}

func TestInterestBudgetLimiter_TheChargeOfABorrowCountsAgainstItsBudgets(t *testing.T) {
	budget := assetBudgetsOf(map[string]string{"ETH": "6", "BTC": "6"})
	l := newTestLimiter(budget)
	basis := testBasis("0")
	basis.Session = nil
	basis.Hours = 0
	basis.Assets["ETH"] = chargedInterest{USDT: fixedpoint.Zero, Complete: true}
	basis.Assets["BTC"] = chargedInterest{USDT: fixedpoint.Zero, Complete: true}
	l.set(basis)

	allowed, _ := l.limit("ETH", Number(1000), types.BalanceMap{})
	require.Equal(t, Number(1000), allowed)
	l.borrowed("ETH", allowed)

	// 3 USDT are left of the ETH budget. The BTC budget is untouched: one BTC
	// costs one charge, 0.06 USDT, so 100 BTC fit
	allowed, _ = l.limit("ETH", Number(2000), types.BalanceMap{})
	assert.InDelta(t, 1000, allowed.Float64(), limitTol)

	allowed, limit := l.limit("BTC", Number(100), types.BalanceMap{})
	assert.Equal(t, Number(100), allowed)
	assert.False(t, limit.limited())

	// a new month does not count the charges of the last one
	december := time.Date(2026, 12, 1, 0, 10, 0, 0, time.UTC)
	l.now = func() time.Time { return december }
	next := testBasis("0")
	next.Session = nil
	next.Month = margininterest.MonthStart(december)
	next.Hours = 0
	next.Assets["ETH"] = chargedInterest{USDT: fixedpoint.Zero, Complete: true}
	l.set(next)

	allowed, limit = l.limit("ETH", Number(2000), types.BalanceMap{})
	assert.Equal(t, Number(2000), allowed)
	assert.False(t, limit.limited())
}

// A borrow of an asset no budget applies to is not counted against any
func TestInterestBudgetLimiter_ABorrowWithoutABudgetIsNotCounted(t *testing.T) {
	l := newTestLimiter(assetBudgetsOf(map[string]string{"ETH": "6"}))

	l.borrowed("BTC", Number(1))
	l.borrowed("ETH", fixedpoint.Zero)

	assert.Empty(t, l.charges)
}

// The borrow loop asks on every run: a limit that stays is reported once
func TestInterestBudgetLimiter_ReportsAChangeOnce(t *testing.T) {
	l := newTestLimiter(sessionBudgetOf("100"))
	unknown := borrowLimit{Reason: borrowLimitUnknown, Budget: sessionBudgetName}
	clamped := borrowLimit{Reason: borrowLimitClamped, Budget: sessionBudgetName}

	// not limited from the start is nothing to report
	assert.False(t, l.changed("ETH", borrowLimit{}))

	assert.True(t, l.changed("ETH", unknown))
	assert.False(t, l.changed("ETH", unknown))
	// each asset on its own
	assert.True(t, l.changed("BTC", unknown))

	assert.True(t, l.changed("ETH", clamped))
	assert.True(t, l.changed("ETH", borrowLimit{}))
	assert.False(t, l.changed("ETH", borrowLimit{}))
}

func TestInterestBudgetLimiter_OverTheBudgetAllowsNothing(t *testing.T) {
	t.Run("charged more than the budget", func(t *testing.T) {
		l := newTestLimiter(sessionBudgetOf("100"))
		l.set(testBasis("101"))

		allowed, limit := l.limit("ETH", Number(1), types.BalanceMap{})
		assert.True(t, allowed.IsZero())
		assert.Equal(t, borrowLimit{Reason: borrowLimitOverBudget, Budget: sessionBudgetName}, limit)
	})

	t.Run("the debt is projected over the budget", func(t *testing.T) {
		l := newTestLimiter(sessionBudgetOf("100"))
		l.set(testBasis("10"))

		// 100 ETH of debt project 111.9 USDT to the month end
		allowed, limit := l.limit("ETH", Number(1), ethDebt("100"))
		assert.True(t, allowed.IsZero())
		assert.Equal(t, borrowLimitOverBudget, limit.Reason)
	})

	t.Run("exactly at the budget", func(t *testing.T) {
		l := newTestLimiter(sessionBudgetOf("100"))
		l.set(testBasis("100"))

		allowed, limit := l.limit("ETH", Number(1), types.BalanceMap{})
		assert.True(t, allowed.IsZero())
		assert.Equal(t, borrowLimitOverBudget, limit.Reason)
	})

	// the figures are lower bounds: over the budget is certain also when a part is missing
	t.Run("incomplete and over", func(t *testing.T) {
		l := newTestLimiter(sessionBudgetOf("100"))
		basis := testBasis("101")
		basis.Session.Complete = false
		l.set(basis)

		_, limit := l.limit("ETH", Number(1), types.BalanceMap{})
		assert.Equal(t, borrowLimitOverBudget, limit.Reason)
	})
}

// What is left of a budget is not known when a part of its figures may be
// missing: within the budget is then not a reason to borrow
func TestInterestBudgetLimiter_UnknownAllowsNothing(t *testing.T) {
	cases := []struct {
		name  string
		asset string
		debts types.BalanceMap
		setup func(basis *interestBudgetBasis)
	}{
		{
			name: "a charge may be missing", asset: "ETH", debts: types.BalanceMap{},
			setup: func(basis *interestBudgetBasis) { basis.Session.Complete = false },
		},
		{
			name: "the asset has no rate", asset: "ETH", debts: types.BalanceMap{},
			setup: func(basis *interestBudgetBasis) { delete(basis.Rates, "ETH") },
		},
		{
			name: "the asset has no price", asset: "XYZ", debts: types.BalanceMap{},
			setup: func(basis *interestBudgetBasis) { basis.Rates = testRates("XYZ") },
		},
		{
			name: "the price of the asset is stale", asset: "ETH", debts: types.BalanceMap{},
			setup: func(basis *interestBudgetBasis) { basis.StalePrice["ETH"] = struct{}{} },
		},
		{
			// a debt opened after the tick, in an asset the tick did not query
			name: "a debt of the session has no rate", asset: "ETH",
			debts: debtsOf(debtBalance("BTC", fixedpoint.Zero, Number("0.1"))),
			setup: func(basis *interestBudgetBasis) { delete(basis.Rates, "BTC") },
		},
		{
			name: "the price of a debt of the session is stale", asset: "ETH",
			debts: debtsOf(debtBalance("BTC", fixedpoint.Zero, Number("0.1"))),
			setup: func(basis *interestBudgetBasis) { basis.StalePrice["BTC"] = struct{}{} },
		},
		{
			name: "the tick has no figures for the budget", asset: "ETH", debts: types.BalanceMap{},
			setup: func(basis *interestBudgetBasis) { basis.Session = nil },
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := newTestLimiter(sessionBudgetOf("100"))
			basis := testBasis("10")
			c.setup(basis)
			l.set(basis)

			allowed, limit := l.limit(c.asset, Number(1), c.debts)

			assert.True(t, allowed.IsZero())
			assert.Equal(t, borrowLimit{Reason: borrowLimitUnknown, Budget: sessionBudgetName}, limit)
		})
	}
}

func TestInterestBudgetLimiter_NoCurrentEstimateAllowsNothing(t *testing.T) {
	t.Run("no tick yet", func(t *testing.T) {
		l := newTestLimiter(sessionBudgetOf("100"))

		allowed, limit := l.limit("ETH", Number(1), types.BalanceMap{})
		assert.True(t, allowed.IsZero())
		assert.Equal(t, borrowLimit{Reason: borrowLimitNoEstimate}, limit)
	})

	t.Run("the last tick is too old", func(t *testing.T) {
		l := newTestLimiter(sessionBudgetOf("100"))
		l.set(testBasis("10"))

		// the estimate interval is 5m: the figures are used for 15m
		l.now = func() time.Time { return testNow.Add(15 * time.Minute) }
		allowed, limit := l.limit("ETH", Number(1), types.BalanceMap{})
		assert.Equal(t, Number(1), allowed)
		assert.False(t, limit.limited())

		l.now = func() time.Time { return testNow.Add(15*time.Minute + time.Second) }
		allowed, limit = l.limit("ETH", Number(1), types.BalanceMap{})
		assert.True(t, allowed.IsZero())
		assert.Equal(t, borrowLimitNoEstimate, limit.Reason)
	})

	t.Run("the last tick is of another month", func(t *testing.T) {
		l := newTestLimiter(sessionBudgetOf("100"))
		lastMinute := time.Date(2026, 11, 30, 23, 59, 0, 0, time.UTC)
		l.now = func() time.Time { return lastMinute }
		basis := testBasis("10")
		basis.Hours = 0
		l.set(basis)

		l.now = func() time.Time { return lastMinute.Add(2 * time.Minute) }
		allowed, limit := l.limit("ETH", Number(1), types.BalanceMap{})
		assert.True(t, allowed.IsZero())
		assert.Equal(t, borrowLimitNoEstimate, limit.Reason)
	})
}

func TestInterestBudgetLimiter_AssetWithoutABudgetIsNotLimited(t *testing.T) {
	l := newTestLimiter(assetBudgetsOf(map[string]string{"ETH": "10"}))

	// no tick yet: that only matters for an asset a budget applies to
	allowed, limit := l.limit("BTC", Number(3), types.BalanceMap{})
	assert.Equal(t, Number(3), allowed)
	assert.False(t, limit.limited())

	_, limit = l.limit("ETH", Number(3), types.BalanceMap{})
	assert.Equal(t, borrowLimitNoEstimate, limit.Reason)
}

func TestInterestBudgetLimiter_AssetBudget(t *testing.T) {
	l := newTestLimiter(assetBudgetsOf(map[string]string{"ETH": "50"}))
	basis := testBasis("0")
	basis.Session = nil
	basis.Assets["ETH"] = chargedInterest{USDT: Number(5), Complete: true}
	l.set(basis)

	// the BTC debt does not count against the ETH budget: 45 USDT are left of the
	// month, 45 / 1.122 = 40.1070 ETH
	debts := debtsOf(debtBalance("BTC", fixedpoint.Zero, Number(100)))
	allowed, limit := l.limit("ETH", Number(100), debts)

	assert.InDelta(t, 40.1070, allowed.Float64(), limitTol)
	assert.Equal(t, borrowLimit{Reason: borrowLimitClamped, Budget: "ETH"}, limit)

	// an asset is unknown on its own: a missing charge of ETH does not stop BTC
	basis = testBasis("0")
	basis.Session = nil
	basis.Assets["ETH"] = chargedInterest{USDT: Number(5), Complete: false}
	l.set(basis)

	_, limit = l.limit("ETH", Number(1), types.BalanceMap{})
	assert.Equal(t, borrowLimit{Reason: borrowLimitUnknown, Budget: "ETH"}, limit)

	allowed, limit = l.limit("BTC", Number(1), types.BalanceMap{})
	assert.Equal(t, Number(1), allowed)
	assert.False(t, limit.limited())
}

// The budget of the asset and the budget of the session both apply
func TestInterestBudgetLimiter_TheSmallerLimitWins(t *testing.T) {
	budget := assetBudgetsOf(map[string]string{"ETH": "50"})
	budget.MonthlyUSDT = Number(100)

	t.Run("the asset budget is the smaller", func(t *testing.T) {
		l := newTestLimiter(budget)
		basis := testBasis("0")
		basis.Assets["ETH"] = chargedInterest{USDT: fixedpoint.Zero, Complete: true}
		l.set(basis)

		// 50 / 1.122 = 44.5633 ETH, which costs 50 of the session's 100 USDT
		allowed, limit := l.limit("ETH", Number(100), types.BalanceMap{})
		assert.InDelta(t, 44.5633, allowed.Float64(), limitTol)
		assert.Equal(t, "ETH", limit.Budget)
	})

	t.Run("the asset budget is the smaller with another debt on the session", func(t *testing.T) {
		l := newTestLimiter(budget)
		basis := testBasis("0")
		basis.Assets["ETH"] = chargedInterest{USDT: fixedpoint.Zero, Complete: true}
		l.set(basis)

		// 1 BTC of debt projects 22.38 USDT to the month end, so 77.62 are left of
		// the session budget: 69.18 ETH. The ETH budget alone allows 44.5633.
		debts := debtsOf(debtBalance("BTC", fixedpoint.Zero, Number(1)))
		allowed, limit := l.limit("ETH", Number(100), debts)
		assert.InDelta(t, 44.5633, allowed.Float64(), limitTol)
		assert.Equal(t, "ETH", limit.Budget)
	})

	t.Run("the session budget is the smaller", func(t *testing.T) {
		l := newTestLimiter(budget)
		basis := testBasis("0")
		basis.Assets["ETH"] = chargedInterest{USDT: fixedpoint.Zero, Complete: true}
		l.set(basis)

		// 3 BTC project 67.14 USDT: 32.86 are left, 32.86 / 1.122 = 29.2870 ETH
		debts := debtsOf(debtBalance("BTC", fixedpoint.Zero, Number(3)))
		allowed, limit := l.limit("ETH", Number(100), debts)
		assert.InDelta(t, 29.2870, allowed.Float64(), limitTol)
		assert.Equal(t, borrowLimit{Reason: borrowLimitClamped, Budget: sessionBudgetName}, limit)
	})

	t.Run("an asset without its own budget is under the session budget", func(t *testing.T) {
		l := newTestLimiter(budget)
		l.set(testBasis("0"))

		// one BTC costs 374 x 0.000001 x 60000 = 22.44 USDT: 100 / 22.44 = 4.4563 BTC
		allowed, limit := l.limit("BTC", Number(10), types.BalanceMap{})
		assert.InDelta(t, 4.4563, allowed.Float64(), limitTol)
		assert.Equal(t, sessionBudgetName, limit.Budget)
	})

	t.Run("a session that is over stops an asset that is within its budget", func(t *testing.T) {
		l := newTestLimiter(budget)
		basis := testBasis("100")
		basis.Assets["ETH"] = chargedInterest{USDT: fixedpoint.Zero, Complete: true}
		l.set(basis)

		allowed, limit := l.limit("ETH", Number(1), types.BalanceMap{})
		assert.True(t, allowed.IsZero())
		assert.Equal(t, borrowLimit{Reason: borrowLimitOverBudget, Budget: sessionBudgetName}, limit)
	})
}

// withBorrowLimit turns limitBorrow on for the budget the worker already has,
// the way newInterestEstimateWorker does for a config that has it set
func withBorrowLimit(w *interestEstimateWorker) *interestBudgetLimiter {
	w.config.Budget.LimitBorrow = true
	w.limiter = newInterestBudgetLimiter(w.config, w.pricer)
	w.limiter.now = w.now
	return w.limiter
}

func TestInterestBudgetLimit_WorkerBuildsALimiterOnlyWhenSet(t *testing.T) {
	build := func(budget *InterestBudgetConfig) *interestEstimateWorker {
		s := newTestStrategy(t, newTestAccount(Number(10)), newFakeMarginBorrowRepayService(nil), configured("USDT")...)
		return newInterestEstimateWorker(s, s.ExchangeSession, &InterestEstimateConfig{
			Interval: types.Duration(5 * time.Minute),
			Budget:   budget,
		})
	}

	assert.Nil(t, build(nil).limiter)
	assert.Nil(t, build(&InterestBudgetConfig{MonthlyUSDT: Number(100)}).limiter)
	assert.NotNil(t, build(&InterestBudgetConfig{MonthlyUSDT: Number(100), LimitBorrow: true}).limiter)
}

// The alert of a session says that its budgets limit borrowing only when they do
func TestInterestBudgetLimit_WorkerTellsTheAlerter(t *testing.T) {
	build := func(limitBorrow bool) *interestEstimateWorker {
		s := newTestStrategy(t, newTestAccount(Number(10)), newFakeMarginBorrowRepayService(nil), configured("USDT")...)
		s.InterestBudgetAlert = &InterestBudgetAlertConfig{}
		return newInterestEstimateWorker(s, s.ExchangeSession, &InterestEstimateConfig{
			Interval: types.Duration(5 * time.Minute),
			Budget:   &InterestBudgetConfig{MonthlyUSDT: Number(100), LimitBorrow: limitBorrow},
		})
	}

	off := build(false)
	require.NotNil(t, off.budgetAlerter)
	assert.False(t, off.budgetAlerter.limitsBorrow)

	on := build(true)
	require.NotNil(t, on.budgetAlerter)
	assert.True(t, on.budgetAlerter.limitsBorrow)
	assert.True(t, on.budgetAlerter.build(&interestEstimate{Month: margininterest.MonthStart(testNow)}).LimitsBorrow)
}

// The budgets are the session's, so another autoborrow instance borrows under
// them too, and its assets are not the ones of the instance that owns the
// estimate. The estimate learns them when a limit is asked for.
func TestInterestBudgetLimit_ValuesAnAssetOnceItsLimitIsAsked(t *testing.T) {
	account := newTestAccount(Number(10))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001", "BTC": "0.000001"}}
	w := newTestInterestWorker(t, account, rates, &fakeHistoryStore{}, configured("USDT")...)
	w.config.Budget = &InterestBudgetConfig{MonthlyUSDT: Number(100)}
	limiter := withBorrowLimit(w)

	var priced []string
	w.queryPrice = func(_ context.Context, asset string) error {
		priced = append(priced, asset)
		return nil
	}

	w.tick(context.Background())
	assert.Empty(t, priced)

	// BTC is not an asset of this instance, and has no debt: it has no rate yet
	allowed, limit := limiter.limit("BTC", Number(1), types.BalanceMap{})
	assert.True(t, allowed.IsZero())
	assert.Equal(t, borrowLimit{Reason: borrowLimitUnknown, Budget: sessionBudgetName}, limit)

	// the next tick has its rate and its price: one BTC costs 22.44 of 100 USDT
	w.tick(context.Background())
	assert.Equal(t, []string{"BTC"}, priced)
	assert.Contains(t, rates.calls[len(rates.calls)-1], "BTC")

	allowed, limit = limiter.limit("BTC", Number(1), types.BalanceMap{})
	assert.Equal(t, Number(1), allowed)
	assert.False(t, limit.limited())
}

// The limiter gets the figures of every tick, and reads the debts itself
func TestInterestBudgetLimit_FollowsTheEstimate(t *testing.T) {
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		usdtRow(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), "10"),
	}}
	// 1000 USDT borrowed at 0.000001 per hour
	w := newBudgetWorker(t, "100", history)
	limiter := withBorrowLimit(w)

	_, limit := limiter.limit("USDT", Number(1), types.BalanceMap{})
	assert.Equal(t, borrowLimitNoEstimate, limit.Reason)

	w.tick(context.Background())

	// 10 USDT are charged, so 90 are left. One USDT borrowed now costs 374 x
	// 0.000001 = 0.000374 USDT to the month end: 90 / 0.000374 = 240641.71 USDT fit.
	allowed, limit := limiter.limit("USDT", Number(1000000), types.BalanceMap{})
	assert.InDelta(t, 240641.71, allowed.Float64(), 0.01)
	assert.Equal(t, borrowLimit{Reason: borrowLimitClamped, Budget: sessionBudgetName}, limit)

	// a debt opened after the tick is counted: 100000 USDT of debt will be
	// charged 37.3 USDT more, which leaves 52.7: 52.7 / 0.000374 = 140909.09
	debts := debtsOf(debtBalance("USDT", fixedpoint.Zero, Number(100000)))
	allowed, _ = limiter.limit("USDT", Number(1000000), debts)
	assert.InDelta(t, 140909.09, allowed.Float64(), 0.01)
}

// An estimate that is not complete allows nothing, and the borrow limit is
// back as soon as a tick is complete again
func TestInterestBudgetLimit_IncompleteEstimateAllowsNothing(t *testing.T) {
	history := &fakeHistoryStore{syncErr: map[string]error{"USDT": errors.New("sync failed")}}
	w := newBudgetWorker(t, "100", history)
	limiter := withBorrowLimit(w)

	est := w.tick(context.Background())
	require.Equal(t, margininterest.BudgetStatusUnknown, est.Budget.Status())

	allowed, limit := limiter.limit("USDT", Number(1), types.BalanceMap{})
	assert.True(t, allowed.IsZero())
	assert.Equal(t, borrowLimit{Reason: borrowLimitUnknown, Budget: sessionBudgetName}, limit)

	history.syncErr = nil
	est = w.tick(context.Background())
	require.Equal(t, margininterest.BudgetStatusOK, est.Budget.Status())

	allowed, limit = limiter.limit("USDT", Number(1), types.BalanceMap{})
	assert.Equal(t, Number(1), allowed)
	assert.False(t, limit.limited())
}

// A failed sync of one asset stops the borrows under that asset's budget only
func TestInterestBudgetLimit_IncompleteIsPerAssetBudget(t *testing.T) {
	history := &fakeHistoryStore{syncErr: map[string]error{"ETH": errors.New("sync failed")}}
	w := newAssetBudgetWorker(t, history, map[string]string{"USDT": "100", "ETH": "100"})
	limiter := withBorrowLimit(w)

	w.tick(context.Background())

	_, limit := limiter.limit("ETH", Number(1), types.BalanceMap{})
	assert.Equal(t, borrowLimit{Reason: borrowLimitUnknown, Budget: "ETH"}, limit)

	allowed, limit := limiter.limit("USDT", Number(1), types.BalanceMap{})
	assert.Equal(t, Number(1), allowed)
	assert.False(t, limit.limited())
}

// The interest of a borrow is valued before the asset has any debt, so with a
// borrow limit the autoborrow assets are priced on every tick too
func TestInterestBudgetLimit_PricesTheAutoborrowAssets(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("USDT", fixedpoint.Zero, Number(1000)))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001", "ETH": "0.000001"}}
	w := newTestInterestWorker(t, account, rates, &fakeHistoryStore{}, configured("USDT", "ETH")...)
	w.config.Budget = &InterestBudgetConfig{MonthlyUSDT: Number(100)}

	var queried []string
	failing := map[string]bool{}
	w.queryPrice = func(_ context.Context, asset string) error {
		queried = append(queried, asset)
		if failing[asset] {
			return errors.New("no ticker")
		}
		return nil
	}

	// without a limit only the assets with debt are priced, and USDT needs no price
	w.tick(context.Background())
	assert.Empty(t, queried)

	limiter := withBorrowLimit(w)
	w.tick(context.Background())
	assert.Equal(t, []string{"ETH"}, queried)

	allowed, limit := limiter.limit("ETH", Number(1), account.Balances().Debts())
	assert.Equal(t, Number(1), allowed)
	assert.False(t, limit.limited())

	// a price that could not be queried is not a price to borrow at
	failing["ETH"] = true
	w.tick(context.Background())

	_, limit = limiter.limit("ETH", Number(1), account.Balances().Debts())
	assert.Equal(t, borrowLimit{Reason: borrowLimitUnknown, Budget: sessionBudgetName}, limit)

	// USDT does not depend on it
	allowed, limit = limiter.limit("USDT", Number(1), account.Balances().Debts())
	assert.Equal(t, Number(1), allowed)
	assert.False(t, limit.limited())
}

// The borrow loop and the estimate run on their own goroutines: the limit is
// read while a tick replaces the figures. Run with -race.
func TestInterestBudgetLimit_IsReadWhileTheEstimateTicks(t *testing.T) {
	w := newAssetBudgetWorker(t, &fakeHistoryStore{}, map[string]string{"USDT": "100", "ETH": "100"})
	w.config.Budget.MonthlyUSDT = Number(1000)
	limiter := withBorrowLimit(w)
	debts := debtsOf(debtBalance("ETH", fixedpoint.Zero, Number(10)))

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			w.tick(context.Background())
		}
	}()

	for i := 0; i < 500; i++ {
		limiter.limit("ETH", Number(1), debts)
		limiter.limit("USDT", Number(1), debts)
	}
	<-done

	allowed, limit := limiter.limit("ETH", Number(1), debts)
	assert.Equal(t, Number(1), allowed)
	assert.False(t, limit.limited())
}

// A budget that limits borrowing on an estimate that can never be complete
// would silently stop every borrow, so it does not start
func TestInterestBudgetLimit_NeedsAnEstimateThatCanBeComplete(t *testing.T) {
	noRates := newBudgetWorker(t, "100", &fakeHistoryStore{})
	withBorrowLimit(noRates)
	noRates.interestQueryService = nil
	_, err := noRates.claimBudget()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "limitBorrow")
	assert.Contains(t, err.Error(), "hourly interest rate API")

	noDatabase := newBudgetWorker(t, "100", &fakeHistoryStore{})
	withBorrowLimit(noDatabase)
	noDatabase.history = nil
	_, err = noDatabase.claimBudget()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database")

	// a budget that only reports still starts: its status says unknown on every tick
	reportOnly := newBudgetWorker(t, "100", &fakeHistoryStore{})
	reportOnly.history = nil
	release, err := reportOnly.claimBudget()
	require.NoError(t, err)
	release()
}

func TestInterestBudgetLimit_LimiterOfTheSession(t *testing.T) {
	w := newBudgetWorker(t, "100", &fakeHistoryStore{})
	assert.Nil(t, interestBudgetLimiterOf(w.session))
	assert.Nil(t, interestBudgetLimiterOf(nil))

	// a budget that only reports has no limiter
	release, err := w.claimBudget()
	require.NoError(t, err)
	assert.Nil(t, interestBudgetLimiterOf(w.session))
	release()

	limiter := withBorrowLimit(w)
	release, err = w.claimBudget()
	require.NoError(t, err)
	assert.Same(t, limiter, interestBudgetLimiterOf(w.session))

	// it goes with the budget when the instance stops
	release()
	assert.Nil(t, interestBudgetLimiterOf(w.session))
}

func borrowLimitedCount(t *testing.T, session, asset string, reason borrowLimitReason) float64 {
	t.Helper()
	for _, series := range gathered(t, "autoborrow_interest_budget_borrow_limited_total") {
		if series["session"] == session && series["asset"] == asset && series["reason"] == string(reason) {
			return series["value"].(float64)
		}
	}
	return 0
}

// newLimitedStrategy is a strategy that wants to borrow 100 ETH, on a session
// with 10 ETH of debt and the given session budget, which limits borrowing.
// The estimate has not ticked yet.
func newLimitedStrategy(
	t *testing.T, budget string, history *fakeHistoryStore,
) (*Strategy, *interestEstimateWorker, *fakeMarginBorrowRepayService) {
	t.Helper()

	service := newFakeMarginBorrowRepayService(map[string]fixedpoint.Value{"ETH": Number(10000)})
	account := newTestAccount(Number(10), debtBalance("ETH", fixedpoint.Zero, Number(10)))
	s := newTestStrategy(t, account, service, MarginAssetConfig{Asset: "ETH", Low: Number(100)})

	rates := &fakeRateService{rates: map[string]string{"ETH": "0.000001"}}
	w := newTestInterestWorkerOf(t, s, rates, history)
	w.config.Budget = &InterestBudgetConfig{MonthlyUSDT: Number(budget)}
	withBorrowLimit(w)

	release, err := w.claimBudget()
	require.NoError(t, err)
	t.Cleanup(release)

	return s, w, service
}

func TestCheckAndBorrow_InterestBudgetLimit(t *testing.T) {
	ctx := context.Background()

	t.Run("a borrow is reduced to what the budget can pay for", func(t *testing.T) {
		s, w, service := newLimitedStrategy(t, "100", &fakeHistoryStore{})
		w.tick(ctx)

		s.checkAndBorrow(ctx)

		// 10 ETH of debt project 11.19 USDT to the month end: 88.81 / 1.122 = 79.1533 ETH fit
		assert.InDelta(t, 79.1533, service.borrowed["ETH"].Float64(), limitTol)
		assert.Equal(t, 1.0, borrowLimitedCount(t, s.ExchangeSession.Name, "ETH", borrowLimitClamped))
	})

	// the test account never changes, so the next run wants the same 100 ETH again
	t.Run("the charge of a borrow counts in the next run", func(t *testing.T) {
		s, w, service := newLimitedStrategy(t, "100", &fakeHistoryStore{})
		w.tick(ctx)

		s.checkAndBorrow(ctx)
		first := service.borrowed["ETH"]
		require.InDelta(t, 79.1533, first.Float64(), limitTol)

		// 79.1533 ETH were charged 0.2375 USDT when they were borrowed, which the
		// history does not show yet: 88.5725 / 1.122 = 78.9416 ETH fit now
		s.checkAndBorrow(ctx)
		assert.InDelta(t, 78.9416, service.borrowed["ETH"].Float64(), limitTol)
		assert.Equal(t, 2.0, borrowLimitedCount(t, s.ExchangeSession.Name, "ETH", borrowLimitClamped))
	})

	t.Run("a borrow that fits is made in full", func(t *testing.T) {
		s, w, service := newLimitedStrategy(t, "1000", &fakeHistoryStore{})
		w.tick(ctx)

		s.checkAndBorrow(ctx)

		assert.Equal(t, Number(100), service.borrowed["ETH"])
		assert.Zero(t, borrowLimitedCount(t, s.ExchangeSession.Name, "ETH", borrowLimitClamped))
	})

	t.Run("nothing is borrowed over the budget", func(t *testing.T) {
		// the 10 ETH of debt alone project 11.19 USDT to the month end
		s, w, service := newLimitedStrategy(t, "10", &fakeHistoryStore{})
		w.tick(ctx)

		s.checkAndBorrow(ctx)

		assert.Empty(t, service.borrowed)
		assert.Empty(t, service.repaid, "the limit never repays")
		assert.Equal(t, 1.0, borrowLimitedCount(t, s.ExchangeSession.Name, "ETH", borrowLimitOverBudget))
	})

	t.Run("nothing is borrowed on an incomplete estimate", func(t *testing.T) {
		history := &fakeHistoryStore{syncErr: map[string]error{"ETH": errors.New("sync failed")}}
		s, w, service := newLimitedStrategy(t, "1000", history)
		w.tick(ctx)

		s.checkAndBorrow(ctx)

		assert.Empty(t, service.borrowed)
		assert.Equal(t, 1.0, borrowLimitedCount(t, s.ExchangeSession.Name, "ETH", borrowLimitUnknown))
	})

	t.Run("nothing is borrowed before the first estimate", func(t *testing.T) {
		s, _, service := newLimitedStrategy(t, "1000", &fakeHistoryStore{})

		s.checkAndBorrow(ctx)

		assert.Empty(t, service.borrowed)
		assert.Equal(t, 1.0, borrowLimitedCount(t, s.ExchangeSession.Name, "ETH", borrowLimitNoEstimate))
	})

	t.Run("a clamped borrow below the minimum is not made", func(t *testing.T) {
		s, w, service := newLimitedStrategy(t, "100", &fakeHistoryStore{})
		// 79.1533 ETH fit the budget
		s.Assets[0].MinQuantityPerBorrow = Number(90)
		w.tick(ctx)

		s.checkAndBorrow(ctx)

		assert.Empty(t, service.borrowed)
	})

	// the budget is the session's: an instance that sets none borrows under it too
	t.Run("another instance on the session is limited too", func(t *testing.T) {
		s, w, _ := newLimitedStrategy(t, "100", &fakeHistoryStore{})
		w.tick(ctx)

		service := newFakeMarginBorrowRepayService(map[string]fixedpoint.Value{"ETH": Number(10000)})
		other := newTestStrategy(t, s.ExchangeSession.Account, service, MarginAssetConfig{Asset: "ETH", Low: Number(100)})
		other.ExchangeSession = s.ExchangeSession

		other.checkAndBorrow(ctx)

		assert.InDelta(t, 79.1533, service.borrowed["ETH"].Float64(), limitTol)
	})

	t.Run("a budget that only reports does not limit", func(t *testing.T) {
		service := newFakeMarginBorrowRepayService(map[string]fixedpoint.Value{"ETH": Number(10000)})
		account := newTestAccount(Number(10), debtBalance("ETH", fixedpoint.Zero, Number(10)))
		s := newTestStrategy(t, account, service, MarginAssetConfig{Asset: "ETH", Low: Number(100)})

		w := newTestInterestWorkerOf(t, s, &fakeRateService{rates: map[string]string{"ETH": "0.000001"}}, &fakeHistoryStore{})
		w.config.Budget = &InterestBudgetConfig{MonthlyUSDT: Number(20)}
		release, err := w.claimBudget()
		require.NoError(t, err)
		defer release()

		est := w.tick(ctx)
		require.Equal(t, margininterest.BudgetStatusOK, est.Budget.Status())

		s.checkAndBorrow(ctx)

		assert.Equal(t, Number(100), service.borrowed["ETH"])
	})
}

func TestInterestBudgetLimitConfig_Validate(t *testing.T) {
	interval := types.Duration(5 * time.Minute)

	assert.NoError(t, (&InterestEstimateConfig{
		Interval: interval, Budget: &InterestBudgetConfig{MonthlyUSDT: Number(100), LimitBorrow: true},
	}).Validate())
	assert.NoError(t, (&InterestEstimateConfig{
		Interval: interval,
		Budget:   &InterestBudgetConfig{Assets: map[string]fixedpoint.Value{"ETH": Number(100)}, LimitBorrow: true},
	}).Validate())

	// a limit with no budget to limit to is an error, not a silent no-op
	err := (&InterestEstimateConfig{Interval: interval, Budget: &InterestBudgetConfig{LimitBorrow: true}}).Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "limitBorrow")

	err = (&InterestEstimateConfig{
		Interval: interval,
		Budget:   &InterestBudgetConfig{Assets: map[string]fixedpoint.Value{"ETH": fixedpoint.Zero}, LimitBorrow: true},
	}).Validate()
	require.Error(t, err)

	var c InterestEstimateConfig
	require.NoError(t, json.Unmarshal([]byte(`{"interval": "5m", "budget": {"monthlyUSDT": 1000, "limitBorrow": true}}`), &c))
	assert.True(t, c.limitsBorrow())

	c = InterestEstimateConfig{}
	require.NoError(t, json.Unmarshal([]byte(`{"interval": "5m", "budget": {"monthlyUSDT": 1000}}`), &c))
	assert.False(t, c.limitsBorrow())
	assert.False(t, (&InterestEstimateConfig{}).limitsBorrow())
}

func TestInterestBudgetAlert_SaysWhenTheBudgetsLimitBorrowing(t *testing.T) {
	alert := &InterestBudgetAlert{
		SessionName: "margin",
		Month:       margininterest.MonthStart(testNow),
		UpdatedAt:   testNow,
		Budgets: []interestBudgetLine{{
			Subject: "Session budget",
			Budget:  margininterest.EvaluateBudget(Number(100), Number(101), Number(120), true),
		}},
	}

	assert.NotContains(t, alert.SlackAttachment().Text, "Autoborrow borrows only")

	alert.LimitsBorrow = true
	assert.Contains(t, alert.SlackAttachment().Text,
		"\nUpdated at 2026-11-15 10:20:00 UTC\nAutoborrow borrows only what these budgets can still pay for. Nothing is repaid.")
}
