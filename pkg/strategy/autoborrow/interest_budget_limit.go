package autoborrow

import (
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/accounting/margininterest"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// budgetBasisMaxIntervals is how many estimate intervals the figures of a tick
// may be used for. After that the estimate is not running as it should, and
// what it last said is not known to hold any more.
const budgetBasisMaxIntervals = 3

// sessionBudgetName names the budget of the whole session in a borrow limit
const sessionBudgetName = "session"

// borrowLimitReason says why a borrow was reduced or refused, empty when it was not
type borrowLimitReason string

const (
	// borrowLimitNoEstimate: there is no estimate to compute a limit from. None was
	// made yet, the last one is too old, or it is of another month.
	borrowLimitNoEstimate borrowLimitReason = "no_estimate"

	// borrowLimitUnknown: the estimate may be missing a part, so what is left of
	// the budget is not known. It may be less than the figures say.
	borrowLimitUnknown borrowLimitReason = "unknown"

	// borrowLimitOverBudget: nothing is left of the budget
	borrowLimitOverBudget borrowLimitReason = "over_budget"

	// borrowLimitClamped: only a part of the borrow fits what is left of the budget
	borrowLimitClamped borrowLimitReason = "clamped"
)

// borrowLimit describes how a budget limited a borrow
type borrowLimit struct {
	Reason borrowLimitReason

	// Budget names the budget that set the limit: an asset, or sessionBudgetName.
	// Empty when there is no estimate: then no budget was looked at.
	Budget string
}

// limited reports whether the borrow was reduced or refused
func (l borrowLimit) limited() bool {
	return l.Reason != ""
}

// chargedInterest is the interest charged against one budget so far this month
type chargedInterest struct {
	USDT fixedpoint.Value

	// Complete is false when USDT may be missing a charge. It is then a lower bound.
	Complete bool
}

// interestBudgetBasis is what one estimate tick knows about the budgets of the
// session, apart from the debts: those are read at the moment of a borrow, so a
// borrow made after the tick is counted. It is not changed once it is set.
type interestBudgetBasis struct {
	// Month is the UTC calendar month of the tick, and Hours its top-of-hour
	// charges that were left at the tick
	Month time.Time
	Hours int

	// SyncedAt is when the last sync of the whole interest history started, zero
	// before the first one. What was charged is known up to about then.
	SyncedAt time.Time

	// Rates are the next hourly rates of the assets autoborrow may borrow and of
	// the assets that had debt at the tick
	Rates types.MarginNextHourlyInterestRateMap

	// StalePrice are the assets whose price could not be queried on the tick
	StalePrice map[string]struct{}

	// Session is what the session was charged, nil without a session budget.
	// Assets is the same for each asset that has a budget.
	Session *chargedInterest
	Assets  map[string]chargedInterest
}

// budgetScope is one budget a borrow has to fit, with the figures of a tick
type budgetScope struct {
	// name is the asset of an asset budget, or sessionBudgetName
	name   string
	amount fixedpoint.Value

	// charged is what the budget was charged this month, nil when the tick has no
	// figures for it
	charged *chargedInterest

	// session says the budget is the session's: every asset counts against it.
	// Otherwise only the asset called name does.
	session bool
}

// covers reports whether the interest of asset counts against the budget
func (b budgetScope) covers(asset string) bool {
	return b.session || asset == b.name
}

// assets returns the assets of debts whose interest counts against the budget
func (b budgetScope) assets(debts types.BalanceMap) []string {
	if b.session {
		return sortedKeys(debts)
	}
	return []string{b.name}
}

// borrowCharge is the interest a borrow was charged when it was made, which the
// interest history does not show yet
type borrowCharge struct {
	at    time.Time
	asset string
	usdt  fixedpoint.Value
}

// chargeCost is what one hourly charge of an asset cost when a borrow of it was
// last fitted: the rate and the price the limit was computed with
type chargeCost struct {
	hourlyRate fixedpoint.Value
	price      fixedpoint.Value
}

// interestBudgetLimiter limits what autoborrow borrows to the interest budgets
// of one session.
//
// The estimate worker gives it the figures of every tick. The borrow loop asks
// it before each borrow, with the debts the account has at that moment, and
// never waits for the estimate: a limit that cannot be computed is zero.
//
// It only ever reduces a borrow. It does not repay.
type interestBudgetLimiter struct {
	config *InterestEstimateConfig
	pricer margininterest.PriceFunc
	now    func() time.Time

	mu    sync.Mutex
	basis *interestBudgetBasis
	setAt time.Time

	// asked are the assets a limit was asked for. Any autoborrow instance of the
	// session asks, so they are not all assets of the instance that owns the
	// estimate: it reads them to query their rate and price on the next tick.
	asked map[string]struct{}

	// fitted is what one charge of an asset cost when its last borrow was fitted
	fitted map[string]chargeCost

	// charges are the charges of the borrows made under the limit that the
	// interest history may not show yet. A borrow is charged once at the moment it
	// is made, and that charge is in what the budget was charged only after a sync.
	charges []borrowCharge

	// reported is how the last borrow of an asset was limited, to log a change once
	reported map[string]borrowLimit
}

func newInterestBudgetLimiter(config *InterestEstimateConfig, pricer margininterest.PriceFunc) *interestBudgetLimiter {
	return &interestBudgetLimiter{
		config:   config,
		pricer:   pricer,
		now:      time.Now,
		asked:    make(map[string]struct{}),
		fitted:   make(map[string]chargeCost),
		reported: make(map[string]borrowLimit),
	}
}

// set replaces the figures with the ones of a new tick
func (l *interestBudgetLimiter) set(basis *interestBudgetBasis) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.basis = basis
	l.setAt = l.now()

	kept := l.charges[:0]
	for _, c := range l.charges {
		if !chargeIsSynced(basis, c) {
			kept = append(kept, c)
		}
	}
	l.charges = kept
}

