package autoborrow

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/accounting/margininterest"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/currency"
)

// maxRateAssetsPerQuery is the asset limit of Binance's next-hourly-interest-rate API
const maxRateAssetsPerQuery = 20

const defaultInterestSyncInterval = time.Hour

// syncedOverlap is how far before the last successful sync of an asset the next
// sync starts, for a charge published after that sync
const syncedOverlap = time.Hour

// priceQueryTimeout bounds the price queries of one tick: one ticker request
// per asset, one after the other. It is long enough for a rate-limited or slow
// exchange to answer all of them, since a price that is not queried in time is
// stale and makes the estimate incomplete.
const priceQueryTimeout = 2 * time.Minute

// valuationTimeout bounds the valuation pass of one tick. The first pass after
// a start values the rows of the whole month and syncs the klines of every
// price pair, so it gets as long as a tick usually is.
const valuationTimeout = 5 * time.Minute

// defaultProjectionMonths are the rolling months (30 days each) projected when
// projectionMonths is not set
var defaultProjectionMonths = []int{1, 3}

// InterestEstimateConfig enables the margin interest estimate of the session.
//
// The estimate covers every margin interest of the session: the assets
// configured in autoborrow, borrowed or not, plus every asset the session has
// debt in or was charged interest for this month. The configured assets are
// read on every tick and a debt counts whoever opened it, so the estimate does
// not depend on the asset list staying what it was at startup. An asset stays
// in the estimate until the month ends.
//
// Interest paid in another asset (Binance BNB deduction) counts toward the
// borrowed asset: its amount at the nominal interest, its USDT value at the BNB
// actually charged. On Binance, BNB rows are synced too, since the exchange
// returns them only for asset=BNB.
//
// Every figure is counted in hourly charges. Two month windows are built on them:
//
//   - the calendar month: realized since the UTC month start plus the projection
//     until the month end
//
//   - the rolling month: 30 x 24 hours. projectionMonths [1, 3] projects the
//     current principal over the next 720 and 2160 hours.
//
//     interestEstimate:
//     interval: 5m
//     syncInterval: 1h
//     realizedPriceMode: charged # or current
//     projectionMonths: [1, 3]   # default
//     budget:
//     monthlyUSDT: 1000        # the whole session; 0 or not set: no budget
//     assets:                  # per asset
//     USDT: 600
//     limitBorrow: true        # borrow only what the budgets can still pay for
type InterestEstimateConfig struct {
	// Interval is the estimate interval, 0 turns the estimate off
	Interval types.Duration `json:"interval"`

	// SyncInterval is how often the interest history is synced into the DB, 0 means 1h.
	// The month total can be low by about ceil(SyncInterval/1h)+1 hourly charges per asset.
	SyncInterval types.Duration `json:"syncInterval"`

	// RealizedPriceMode values realized interest at the price when charged ("charged", default)
	// or at the current price ("current")
	RealizedPriceMode margininterest.PriceMode `json:"realizedPriceMode"`

	// ProjectionMonths are the rolling months (30 x 24 hours each) to project the
	// current principal over, from now. Not set means [1, 3], an empty list means none.
	ProjectionMonths []int `json:"projectionMonths,omitempty"`

	// Budget holds the interest budgets of the session, compared against the
	// estimate on every tick. Not set means no budget.
	Budget *InterestBudgetConfig `json:"budget,omitempty"`
}

// InterestBudgetConfig holds the margin interest budgets of one session: one
// for the whole session, and one per asset. Each is optional and independent:
// the asset budgets do not have to add up to the session budget.
//
// A budget reports: a status in the log and in the metrics. It does not alert,
// and it does not change how autoborrow borrows unless limitBorrow is set. It
// never repays.
//
// It always compares what was charged, at the price when it was charged,
// whatever realizedPriceMode is: a budget is about money already spent. The
// projected part is at the current price.
type InterestBudgetConfig struct {
	// MonthlyUSDT is the budget of the session per UTC calendar month, summed
	// over every asset the session pays interest for, in USDT. 0 means no session budget.
	MonthlyUSDT fixedpoint.Value `json:"monthlyUSDT"`

	// Assets are the budgets per asset and UTC calendar month, in USDT. An asset
	// here has to be an asset of the exchange. It does not have to be an
	// autoborrow asset.
	// Interest paid in another asset (e.g. BNB) counts toward the asset that was
	// borrowed. 0 means no budget for the asset.
	Assets map[string]fixedpoint.Value `json:"assets,omitempty"`

	// LimitBorrow makes autoborrow borrow only what the budgets can still pay
	// for: a borrow is reduced to the part whose interest fits the budget of its
	// asset and the budget of the session, and refused when nothing fits or when
	// the estimate is incomplete. Nothing is repaid. Off by default: the budgets
	// then only report.
	LimitBorrow bool `json:"limitBorrow,omitempty"`
}

// Validate checks the price mode, the projection months and the budget
func (c *InterestEstimateConfig) Validate() error {
	if err := c.RealizedPriceMode.Validate(); err != nil {
		return err
	}

	if c.Budget != nil {
		if c.Budget.MonthlyUSDT.Sign() < 0 {
			return fmt.Errorf("invalid interest budget monthlyUSDT %s, expected a positive number, or 0 for no budget", c.Budget.MonthlyUSDT.String())
		}

		for _, asset := range sortedKeys(c.Budget.Assets) {
			if budget := c.Budget.Assets[asset]; budget.Sign() < 0 {
				return fmt.Errorf("invalid interest budget %s of asset %s, expected a positive number, or 0 for no budget", budget.String(), asset)
			}
		}
	}

	// a borrow limit needs a budget to limit to: do not ignore it silently
	if c.limitsBorrow() && !c.hasBudget() {
		return fmt.Errorf("interest budget limitBorrow is set but no budget is, set monthlyUSDT or assets")
	}

	// a budget needs the estimate: do not ignore it silently
	if c.hasBudget() && c.Interval.Duration() <= 0 {
		return fmt.Errorf("interest budget is set but the interest estimate is off, set interestEstimate.interval")
	}

	for _, n := range c.ProjectionMonths {
		if n <= 0 {
			return fmt.Errorf("invalid projection months %d, expected a positive number", n)
		}
	}

	return nil
}

// projectionHours returns the sorted, distinct hours of the rolling-month projections
func (c *InterestEstimateConfig) projectionHours() []int {
	months := c.ProjectionMonths
	if months == nil {
		months = defaultProjectionMonths
	}

	set := map[int]struct{}{}
	for _, n := range months {
		if h := margininterest.RollingMonthsHours(n); h > 0 {
			set[h] = struct{}{}
		}
	}

	hours := make([]int, 0, len(set))
	for h := range set {
		hours = append(hours, h)
	}
	sort.Ints(hours)
	return hours
}

// hasBudget reports whether a session budget or an asset budget is set
func (c *InterestEstimateConfig) hasBudget() bool {
	_, session := c.sessionBudget()
	return session || len(c.assetBudgets()) > 0
}

// limitsBorrow reports whether the budgets limit what autoborrow borrows
func (c *InterestEstimateConfig) limitsBorrow() bool {
	return c.Budget != nil && c.Budget.LimitBorrow
}