// chargeIsSynced reports whether the charge of a borrow is no longer the
// limiter's to count: it is of another month, or the interest history was
// synced late enough after the borrow to hold it. The exchange can publish a
// charge late, so the sync has to start syncedOverlap after the borrow, as the
// next sync of an asset starts that long before the last one.
func chargeIsSynced(basis *interestBudgetBasis, c borrowCharge) bool {
	if c.at.Before(basis.Month) {
		return true
	}
	return !basis.SyncedAt.IsZero() && basis.SyncedAt.Sub(c.at) >= syncedOverlap
}

// maxAge is how long the figures of a tick are used
func (l *interestBudgetLimiter) maxAge() time.Duration {
	return budgetBasisMaxIntervals * l.config.Interval.Duration()
}

// current returns the figures to compute a limit from at now, nil when there
// are none: no tick yet, the last one is too old, or it is of another month.
// The caller holds mu.
func (l *interestBudgetLimiter) current(now time.Time) *interestBudgetBasis {
	if l.basis == nil || now.Sub(l.setAt) > l.maxAge() {
		return nil
	}

	if !margininterest.MonthStart(now).Equal(l.basis.Month) {
		return nil
	}

	return l.basis
}

// askedAssets returns the assets a limit was asked for so far, sorted
func (l *interestBudgetLimiter) askedAssets() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return sortedKeys(l.asked)
}