// sessionBudget returns the budget of the whole session. ok is false without
// one: no budget is not a zero budget.
func (c *InterestEstimateConfig) sessionBudget() (budget fixedpoint.Value, ok bool) {
	if c.Budget == nil || c.Budget.MonthlyUSDT.Sign() <= 0 {
		return fixedpoint.Zero, false
	}
	return c.Budget.MonthlyUSDT, true
}

// assetBudgets returns the assets that have a budget, with it
func (c *InterestEstimateConfig) assetBudgets() map[string]fixedpoint.Value {
	if c.Budget == nil {
		return nil
	}

	out := make(map[string]fixedpoint.Value, len(c.Budget.Assets))
	for asset, budget := range c.Budget.Assets {
		if budget.Sign() > 0 {
			out[asset] = budget
		}
	}
	return out
}

// budgetAssetsOutside returns the assets with a budget that are not in assets,
// sorted. Such a budget is valid, because the asset list can change while the
// budget stays.
func (c *InterestEstimateConfig) budgetAssetsOutside(assets []string) []string {
	listed := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		listed[asset] = struct{}{}
	}

	var out []string
	for _, asset := range sortedKeys(c.assetBudgets()) {
		if _, ok := listed[asset]; !ok {
			out = append(out, asset)
		}
	}
	return out
}

func (c *InterestEstimateConfig) syncInterval() time.Duration {
	if d := c.SyncInterval.Duration(); d > 0 {
		return d
	}
	return defaultInterestSyncInterval
}

// marginHistoryStore syncs the raw interest rows, values them in USDT and
// queries the valued rows
type marginHistoryStore interface {
	SyncInterests(ctx context.Context, ex types.Exchange, session, asset string, startTime time.Time) error
	ValueInterests(ctx context.Context, ex types.Exchange, session string, since time.Time) error
	QueryValuations(ctx context.Context, opts service.QueryMarginInterestValuationOptions) ([]types.MarginInterestValuation, error)
}

// marginHistory is the marginHistoryStore of the environment: margin_interests
// holds the rows as the exchange returns them, margin_interest_valuations their
// USDT value at charge time
type marginHistory struct {
	margin     *service.MarginService
	valuations *service.MarginInterestValuationService
}

func (h *marginHistory) SyncInterests(ctx context.Context, ex types.Exchange, session, asset string, startTime time.Time) error {
	return h.margin.SyncInterests(ctx, ex, session, asset, startTime)
}

func (h *marginHistory) ValueInterests(ctx context.Context, ex types.Exchange, session string, since time.Time) error {
	return h.valuations.Sync(ctx, ex, session, since)
}

func (h *marginHistory) QueryValuations(
	ctx context.Context, opts service.QueryMarginInterestValuationOptions,
) ([]types.MarginInterestValuation, error) {
	return h.valuations.Query(ctx, opts)
}

// sessionSyncLocks keeps two autoborrow instances from syncing the same session at once.
// It does not take Environment.syncMutex: the startup sync is done before strategies run.
var sessionSyncLocks = struct {
	sync.Mutex
	m map[string]*sync.Mutex
}{m: make(map[string]*sync.Mutex)}

func sessionSyncLock(key string) *sync.Mutex {
	sessionSyncLocks.Lock()
	defer sessionSyncLocks.Unlock()

	mu, ok := sessionSyncLocks.m[key]
	if !ok {
		mu = &sync.Mutex{}
		sessionSyncLocks.m[key] = mu
	}
	return mu
}

// budgetedSessions holds the sessions that have an interest budget, with the
// limiter of the budget, nil when the budget does not limit borrowing. A
// session has one bill, so it has at most one budget.
var budgetedSessions = struct {
	sync.Mutex
	m map[string]*interestBudgetLimiter
}{m: make(map[string]*interestBudgetLimiter)}

func budgetedSessionKey(session *bbgo.ExchangeSession) string {
	return session.ExchangeName.String() + "/" + session.Name
}

// claimInterestBudget makes session the owner of its interest budget and
// returns the function that gives it up. It fails when another autoborrow
// instance already budgets the session: two budgets on one bill cannot both be right.
// limiter is the borrow limit of the budget, nil without one.
func claimInterestBudget(session *bbgo.ExchangeSession, limiter *interestBudgetLimiter) (release func(), err error) {
	key := budgetedSessionKey(session)

	budgetedSessions.Lock()
	defer budgetedSessions.Unlock()

	if _, taken := budgetedSessions.m[key]; taken {
		return nil, fmt.Errorf("session %s already has an interest budget, set by another autoborrow instance: a session has at most one", session.Name)
	}

	budgetedSessions.m[key] = limiter
	return func() {
		budgetedSessions.Lock()
		defer budgetedSessions.Unlock()
		delete(budgetedSessions.m, key)
	}, nil
}

// interestEstimate is the result of one estimate tick
type interestEstimate struct {
	// Time is when the estimate was made
	Time time.Time

	// Month, RemainingHours, Projection, Realized and Assets are the calendar month
	Month          time.Time
	RemainingHours int
	Projection     margininterest.Projection
	Realized       margininterest.Realized
	RealizedOK     bool
	MonthTotalUSDT fixedpoint.Value
	Complete       bool
	SyncFailed     []string

	// Unattributed is the number of realized rows whose borrowed asset is
	// unknown, e.g. a converted row without rawAsset. They are not counted and
	// make the estimate incomplete.
	Unattributed int

	// MissingHistory reports whether the realized interest may be incomplete:
	// it could not be read, a sync or the valuation failed, an asset was not
	// synced yet, or a row could not be attributed
	MissingHistory bool

	// StalePrice are the assets whose current price could not be queried on this
	// tick: what is valued at it uses the last price the session has.
	// StaleDebtPrice are the ones with debt, whose projection that is. Either
	// makes the estimate incomplete.
	StalePrice     []string
	StaleDebtPrice []string

	// Assets is the month interest of each asset, the input of a per-asset budget
	Assets map[string]margininterest.AssetMonth

	// Rolling are the projections over the next n x 30 x 24 hours, by ascending hours.
	// They have no realized part.
	Rolling []margininterest.Projection

	// RollingComplete reports whether every rolling projection is complete: every
	// borrowed asset had a rate and a price. Each one is checked on its own hours,
	// because a zero-hour calendar projection needs no price.
	RollingComplete bool

	// Budget compares the calendar month against the budget of the session, nil
	// without a session budget. Its realized part is always at the price when charged,
	// so in the current price mode its figures differ from Realized and MonthTotalUSDT.
	Budget *margininterest.BudgetEvaluation

	// AssetBudgets compares the calendar month of each asset that has a budget
	// against it, in USDT and at the price when charged, like Budget
	AssetBudgets map[string]margininterest.BudgetEvaluation
}

// interestEstimateWorker estimates the margin interest of one session.
// Run creates one worker per session, so no per-session state lives on Strategy:
// with `on: [a, b]` the same *Strategy runs on both sessions.
type interestEstimateWorker struct {
	session *bbgo.ExchangeSession
	config  *InterestEstimateConfig

	// configAssets returns the autoborrow assets, sorted and distinct. It is called
	// on every tick, like the borrow loop reads them, so a list that is changed
	// while bbgo runs is followed.
	configAssets func() []string

	// covered are the assets of the estimate in coveredMonth: the autoborrow
	// assets, the ones with a budget, and the ones seen with debt or with interest
	// rows. An asset stays covered, and synced, until the month ends: the last
	// charges of an asset repaid or taken off the list are still counted.
	covered      map[string]struct{}
	coveredMonth time.Time

	// debtAssets are the assets with debt on the last tick. They are charged at
	// the next hour, so they start the next month covered.
	debtAssets []string

	// feeAsset is the asset interest can be paid in besides the borrowed one, empty
	// if none (BNB on Binance). It is always synced: interest paid with BNB
	// deduction is stored under BNB, and the exchange returns it only for
	// asset=BNB, not for the borrowed asset.
	feeAsset string

	// interestQueryService queries the next hourly interest rates; nil when the exchange has none
	interestQueryService marginFutureInterestQueryService
	history              marginHistoryStore
	pricer               margininterest.PriceFunc

	// queryPrice queries the current price of asset into what pricer reads, and
	// fails when it could not. The session only follows the prices of the markets
	// a strategy subscribed to at startup, and the estimate covers assets nobody
	// subscribed to.
	queryPrice func(ctx context.Context, asset string) error
	logger     logrus.FieldLogger
	now        func() time.Time

	// syncSince is where the interest sync starts when the session has no rows of
	// an asset yet. It is the start of the full margin history sync, not the month
	// start: the full sync resumes from the newest row, so month-only rows would
	// make it skip the months before.
	syncSince time.Time

	lastSync time.Time

	// syncFailed is the error of the latest sync of an asset, cleared by its next successful sync
	syncFailed map[string]error

	// valuationFailed says the last valuation pass failed: rows that were synced
	// may have no value yet, and those are not read
	valuationFailed bool

	// priceFailed are the assets whose price query failed on this tick
	priceFailed map[string]struct{}

	// budgetOutside is the last reported list of budgeted assets that are not
	// autoborrow assets, joined: it is reported when it changes, not on every tick
	budgetOutside string

	// synced maps an asset to the time its last successful sync started. Every row
	// before it is stored, so the next sync starts from there (less syncedOverlap)
	// instead of scanning back to syncSince again: an asset that was never
	// borrowed has no row to resume from.
	synced map[string]time.Time

	// labels holds the label values (after session and exchange) each vec was set
	// with on the last tick, keyed by their joined form
	labels map[*prometheus.GaugeVec]map[string][]string

	// budgetStatus holds the budget statuses of the last tick, kept in memory
	// only to log a status change once instead of on every tick. The key is the
	// asset, or sessionBudgetKey for the budget of the session.
	budgetStatus map[string]margininterest.BudgetStatus

	// budgetAlerter raises the interest budget alert, nil when interestBudgetAlert is not set
	budgetAlerter *interestBudgetAlerter

	// limiter limits what autoborrow borrows to the budgets, nil unless
	// budget.limitBorrow is set. It is given the figures of every tick.
	limiter *interestBudgetLimiter
}

// sessionBudgetKey is the key of the session budget in budgetStatus; no asset has an empty name
const sessionBudgetKey = ""

func newInterestEstimateWorker(s *Strategy, session *bbgo.ExchangeSession, config *InterestEstimateConfig) *interestEstimateWorker {
	w := &interestEstimateWorker{
		session:    session,
		config:     config,
		logger:     log.WithFields(logrus.Fields{"session": session.Name, "component": "interest_estimate"}),
		now:        time.Now,
		covered:    make(map[string]struct{}),
		syncFailed: make(map[string]error),
		synced:     make(map[string]time.Time),
		labels:     make(map[*prometheus.GaugeVec]map[string][]string),

		budgetStatus: make(map[string]margininterest.BudgetStatus),
	}

	w.configAssets = func() []string { return sortedDistinct(s.getAssetStringSlice()) }

	if s.InterestBudgetAlert != nil && config.hasBudget() {
		w.budgetAlerter = newInterestBudgetAlerter(session, s.InterestBudgetAlert)
		w.budgetAlerter.limitsBorrow = config.limitsBorrow()
	}

	w.feeAsset = service.MarginInterestFeeAsset(session.ExchangeName)

	if service, ok := session.Exchange.(marginFutureInterestQueryService); ok {
		w.interestQueryService = service
	}

	// do not put a marginHistory with nil services into the interface
	if s.Environment != nil && s.Environment.MarginService != nil && s.Environment.MarginInterestValuationService != nil {
		w.history = &marginHistory{
			margin:     s.Environment.MarginService,
			valuations: s.Environment.MarginInterestValuationService,
		}
		w.syncSince = s.Environment.SyncStartTime()
	}

	if err := service.ValidateMarginSessionName(session.Name); err != nil {
		w.logger.WithError(err).Errorf("interest estimate: realized interest is disabled")
		w.history = nil
	}

	w.pricer = func(asset string) (fixedpoint.Value, bool) {
		return session.GetPriceSolver().ResolvePrice(asset, currency.USDT)
	}

	if config.limitsBorrow() {
		w.limiter = newInterestBudgetLimiter(config, w.pricer)
	}

	// Not SimplePriceSolver.UpdateFromTickers: it skips a market it does not know
	// and a ticker without a price silently, and the worker has to know when a
	// price was not refreshed.
	w.queryPrice = func(ctx context.Context, asset string) error {
		market, ok := session.Markets().FindPair(asset, currency.USDT)
		if !ok {
			return fmt.Errorf("%s has no %s market", asset, currency.USDT)
		}

		ticker, err := session.Exchange.QueryTicker(ctx, market.Symbol)
		if err != nil {
			return err
		}

		price := ticker.GetValidPrice()
		if price.IsZero() {
			return fmt.Errorf("the %s ticker has no price", market.Symbol)
		}

		session.GetPriceSolver().Update(market.Symbol, price)
		return nil
	}

	return w
}

// disabledReason says why the estimate cannot run on the session, empty if it can
func (w *interestEstimateWorker) disabledReason() string {
	if w.session.IsolatedMargin {
		return "isolated margin is not supported"
	}

	return ""
}

// claimBudget claims the interest budget of the session, if one is set, and
// returns the function that gives it up. It fails when the estimate cannot run
// on the session, because a budget must not be ignored silently, and when
// another autoborrow instance already budgets the session.
func (w *interestEstimateWorker) claimBudget() (release func(), err error) {
	if !w.config.hasBudget() {
		return func() {}, nil
	}

	if reason := w.disabledReason(); reason != "" {
		return nil, fmt.Errorf("interest budget is set on session %s but the interest estimate cannot run: %s", w.session.Name, reason)
	}

	if unknown := w.unknownBudgetAssets(); len(unknown) > 0 {
		return nil, fmt.Errorf("interest budget is set on session %s for %v, which are not assets of %s: check the names", w.session.Name, unknown, w.session.ExchangeName)
	}

	if w.limiter != nil {
		if reason := w.neverCompleteReason(); reason != "" {
			return nil, fmt.Errorf("interest budget limitBorrow is set on session %s but %s: the budgets would stay unknown and nothing would be borrowed", w.session.Name, reason)
		}
	}

	return claimInterestBudget(w.session, w.limiter)
}