// limit returns how much of quantity of asset may be borrowed under the budgets
// of the session, given the debts the account has now.
//
// The budget of the asset and the budget of the session both apply, and the
// smaller limit wins. An asset no budget applies to is not limited.
//
// Under each budget the borrow has to fit what is left of the calendar month:
// the budget, less what was charged, less the debts held to the month end. So a
// month that was charged little so far leaves more to borrow with. The month
// after is not looked at: a debt taken near the month end can start the next
// month over its budget, and nothing here repays it.
//
// A budget whose figures may be missing a part allows nothing: what is left of
// it is then not known.
func (l *interestBudgetLimiter) limit(asset string, quantity fixedpoint.Value, debts types.BalanceMap) (fixedpoint.Value, borrowLimit) {
	sessionBudget, hasSessionBudget := l.config.sessionBudget()
	assetBudget, hasAssetBudget := l.config.assetBudgets()[asset]
	if !hasSessionBudget && !hasAssetBudget {
		return quantity, borrowLimit{}
	}

	if quantity.Sign() <= 0 {
		return fixedpoint.Zero, borrowLimit{}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.asked[asset] = struct{}{}

	basis := l.current(l.now())
	if basis == nil {
		return fixedpoint.Zero, borrowLimit{Reason: borrowLimitNoEstimate}
	}

	var budgets []budgetScope
	if hasAssetBudget {
		b := budgetScope{name: asset, amount: assetBudget}
		if c, ok := basis.Assets[asset]; ok {
			b.charged = &c
		}
		budgets = append(budgets, b)
	}
	if hasSessionBudget {
		budgets = append(budgets, budgetScope{name: sessionBudgetName, amount: sessionBudget, charged: basis.Session, session: true})
	}

	allowed, limit := quantity, borrowLimit{}
	for _, budget := range budgets {
		fit, reason := l.fit(basis, budget, asset, allowed, debts)
		if reason == "" {
			continue
		}

		// the budget that reduced the borrow last is the one that set the limit
		allowed, limit = fit, borrowLimit{Reason: reason, Budget: budget.name}
		if allowed.Sign() <= 0 {
			break
		}
	}

	return allowed, limit
}

// fit returns how much of quantity of asset fits budget, and why it is less
// than quantity, if it is. The caller holds mu.
func (l *interestBudgetLimiter) fit(
	basis *interestBudgetBasis, budget budgetScope, asset string, quantity fixedpoint.Value, debts types.BalanceMap,
) (fixedpoint.Value, borrowLimitReason) {
	if budget.charged == nil {
		return fixedpoint.Zero, borrowLimitUnknown
	}

	scope := budget.assets(debts)

	// what the debts will still be charged this month, if nothing changes
	projected := margininterest.ProjectAssets(scope, debts, basis.Rates, l.pricer, basis.Hours)
	headroom := budget.amount.Sub(budget.charged.USDT).Sub(l.unsyncedCharges(basis, budget)).Sub(projected.TotalUSDT)

	// the figures are lower bounds, so over the budget is certain even when a
	// part of them is missing
	if headroom.Sign() <= 0 {
		return fixedpoint.Zero, borrowLimitOverBudget
	}

	if !budget.charged.Complete || !projected.Complete() || anyIn(basis.StalePrice, scope) {
		return fixedpoint.Zero, borrowLimitUnknown
	}

	// the cost of the borrow itself needs a rate and a current price
	rate, ok := basis.Rates[asset]
	if !ok || rate == nil || rate.HourlyRate.Sign() <= 0 {
		return fixedpoint.Zero, borrowLimitUnknown
	}

	price, ok := margininterest.PriceOf(l.pricer, asset)
	if _, stale := basis.StalePrice[asset]; !ok || stale {
		return fixedpoint.Zero, borrowLimitUnknown
	}

	l.fitted[asset] = chargeCost{hourlyRate: rate.HourlyRate, price: price}

	fit := margininterest.FitBorrow(quantity, headroom, rate.HourlyRate, price, margininterest.BorrowCharges(basis.Hours))

	switch {
	case fit.Compare(quantity) >= 0:
		return quantity, ""
	case fit.Sign() <= 0:
		return fixedpoint.Zero, borrowLimitOverBudget
	default:
		return fit, borrowLimitClamped
	}
}

// unsyncedCharges returns what the borrows made under the limit were charged
// when they were made, as far as it counts against budget and is not yet in
// what the budget was charged. The caller holds mu.
func (l *interestBudgetLimiter) unsyncedCharges(basis *interestBudgetBasis, budget budgetScope) fixedpoint.Value {
	total := fixedpoint.Zero
	for _, c := range l.charges {
		if budget.covers(c.asset) && !chargeIsSynced(basis, c) {
			total = total.Add(c.usdt)
		}
	}
	return total
}

// borrowed tells the limiter that quantity of asset was borrowed. The borrow
// is charged once right away, counted as a whole hour like the limit counted
// it. Until the interest history shows that charge the limiter counts it
// itself: without it, the borrows of the last hours of a month, which pay
// little else, would each see the same budget left.
func (l *interestBudgetLimiter) borrowed(asset string, quantity fixedpoint.Value) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// an asset no budget applies to was never fitted, and counts against none
	cost, ok := l.fitted[asset]
	if !ok || quantity.Sign() <= 0 {
		return
	}

	l.charges = append(l.charges, borrowCharge{
		at:    l.now(),
		asset: asset,
		usdt:  margininterest.ProjectInterest(quantity, cost.hourlyRate, 1).Mul(cost.price),
	})
}

// changed reports whether limit is not how the last borrow of asset was
// limited, and remembers it. The borrow loop asks again on every run, and a
// limit that stays is worth one log line, not one per run.
func (l *interestBudgetLimiter) changed(asset string, limit borrowLimit) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.reported[asset] == limit {
		return false
	}

	l.reported[asset] = limit
	return true
}

// anyIn reports whether one of keys is in set
func anyIn(set map[string]struct{}, keys []string) bool {
	for _, key := range keys {
		if _, ok := set[key]; ok {
			return true
		}
	}
	return false
}

// interestBudgetLimiterOf returns the limiter of the interest budgets of
// session, nil when the session has no budget or its budget does not limit
// borrowing. The budget is the session's, so every autoborrow instance on the
// session borrows under it, also one that sets no budget itself.
func interestBudgetLimiterOf(session *bbgo.ExchangeSession) *interestBudgetLimiter {
	if session == nil {
		return nil
	}

	budgetedSessions.Lock()
	defer budgetedSessions.Unlock()

	return budgetedSessions.m[budgetedSessionKey(session)]
}