// neverCompleteReason says why the estimate of the session can never be
// complete, empty if it can. A budget on such an estimate is never ok, which a
// budget that only reports says on every tick. One that limits borrowing would
// silently stop it, so it must not start.
func (w *interestEstimateWorker) neverCompleteReason() string {
	switch {
	case w.interestQueryService == nil:
		return fmt.Sprintf("exchange %s has no hourly interest rate API", w.session.ExchangeName)
	case w.history == nil:
		return "the interest history cannot be read (it needs a configured database and a valid session name)"
	}
	return ""
}

// unknownBudgetAssets returns the assets with a budget that no market of the
// session trades. Such a budget would never see a debt or a charge and would
// read ok forever, so a misspelled name must not start. A session without
// markets cannot tell, and refuses nothing.
func (w *interestEstimateWorker) unknownBudgetAssets() []string {
	markets := w.session.Markets()
	if len(markets) == 0 {
		return nil
	}

	currencies := make([]string, 0, 2*len(markets))
	for _, market := range markets {
		currencies = append(currencies, market.BaseCurrency, market.QuoteCurrency)
	}
	return w.config.budgetAssetsOutside(currencies)
}

func (w *interestEstimateWorker) Run(ctx context.Context) {
	if reason := w.disabledReason(); reason != "" {
		w.logger.Warnf("interest estimate: %s, worker disabled", reason)
		return
	}

	if w.interestQueryService == nil {
		w.logger.Warnf("interest estimate: exchange %s has no hourly interest rate API, projection is disabled", w.session.ExchangeName)
	}

	if w.history == nil {
		w.logger.Infof("interest estimate: no database configured, realized interest is disabled")
	}

	interval := w.config.Interval.Duration()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	w.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

// reportBudgetAssetsOutside warns about the budgeted assets that are not
// autoborrow assets: such a budget is valid, but worth knowing about. It is
// said once, and again when the list changes, not on every tick.
func (w *interestEstimateWorker) reportBudgetAssetsOutside(configured []string) {
	outside := w.config.budgetAssetsOutside(configured)
	key := strings.Join(outside, ",")
	if key == w.budgetOutside {
		return
	}

	w.budgetOutside = key
	if len(outside) > 0 {
		w.logger.Warnf("interest budget: %v have a budget but are not autoborrow assets: such a budget only counts the debt and the interest the session has in the asset", outside)
	}
}

func (w *interestEstimateWorker) countError(kind string) {
	interestEstimateErrorsMetrics.WithLabelValues(w.session.Name, w.session.ExchangeName.String(), kind).Inc()
}

func (w *interestEstimateWorker) tick(ctx context.Context) *interestEstimate {
	now := w.now().UTC()
	month := margininterest.MonthRange(now)
	// realized is [month start, floor(now)+1h), projected is the hours after floor(now)
	realizedRange, hours := month.SplitAt(now)

	debts := w.loadDebts(ctx)
	borrowed := sortedKeys(debts)
	configured := w.configAssets()
	w.reportBudgetAssetsOutside(configured)

	w.startMonth(month.Start)
	w.cover(configured)
	w.cover(sortedKeys(w.config.assetBudgets()))
	w.cover(borrowed)
	w.debtAssets = borrowed

	// with a borrow limit, a borrow is valued before its asset has any debt
	borrowable := w.borrowableAssets(configured)
	w.refreshPrices(ctx, w.assetsToPrice(borrowed, borrowable))

	realized, charged, realizedOK, unattributed := w.queryRealized(ctx, now, realizedRange, w.coveredAssets())

	// an asset that only has rows, e.g. synced by `bbgo sync`: it is counted now
	// and synced from the next tick
	w.cover(sortedKeys(realized.Assets))
	assets := w.coveredAssets()

	// a rate is needed to project a debt and to value a borrow, and is reported
	// for every autoborrow asset
	rates := w.queryRates(ctx, unionSorted(unionSorted(configured, borrowable), borrowed))
	projection := margininterest.ProjectAssets(assets, debts, rates, w.pricer, hours)
	for range projection.MissingPrice {
		w.countError("price")
	}

	var rolling []margininterest.Projection
	for _, h := range w.config.projectionHours() {
		rolling = append(rolling, margininterest.ProjectAssets(assets, debts, rates, w.pricer, h))
	}

	est := &interestEstimate{
		Time:           now,
		Month:          month.Start,
		RemainingHours: hours,
		Projection:     projection,
		Realized:       realized,
		RealizedOK:     realizedOK,
		MonthTotalUSDT: realized.TotalUSDT.Add(projection.TotalUSDT),
		Rolling:        rolling,
		Unattributed:   unattributed,
		StalePrice:     sortedKeys(w.priceFailed),
	}

	for _, asset := range est.StalePrice {
		if _, ok := debts[asset]; ok {
			est.StaleDebtPrice = append(est.StaleDebtPrice, asset)
		}
	}
	est.RollingComplete = w.interestQueryService != nil && allComplete(rolling) && len(est.StaleDebtPrice) == 0

	for asset := range w.syncFailed {
		est.SyncFailed = append(est.SyncFailed, asset)
	}
	sort.Strings(est.SyncFailed)

	est.MissingHistory = w.sessionHistoryMissing(est) || len(w.syncFailed) > 0
	// every asset of the estimate is listed, borrowed or not
	est.Assets = margininterest.MonthByAsset(projection, realized)
	for asset, m := range est.Assets {
		m.MissingHistory = w.assetHistoryMissing(est, asset)
		if m.MissingHistory {
			est.MissingHistory = true
		}
		_, m.StalePrice = w.priceFailed[asset]
		est.Assets[asset] = m
	}

	// realized is valued at the current price in the current price mode, so there
	// every stale price counts
	est.Complete = w.complete(est, realized) && len(est.StalePrice) == 0

	w.evaluateBudgets(est, charged)
	if w.limiter != nil {
		w.limiter.set(w.budgetBasis(est, charged, rates))
	}

	w.log(est, rates)
	w.publish(est)
	if w.budgetAlerter != nil {
		w.budgetAlerter.update(est)
	}
	return est
}

// complete reports whether est has every rate, every price and every row, with
// the realized interest valued as realized is
func (w *interestEstimateWorker) complete(est *interestEstimate, realized margininterest.Realized) bool {
	return w.interestQueryService != nil && est.Projection.Complete() && !est.MissingHistory && len(realized.MissingPrice) == 0
}

// evaluateBudgets compares the month against the budget of the session and
// the budgets of the assets. Every budget compares charged: what was charged,
// at the price when it was charged, in every price mode.
func (w *interestEstimateWorker) evaluateBudgets(est *interestEstimate, charged margininterest.Realized) {
	projection := est.Projection

	if budget, ok := w.config.sessionBudget(); ok {
		c := chargedOfSession(est, charged)
		// charged is valued when charged: only the price of a debt can be stale
		complete := w.interestQueryService != nil && projection.Complete() && c.Complete && len(est.StaleDebtPrice) == 0
		e := margininterest.EvaluateBudget(budget, c.USDT, c.USDT.Add(projection.TotalUSDT), complete)
		est.Budget = &e
	}

	budgets := w.config.assetBudgets()
	if len(budgets) == 0 {
		return
	}

	est.AssetBudgets = make(map[string]margininterest.BudgetEvaluation, len(budgets))
	for asset, budget := range budgets {
		c := w.chargedOfAsset(est, charged, asset)

		// an asset with principal but no rate is not projected at all
		projectedUSDT := fixedpoint.Zero
		ap, projected := projection.Assets[asset]
		if projected && ap.PriceOK {
			projectedUSDT = ap.USDT
		}

		_, stalePrice := w.priceFailed[asset]
		complete := w.interestQueryService != nil && projected && ap.PriceOK && !stalePrice && c.Complete

		est.AssetBudgets[asset] = margininterest.EvaluateBudget(budget, c.USDT, c.USDT.Add(projectedUSDT), complete)
	}
}

// chargedOfSession returns what the session was charged this month, and
// whether that is every charge, each with its value
func chargedOfSession(est *interestEstimate, charged margininterest.Realized) chargedInterest {
	return chargedInterest{
		USDT:     charged.TotalUSDT,
		Complete: !est.MissingHistory && len(charged.MissingPrice) == 0,
	}
}

// chargedOfAsset returns what the session was charged this month for asset,
// and whether that is every charge, each with its value
func (w *interestEstimateWorker) chargedOfAsset(est *interestEstimate, charged margininterest.Realized, asset string) chargedInterest {
	usdt, priceOK := fixedpoint.Zero, true
	if ar, ok := charged.Assets[asset]; ok {
		usdt, priceOK = ar.USDT, ar.PriceOK
	}

	return chargedInterest{USDT: usdt, Complete: priceOK && !w.assetHistoryMissing(est, asset)}
}

// budgetBasis returns what the tick of est knows about the budgets, for the
// borrow limit: what each budget was charged, and the rates and the remaining
// hours to project a debt with. The debts are not in it, the limit reads them
// when a borrow is about to be made.
func (w *interestEstimateWorker) budgetBasis(
	est *interestEstimate, charged margininterest.Realized, rates types.MarginNextHourlyInterestRateMap,
) *interestBudgetBasis {
	basis := &interestBudgetBasis{
		Month:      est.Month,
		Hours:      est.RemainingHours,
		SyncedAt:   w.lastSync,
		Rates:      rates,
		StalePrice: make(map[string]struct{}, len(w.priceFailed)),
		Assets:     make(map[string]chargedInterest),
	}

	// a copy: the borrow loop reads the basis while the worker goes on
	for asset := range w.priceFailed {
		basis.StalePrice[asset] = struct{}{}
	}

	if _, ok := w.config.sessionBudget(); ok {
		c := chargedOfSession(est, charged)
		basis.Session = &c
	}

	for asset := range w.config.assetBudgets() {
		basis.Assets[asset] = w.chargedOfAsset(est, charged, asset)
	}

	return basis
}

func (w *interestEstimateWorker) loadDebts(ctx context.Context) types.BalanceMap {
	account, err := w.session.UpdateAccount(ctx)
	if err != nil {
		w.logger.WithError(err).Warnf("interest estimate: unable to update account, using the cached account")
		w.countError("account")
		account = w.session.GetAccount()
	}

	if account == nil {
		return types.BalanceMap{}
	}

	return account.Balances().Debts()
}

// startMonth forgets the assets covered in an earlier month. The ones that had
// debt on the last tick stay: they are charged at the first hour of the new
// month, also when they are repaid before its first tick.
func (w *interestEstimateWorker) startMonth(monthStart time.Time) {
	if monthStart.Equal(w.coveredMonth) {
		return
	}

	w.covered = make(map[string]struct{})
	w.coveredMonth = monthStart
	w.cover(w.debtAssets)
}

// cover adds assets to the assets of the estimate, until the month ends
func (w *interestEstimateWorker) cover(assets []string) {
	for _, asset := range assets {
		w.covered[asset] = struct{}{}
	}
}

// coveredAssets returns the assets of the estimate, sorted
func (w *interestEstimateWorker) coveredAssets() []string {
	return sortedKeys(w.covered)
}

// queryRates queries the next hourly rate of assets
func (w *interestEstimateWorker) queryRates(ctx context.Context, assets []string) types.MarginNextHourlyInterestRateMap {
	rates := types.MarginNextHourlyInterestRateMap{}
	if w.interestQueryService == nil {
		return rates
	}

	for start := 0; start < len(assets); start += maxRateAssetsPerQuery {
		end := min(start+maxRateAssetsPerQuery, len(assets))
		chunk := assets[start:end]

		result, err := w.interestQueryService.QueryMarginFutureHourlyInterestRate(ctx, chunk)
		if err != nil {
			w.logger.WithError(err).Warnf("interest estimate: unable to query hourly interest rates of %v", chunk)
			w.countError("rate")
			continue
		}

		for asset, rate := range result {
			rates[asset] = rate
		}
	}

	return rates
}

// borrowableAssets returns the assets a borrow limit has to value a borrow of,
// nil without one: the autoborrow assets of this instance, and the assets a
// limit was asked for. Another autoborrow instance on the session borrows under
// the same budgets, and its assets are only known once it asks. Its first
// borrow of an asset is refused as unknown, and the next tick has the rate and
// the price.
func (w *interestEstimateWorker) borrowableAssets(configured []string) []string {
	if w.limiter == nil {
		return nil
	}
	return unionSorted(configured, w.limiter.askedAssets())
}

// assetsToPrice returns the assets the estimate needs the current price of: the
// borrowed ones, to value their projection, and the borrowable ones of a
// borrow limit. In the current price mode also every covered asset and the fee
// asset, to value what was charged and what it was paid in.
func (w *interestEstimateWorker) assetsToPrice(borrowed, borrowable []string) []string {
	assets := unionSorted(borrowed, borrowable)

	if w.config.RealizedPriceMode.OrDefault() != margininterest.PriceModeCurrent {
		return assets
	}

	assets = unionSorted(assets, w.coveredAssets())
	if w.feeAsset != "" {
		assets = unionSorted(assets, []string{w.feeAsset})
	}
	return assets
}

// refreshPrices queries the current price of assets. A price that cannot be
// queried keeps its last value, or stays missing, and its asset is in
// priceFailed until a tick queries it: an old price must not pass for a current one.
func (w *interestEstimateWorker) refreshPrices(ctx context.Context, assets []string) {
	// one slow ticker must not hold the tick back; the queries after the timeout fail
	pctx, cancel := context.WithTimeout(ctx, priceQueryTimeout)
	defer cancel()

	w.priceFailed = make(map[string]struct{})
	for _, asset := range assets {
		if asset == currency.USDT {
			continue
		}

		if err := w.queryPrice(pctx, asset); err != nil {
			w.logger.WithError(err).Warnf("interest estimate: unable to query the price of %s", asset)
			w.countError("price_query")
			w.priceFailed[asset] = struct{}{}
		}
	}
}

// queryRealized syncs the interest history of assets when due, values the rows
// that have no value yet, and sums every row of the session in the hours r, by
// borrowed asset: a charge counts whichever asset it is for, so the month total
// never loses what was already charged. realized is valued in the configured price mode, charged always at
// the price when charged: it is what the budget compares. unattributed counts
// the rows whose borrowed asset is unknown.
func (w *interestEstimateWorker) queryRealized(
	ctx context.Context, now time.Time, r margininterest.HourRange, assets []string,
) (realized, charged margininterest.Realized, ok bool, unattributed int) {
	mode := w.config.RealizedPriceMode.OrDefault()
	empty := margininterest.Realized{Mode: mode, TotalUSDT: fixedpoint.Zero}
	emptyCharged := margininterest.Realized{Mode: margininterest.PriceModeCharged, TotalUSDT: fixedpoint.Zero}
	if w.history == nil {
		return empty, emptyCharged, false, 0
	}

	w.syncDueAssets(ctx, now, r.Start, assets)
	w.valueHistory(ctx, r.Start)

	// r ends after the current hour, so the charge at floor(now) and a borrow-time
	// row stamped inside the current hour are realized

	rows, err := w.history.QueryValuations(ctx, service.QueryMarginInterestValuationOptions{
		Exchange: w.session.ExchangeName,
		Session:  w.session.Name,
		Since:    r.Start,
		Until:    r.End,
	})
	if err != nil {
		w.logger.WithError(err).Warnf("interest estimate: unable to query margin interest valuations")
		w.countError("query")
		return empty, emptyCharged, false, 0
	}

	// a row counts toward the asset it was borrowed in, also when paid in BNB
	counted := rows[:0:0]
	for _, row := range rows {
		if row.BorrowedAsset == "" || row.BorrowedInterest == nil {
			unattributed++
			continue
		}

		counted = append(counted, row)
	}

	if unattributed > 0 {
		w.logger.Warnf("interest estimate: %d margin interest rows have no borrowed asset, they are not counted", unattributed)
		w.countError("unattributed")
	}

	realized = margininterest.SumRealized(counted, mode, w.pricer)
	charged = realized
	if mode != margininterest.PriceModeCharged {
		charged = margininterest.SumRealized(counted, margininterest.PriceModeCharged, w.pricer)
	}

	return realized, charged, true, unattributed
}

// syncDueAssets syncs the interest rows of assets and of the fee asset that
// are due. Every syncInterval all of them are. Between two of those, only the
// ones that were never synced or whose last sync failed are: an asset that was
// just borrowed is counted on the tick it shows up, and a failed sync is tried
// again on the next tick instead of an interval later.
func (w *interestEstimateWorker) syncDueAssets(ctx context.Context, now, rangeStart time.Time, assets []string) {
	if w.feeAsset != "" {
		assets = unionSorted(assets, []string{w.feeAsset})
	}

	// An asset leaves the estimate when a month starts. Its failure would never
	// be cleared, and if it comes back it has rows missing since its last sync:
	// it is new again.
	wanted := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		wanted[asset] = struct{}{}
	}
	keepOnly(w.syncFailed, wanted)
	keepOnly(w.synced, wanted)

	due := w.lastSync.IsZero() || now.Sub(w.lastSync) >= w.config.syncInterval()
	if !due {
		var pending []string
		for _, asset := range assets {
			if w.unsynced(asset) {
				pending = append(pending, asset)
			}
		}
		assets = pending
	}

	if len(assets) == 0 {
		return
	}

	w.syncHistory(ctx, now, rangeStart, assets)
	if due {
		w.lastSync = now
	}
}

// unsynced reports whether the rows of asset may be missing: it was never
// synced by this worker, or its last sync failed
func (w *interestEstimateWorker) unsynced(asset string) bool {
	if _, failed := w.syncFailed[asset]; failed {
		return true
	}
	_, synced := w.synced[asset]
	return !synced
}

// syncHistory syncs the interest rows of assets, borrowed or not, so the
// realized range starting at rangeStart is covered. A charge of an asset
// borrowed and repaid between two syncs is synced too.
func (w *interestEstimateWorker) syncHistory(ctx context.Context, now, rangeStart time.Time, assets []string) {
	defer w.lockSync()()

	// rows inserted before the timeout are kept, and the next sync resumes from them
	sctx, cancel := context.WithTimeout(ctx, w.config.Interval.Duration())
	defer cancel()

	for _, asset := range assets {
		if err := w.history.SyncInterests(sctx, w.session.Exchange, w.session.Name, asset, w.assetSyncStart(asset, rangeStart)); err != nil {
			w.logger.WithError(err).Warnf("interest estimate: unable to sync %s margin interests", asset)
			w.countError("sync")
			w.syncFailed[asset] = err
			continue
		}

		delete(w.syncFailed, asset)
		w.synced[asset] = now
	}
}

// valueHistory values the rows of the realized range that have no value yet.
// It runs on every tick, with its own timeout: it is one query when nothing is
// pending, a sync that used its whole timeout does not starve it, and a row
// whose kline was not available is tried again on the next tick. A row that
// cannot be priced is stored unpriced and marks its asset's price missing.
func (w *interestEstimateWorker) valueHistory(ctx context.Context, rangeStart time.Time) {
	defer w.lockSync()()

	vctx, cancel := context.WithTimeout(ctx, valuationTimeout)
	defer cancel()

	if err := w.history.ValueInterests(vctx, w.session.Exchange, w.session.Name, rangeStart); err != nil {
		w.logger.WithError(err).Warnf("interest estimate: unable to value margin interests")
		w.countError("value")
		w.valuationFailed = true
		return
	}

	w.valuationFailed = false
}

// lockSync keeps two autoborrow instances from syncing or valuing the same
// session at once, and returns the unlock
func (w *interestEstimateWorker) lockSync() (unlock func()) {
	mu := sessionSyncLock(w.session.ExchangeName.String() + "/" + w.session.Name)
	mu.Lock()
	return mu.Unlock
}

// sessionHistoryMissing reports whether charges may be missing from the
// realized interest of est whichever asset they are for: the rows could not be
// read or valued, or one could not be attributed and may belong to any asset
func (w *interestEstimateWorker) sessionHistoryMissing(est *interestEstimate) bool {
	return !est.RealizedOK || est.Unattributed > 0 || w.valuationFailed
}

// assetHistoryMissing reports whether the realized interest of asset in est may
// be incomplete: charges may be missing for every asset, or the asset, or the
// fee asset its interest may be paid in, was never synced or its latest sync failed
func (w *interestEstimateWorker) assetHistoryMissing(est *interestEstimate, asset string) bool {
	if w.sessionHistoryMissing(est) || w.unsynced(asset) {
		return true
	}
	return w.feeAsset != "" && w.unsynced(w.feeAsset)
}

// assetSyncStart returns syncStart, or the last successful sync of asset (less
// syncedOverlap) when that is later. A later start leaves no gap: that sync
// stored every row before it.
func (w *interestEstimateWorker) assetSyncStart(asset string, rangeStart time.Time) time.Time {
	start := w.syncStart(rangeStart)
	if last, ok := w.synced[asset]; ok {
		if resume := last.Add(-syncedOverlap); resume.After(start) {
			return resume
		}
	}
	return start
}

// syncStart returns the earlier of syncSince and the start of the realized
// range, so the realized range is always covered.
func (w *interestEstimateWorker) syncStart(rangeStart time.Time) time.Time {
	if !w.syncSince.IsZero() && w.syncSince.Before(rangeStart) {
		return w.syncSince
	}
	return rangeStart
}

func (w *interestEstimateWorker) log(est *interestEstimate, rates types.MarginNextHourlyInterestRateMap) {
	missingPrice := unionSorted(est.Projection.MissingPrice, est.Realized.MissingPrice)

	w.logger.Infof("interest estimate: session=%s month=%s remaining_hours=%d projected_usdt=%s realized_usdt=%s realized_mode=%s month_total_usdt=%s complete=%t missing_rate=%v missing_price=%v stale_price=%v missing_history=%t sync_failed=%v unattributed=%d",
		w.session.Name,
		est.Month.Format("2006-01"),
		est.RemainingHours,
		est.Projection.TotalUSDT.String(),
		est.Realized.TotalUSDT.String(),
		est.Realized.Mode,
		est.MonthTotalUSDT.String(),
		est.Complete,
		est.Projection.MissingRate,
		missingPrice,
		est.StalePrice,
		est.MissingHistory,
		est.SyncFailed,
		est.Unattributed,
	)

	for _, p := range est.Rolling {
		w.logger.Infof("interest projection: session=%s hours=%d rolling_months=%d projected_usdt=%s complete=%t",
			w.session.Name, p.Hours, p.Hours/margininterest.RollingMonthHours, p.TotalUSDT.String(), w.rollingComplete(est, p))
	}

	w.logBudget(est)

	for _, asset := range estimateAssets(est) {
		p := est.Projection.Assets[asset]
		m := est.Assets[asset]
		hourlyRate := fixedpoint.Zero
		if rate, ok := rates[asset]; ok && rate != nil {
			hourlyRate = rate.HourlyRate
		}

		w.logger.Debugf("interest estimate asset: session=%s asset=%s principal=%s hourly_rate=%s hours=%d projected=%s projected_usdt=%s realized=%s realized_usdt=%s month_total=%s month_total_usdt=%s complete=%t missing_rate=%t missing_price=%t stale_price=%t missing_history=%t",
			w.session.Name, asset,
			p.Principal.String(), hourlyRate.String(), p.Hours,
			m.Projected.String(), m.ProjectedUSDT.String(),
			m.Realized.String(), m.RealizedUSDT.String(),
			m.Total.String(), m.TotalUSDT.String(),
			m.Complete(), m.MissingRate, m.MissingPrice, m.StalePrice, m.MissingHistory,
		)
	}
}

// logBudget logs the budgets of the tick: the session budget on an info line,
// each asset budget on a debug line, like the estimate. A status change is
// logged once, see logBudgetChange.
func (w *interestEstimateWorker) logBudget(est *interestEstimate) {
	const format = "budget_usdt=%s realized_usdt=%s month_total_usdt=%s remaining_usdt=%s used_ratio=%s status=%s complete=%t"
	fields := func(b margininterest.BudgetEvaluation) []any {
		return []any{
			b.BudgetUSDT.String(), b.RealizedUSDT.String(), b.MonthUSDT.String(),
			b.RemainingUSDT.String(), b.UsedRatio.String(), b.Status(), b.Complete,
		}
	}

	month := est.Month.Format("2006-01")
	if b := est.Budget; b != nil {
		w.logger.Infof("interest budget: session=%s month=%s "+format,
			append([]any{w.session.Name, month}, fields(*b)...)...)
		w.logBudgetChange(sessionBudgetKey, "session "+w.session.Name, *b)
	}

	for _, asset := range sortedKeys(est.AssetBudgets) {
		b := est.AssetBudgets[asset]
		w.logger.Debugf("interest budget asset: session=%s asset=%s month=%s "+format,
			append([]any{w.session.Name, asset, month}, fields(b)...)...)
		w.logBudgetChange(asset, fmt.Sprintf("%s of session %s", asset, w.session.Name), b)
	}
}

// logBudgetChange logs a change of the budget status stored under key: a
// warning when subject is over or may be over its budget, an info line when it
// is ok again. The same status is not logged again on the next tick.
func (w *interestEstimateWorker) logBudgetChange(key, subject string, b margininterest.BudgetEvaluation) {
	status := b.Status()
	previous := w.budgetStatus[key]
	w.budgetStatus[key] = status
	if status == previous {
		return
	}

	// previous is empty on the first tick: starting ok is not a change worth a line
	was := ""
	if previous != "" {
		was = fmt.Sprintf(" (was %s)", previous)
	}

	switch {
	case status != margininterest.BudgetStatusOK:
		w.logger.Warnf("interest budget: %s is %s%s: budget %s USDT, realized %s USDT, month total %s USDT",
			subject, status, was, b.BudgetUSDT.String(), b.RealizedUSDT.String(), b.MonthUSDT.String())
	case previous != "":
		w.logger.Infof("interest budget: %s is %s again%s", subject, status, was)
	}
}

// publishBudget sets the budget gauges. Without a budget they are not set at
// all, so no budget cannot read as a zero budget or as an exceeded one.
func (w *interestEstimateWorker) publishBudget(est *interestEstimate) {
	flag := func(on bool) float64 {
		if on {
			return 1
		}
		return 0
	}

	if b := est.Budget; b != nil {
		session, exchange := w.session.Name, w.session.ExchangeName.String()

		interestBudgetInUSDTMetrics.WithLabelValues(session, exchange).Set(b.BudgetUSDT.Float64())
		interestBudgetRemainingInUSDTMetrics.WithLabelValues(session, exchange).Set(b.RemainingUSDT.Float64())
		interestBudgetUsedRatioMetrics.WithLabelValues(session, exchange).Set(b.UsedRatio.Float64())
		interestBudgetExceededMetrics.WithLabelValues(session, exchange).Set(flag(b.Exceeded))
		interestBudgetProjectedExceededMetrics.WithLabelValues(session, exchange).Set(flag(b.ProjectedExceeded))

		status := b.Status()
		for _, s := range margininterest.BudgetStatuses {
			interestBudgetStatusMetrics.WithLabelValues(session, exchange, string(s)).Set(flag(s == status))
		}
	}

	if len(est.AssetBudgets) == 0 {
		return
	}

	budget := map[string]float64{}
	remaining := map[string]float64{}
	usedRatio := map[string]float64{}
	exceeded := map[string]float64{}
	projectedExceeded := map[string]float64{}
	var statuses []gaugeSample
	for asset, b := range est.AssetBudgets {
		budget[asset] = b.BudgetUSDT.Float64()
		remaining[asset] = b.RemainingUSDT.Float64()
		usedRatio[asset] = b.UsedRatio.Float64()
		exceeded[asset] = flag(b.Exceeded)
		projectedExceeded[asset] = flag(b.ProjectedExceeded)

		status := b.Status()
		for _, s := range margininterest.BudgetStatuses {
			statuses = append(statuses, gaugeSample{[]string{asset, string(s)}, flag(s == status)})
		}
	}

	w.setAssetGauges(interestAssetBudgetInUSDTMetrics, budget)
	w.setAssetGauges(interestAssetBudgetRemainingInUSDTMetrics, remaining)
	w.setAssetGauges(interestAssetBudgetUsedRatioMetrics, usedRatio)
	w.setAssetGauges(interestAssetBudgetExceededMetrics, exceeded)
	w.setAssetGauges(interestAssetBudgetProjectedExceededMetrics, projectedExceeded)
	w.setGauges(interestAssetBudgetStatusMetrics, statuses)
}

func (w *interestEstimateWorker) publish(est *interestEstimate) {
	session, exchange := w.session.Name, w.session.ExchangeName.String()

	projectedUSDT := map[string]float64{}
	projectedAmount := map[string]float64{}
	hourlyRate := map[string]float64{}
	for asset, p := range est.Projection.Assets {
		projectedAmount[asset] = p.Interest.Float64()
		if p.RateOK {
			hourlyRate[asset] = p.HourlyRate.Float64()
		}
		if p.PriceOK {
			projectedUSDT[asset] = p.USDT.Float64()
		}
	}

	realizedUSDT := map[string]float64{}
	if est.RealizedOK {
		for asset, r := range est.Realized.Assets {
			if r.PriceOK {
				realizedUSDT[asset] = r.USDT.Float64()
			}
		}
	}

	realizedAmount := map[string]float64{}
	var realizedCharged []gaugeSample
	if est.RealizedOK {
		for asset, r := range est.Realized.Assets {
			realizedAmount[asset] = r.Interest.Float64()
			for paid, amount := range r.Charged {
				realizedCharged = append(realizedCharged, gaugeSample{[]string{asset, paid}, amount.Float64()})
			}
		}
	}

	// the amount is always published, USDT only when every part has a price;
	// asset_estimate_complete says whether the figures are the full month
	monthAmount := map[string]float64{}
	monthUSDT := map[string]float64{}
	assetComplete := map[string]float64{}
	for asset, m := range est.Assets {
		monthAmount[asset] = m.Total.Float64()
		if !m.MissingPrice {
			monthUSDT[asset] = m.TotalUSDT.Float64()
		}
		assetComplete[asset] = 0
		if m.Complete() {
			assetComplete[asset] = 1
		}
	}

	w.setAssetGauges(interestRealizedAmountMetrics, realizedAmount)
	w.setGauges(interestRealizedChargedAmountMetrics, realizedCharged)
	w.setAssetGauges(interestMonthAmountMetrics, monthAmount)
	w.setAssetGauges(interestMonthInUSDTMetrics, monthUSDT)
	w.setAssetGauges(interestAssetEstimateCompleteMetrics, assetComplete)
	w.setAssetGauges(interestProjectedInUSDTMetrics, projectedUSDT)
	w.setAssetGauges(interestProjectedAmountMetrics, projectedAmount)
	w.setAssetGauges(interestHourlyRateMetrics, hourlyRate)
	w.setAssetGauges(interestRealizedInUSDTMetrics, realizedUSDT)

	complete := 0.0
	if est.Complete {
		complete = 1.0
	}

	interestProjectedTotalInUSDTMetrics.WithLabelValues(session, exchange).Set(est.Projection.TotalUSDT.Float64())
	interestRealizedTotalInUSDTMetrics.WithLabelValues(session, exchange).Set(est.Realized.TotalUSDT.Float64())
	interestMonthTotalInUSDTMetrics.WithLabelValues(session, exchange).Set(est.MonthTotalUSDT.Float64())
	interestRemainingHoursMetrics.WithLabelValues(session, exchange).Set(float64(est.RemainingHours))
	interestEstimateCompleteMetrics.WithLabelValues(session, exchange).Set(complete)

	w.publishRolling(est)
	w.publishBudget(est)
}

// rollingComplete reports whether the rolling projection p of est had a rate
// and a current price for every borrowed asset
func (w *interestEstimateWorker) rollingComplete(est *interestEstimate, p margininterest.Projection) bool {
	return w.interestQueryService != nil && p.Complete() && len(est.StaleDebtPrice) == 0
}

func (w *interestEstimateWorker) publishRolling(est *interestEstimate) {
	var amount, usdt, total, complete []gaugeSample

	for _, p := range est.Rolling {
		hours := strconv.Itoa(p.Hours)
		rollingComplete := 0.0
		if w.rollingComplete(est, p) {
			rollingComplete = 1.0
		}

		total = append(total, gaugeSample{[]string{hours}, p.TotalUSDT.Float64()})
		complete = append(complete, gaugeSample{[]string{hours}, rollingComplete})

		for asset, ap := range p.Assets {
			amount = append(amount, gaugeSample{[]string{asset, hours}, ap.Interest.Float64()})
			if ap.PriceOK {
				usdt = append(usdt, gaugeSample{[]string{asset, hours}, ap.USDT.Float64()})
			}
		}
	}

	w.setGauges(interestRollingProjectedAmountMetrics, amount)
	w.setGauges(interestRollingProjectedInUSDTMetrics, usdt)
	w.setGauges(interestRollingProjectedTotalInUSDTMetrics, total)
	w.setGauges(interestRollingProjectionCompleteMetrics, complete)
}

// gaugeSample is one series of a vec: the label values after session and exchange, and its value
type gaugeSample struct {
	labels []string
	value  float64
}

// setAssetGauges sets the per-asset series of vec, see setGauges.
func (w *interestEstimateWorker) setAssetGauges(vec *prometheus.GaugeVec, values map[string]float64) {
	samples := make([]gaugeSample, 0, len(values))
	for asset, v := range values {
		samples = append(samples, gaugeSample{[]string{asset}, v})
	}
	w.setGauges(vec, samples)
}

// setGauges sets the series of vec and deletes the series set on the last tick
// but not on this one, e.g. a repaid asset or one that lost its price.
func (w *interestEstimateWorker) setGauges(vec *prometheus.GaugeVec, samples []gaugeSample) {
	prefix := []string{w.session.Name, w.session.ExchangeName.String()}
	labels := func(rest []string) []string {
		return append(append([]string(nil), prefix...), rest...)
	}

	current := make(map[string][]string, len(samples))
	for _, s := range samples {
		vec.WithLabelValues(labels(s.labels)...).Set(s.value)
		current[strings.Join(s.labels, "\x00")] = s.labels
	}

	for key, rest := range w.labels[vec] {
		if _, ok := current[key]; !ok {
			vec.DeleteLabelValues(labels(rest)...)
		}
	}

	w.labels[vec] = current
}

func estimateAssets(est *interestEstimate) []string {
	set := map[string]struct{}{}
	for asset := range est.Projection.Assets {
		set[asset] = struct{}{}
	}
	for asset := range est.Realized.Assets {
		set[asset] = struct{}{}
	}
	for asset := range est.Assets {
		set[asset] = struct{}{}
	}

	assets := make([]string, 0, len(set))
	for asset := range set {
		assets = append(assets, asset)
	}
	sort.Strings(assets)
	return assets
}

func allComplete(projections []margininterest.Projection) bool {
	for _, p := range projections {
		if !p.Complete() {
			return false
		}
	}
	return true
}

// sortedKeys returns the keys of m in ascending order
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// keepOnly deletes the keys of m that are not in wanted
func keepOnly[V any](m map[string]V, wanted map[string]struct{}) {
	for key := range m {
		if _, ok := wanted[key]; !ok {
			delete(m, key)
		}
	}
}

func sortedDistinct(a []string) []string {
	return unionSorted(a, nil)
}

func unionSorted(a, b []string) []string {
	set := map[string]struct{}{}
	for _, s := range a {
		set[s] = struct{}{}
	}
	for _, s := range b {
		set[s] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
