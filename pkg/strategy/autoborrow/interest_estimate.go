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

// defaultProjectionMonths are the rolling months (30 days each) projected when
// projectionMonths is not set
var defaultProjectionMonths = []int{1, 3}

// InterestEstimateConfig enables the margin interest estimate of the session.
//
// The estimate covers the assets configured in autoborrow, borrowed or not, so
// every asset is reported and synced even with no debt. The config is the
// asset list by design: a debt or a charge of another asset is not counted, only warned.
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
}

// Validate checks the price mode and the projection months
func (c *InterestEstimateConfig) Validate() error {
	if err := c.RealizedPriceMode.Validate(); err != nil {
		return err
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

func (c *InterestEstimateConfig) syncInterval() time.Duration {
	if d := c.SyncInterval.Duration(); d > 0 {
		return d
	}
	return defaultInterestSyncInterval
}

type marginHistoryStore interface {
	SyncInterests(ctx context.Context, ex types.Exchange, session, asset string, startTime time.Time) error
	QueryInterests(ctx context.Context, opts service.QueryMarginInterestOptions) ([]types.MarginInterest, error)
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

// interestEstimate is the result of one estimate tick
type interestEstimate struct {
	// Month, RemainingHours, Projection, Realized and Assets are the calendar month
	Month          time.Time
	RemainingHours int
	Projection     margininterest.Projection
	Realized       margininterest.Realized
	RealizedOK     bool
	MonthTotalUSDT fixedpoint.Value
	Complete       bool
	SyncFailed     []string

	// Unconfigured are the assets outside the autoborrow assets that have debt or
	// realized rows this month. They are out of scope: warned, not counted, and
	// they do not make the estimate incomplete.
	Unconfigured []string

	// Assets is the month interest of each asset, the input of a per-asset budget
	Assets map[string]margininterest.AssetMonth

	// Rolling are the projections over the next n x 30 x 24 hours, by ascending hours.
	// They have no realized part.
	Rolling []margininterest.Projection

	// RollingComplete reports whether every rolling projection is complete: every
	// borrowed asset had a rate and a price. Each one is checked on its own hours,
	// because a zero-hour calendar projection needs no price.
	RollingComplete bool
}

// interestEstimateWorker estimates the margin interest of one session.
// Run creates one worker per session, so no per-session state lives on Strategy:
// with `on: [a, b]` the same *Strategy runs on both sessions.
type interestEstimateWorker struct {
	session *bbgo.ExchangeSession
	config  *InterestEstimateConfig

	// assets are the autoborrow assets, sorted and distinct: the scope of the
	// estimate and of the interest sync
	assets []string

	rates   marginFutureInterestQueryService
	history marginHistoryStore
	price   margininterest.PriceFunc
	logger  logrus.FieldLogger
	now     func() time.Time

	// syncSince is where the interest sync starts when the session has no rows of
	// an asset yet. It is the start of the full margin history sync, not the month
	// start: the full sync resumes from the newest row, so month-only rows would
	// make it skip the months before.
	syncSince time.Time

	lastSync time.Time

	// syncFailed is the error of the latest sync of an asset, cleared by its next successful sync
	syncFailed map[string]error

	// synced maps an asset to the time its last successful sync started. Every row
	// before it is stored, so the next sync starts from there (less syncedOverlap)
	// instead of scanning back to syncSince again: an asset that was never
	// borrowed has no row to resume from.
	synced map[string]time.Time

	// labels holds the label values (after session and exchange) each vec was set
	// with on the last tick, keyed by their joined form
	labels map[*prometheus.GaugeVec]map[string][]string
}

func newInterestEstimateWorker(s *Strategy, session *bbgo.ExchangeSession, config *InterestEstimateConfig) *interestEstimateWorker {
	w := &interestEstimateWorker{
		session:    session,
		config:     config,
		assets:     sortedDistinct(s.getAssetStringSlice()),
		logger:     log.WithFields(logrus.Fields{"session": session.Name, "component": "interest_estimate"}),
		now:        time.Now,
		syncFailed: make(map[string]error),
		synced:     make(map[string]time.Time),
		labels:     make(map[*prometheus.GaugeVec]map[string][]string),
	}

	if rates, ok := session.Exchange.(marginFutureInterestQueryService); ok {
		w.rates = rates
	}

	// do not put a typed nil *MarginService into the interface
	if s.Environment != nil && s.Environment.MarginService != nil {
		w.history = s.Environment.MarginService
		w.syncSince = s.Environment.SyncStartTime()
	}

	if err := service.ValidateMarginSessionName(session.Name); err != nil {
		w.logger.WithError(err).Errorf("interest estimate: realized interest is disabled")
		w.history = nil
	}

	w.price = func(asset string) (fixedpoint.Value, bool) {
		return session.GetPriceSolver().ResolvePrice(asset, currency.USDT)
	}

	return w
}

func (w *interestEstimateWorker) Run(ctx context.Context) {
	if w.session.IsolatedMargin {
		w.logger.Warnf("interest estimate: isolated margin is not supported, worker disabled")
		return
	}

	if len(w.assets) == 0 {
		w.logger.Warnf("interest estimate: no autoborrow assets configured, worker disabled")
		return
	}

	if w.rates == nil {
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

func (w *interestEstimateWorker) countError(kind string) {
	interestEstimateErrorsMetrics.WithLabelValues(w.session.Name, w.session.ExchangeName.String(), kind).Inc()
}

func (w *interestEstimateWorker) tick(ctx context.Context) *interestEstimate {
	now := w.now().UTC()
	month := margininterest.MonthRange(now)
	// realized is [month start, floor(now)+1h), projected is the hours after floor(now)
	realizedRange, hours := month.SplitAt(now)

	debts := w.loadDebts(ctx)
	rates := w.queryRates(ctx)
	projection := margininterest.ProjectAssets(w.assets, debts, rates, w.price, hours)
	for range projection.MissingPrice {
		w.countError("price")
	}

	realized, realizedOK, unconfiguredRows := w.queryRealized(ctx, now, realizedRange)

	var rolling []margininterest.Projection
	for _, h := range w.config.projectionHours() {
		rolling = append(rolling, margininterest.ProjectAssets(w.assets, debts, rates, w.price, h))
	}

	est := &interestEstimate{
		Month:           month.Start,
		RemainingHours:  hours,
		Projection:      projection,
		Realized:        realized,
		RealizedOK:      realizedOK,
		MonthTotalUSDT:  realized.TotalUSDT.Add(projection.TotalUSDT),
		Rolling:         rolling,
		RollingComplete: w.rates != nil && allComplete(rolling),
	}

	for asset := range w.syncFailed {
		est.SyncFailed = append(est.SyncFailed, asset)
	}
	sort.Strings(est.SyncFailed)

	est.Unconfigured = w.unconfigured(debts, unconfiguredRows)
	if len(est.Unconfigured) > 0 {
		w.logger.Warnf("interest estimate: %v have debt or interest but are not autoborrow assets, they are not counted", est.Unconfigured)
	}

	missingHistory := !realizedOK || len(w.syncFailed) > 0
	est.Complete = w.rates != nil && projection.Complete() && !missingHistory && len(realized.MissingPrice) == 0

	// every autoborrow asset is listed, borrowed or not
	est.Assets = margininterest.MonthByAsset(projection, realized)
	for asset, m := range est.Assets {
		_, failed := w.syncFailed[asset]
		m.MissingHistory = !realizedOK || failed
		est.Assets[asset] = m
	}

	w.log(est, rates)
	w.publish(est)
	return est
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

// unconfigured returns the sorted assets outside w.assets that have principal or realized rows
func (w *interestEstimateWorker) unconfigured(debts types.BalanceMap, rowAssets []string) []string {
	configured := make(map[string]struct{}, len(w.assets))
	for _, asset := range w.assets {
		configured[asset] = struct{}{}
	}

	var out []string
	for asset, b := range debts {
		if _, ok := configured[asset]; !ok && b.Borrowed.Sign() > 0 {
			out = append(out, asset)
		}
	}
	return unionSorted(out, rowAssets)
}

// queryRates queries the next hourly rate of every autoborrow asset, borrowed or not
func (w *interestEstimateWorker) queryRates(ctx context.Context) types.MarginNextHourlyInterestRateMap {
	rates := types.MarginNextHourlyInterestRateMap{}
	if w.rates == nil {
		return rates
	}

	assets := w.assets

	for start := 0; start < len(assets); start += maxRateAssetsPerQuery {
		end := min(start+maxRateAssetsPerQuery, len(assets))
		chunk := assets[start:end]

		result, err := w.rates.QueryMarginFutureHourlyInterestRate(ctx, chunk)
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

// queryRealized syncs the interest history when due and sums the rows of the
// autoborrow assets in the hours r. unconfigured lists the other assets that
// have rows, e.g. synced by `bbgo sync` or before the asset was removed.
func (w *interestEstimateWorker) queryRealized(
	ctx context.Context, now time.Time, r margininterest.HourRange,
) (realized margininterest.Realized, ok bool, unconfigured []string) {
	mode := w.config.RealizedPriceMode.OrDefault()
	empty := margininterest.Realized{Mode: mode, TotalUSDT: fixedpoint.Zero}
	if w.history == nil {
		return empty, false, nil
	}

	if w.lastSync.IsZero() || now.Sub(w.lastSync) >= w.config.syncInterval() {
		w.syncHistory(ctx, now, r.Start)
		w.lastSync = now
	}

	// r ends after the current hour, so the charge at floor(now) and a borrow-time
	// row stamped inside the current hour are realized

	rows, err := w.history.QueryInterests(ctx, service.QueryMarginInterestOptions{
		Exchange: w.session.ExchangeName,
		Session:  w.session.Name,
		Since:    r.Start,
		Until:    r.End,
	})
	if err != nil {
		w.logger.WithError(err).Warnf("interest estimate: unable to query margin interests")
		w.countError("query")
		return empty, false, nil
	}

	configured := make(map[string]struct{}, len(w.assets))
	for _, asset := range w.assets {
		configured[asset] = struct{}{}
	}

	counted := rows[:0:0]
	others := map[string]struct{}{}
	for _, row := range rows {
		if _, ok := configured[row.Asset]; ok {
			counted = append(counted, row)
		} else {
			others[row.Asset] = struct{}{}
		}
	}

	for asset := range others {
		unconfigured = append(unconfigured, asset)
	}
	sort.Strings(unconfigured)

	return margininterest.SumRealized(counted, mode, w.price), true, unconfigured
}

// syncHistory syncs the interest rows of every autoborrow asset, borrowed or
// not, so the realized range starting at rangeStart is covered. A charge of an
// asset borrowed and repaid between two syncs is synced too.
func (w *interestEstimateWorker) syncHistory(ctx context.Context, now, rangeStart time.Time) {
	mu := sessionSyncLock(w.session.ExchangeName.String() + "/" + w.session.Name)
	mu.Lock()
	defer mu.Unlock()

	// rows inserted before the timeout are kept, and the next sync resumes from them
	sctx, cancel := context.WithTimeout(ctx, w.config.Interval.Duration())
	defer cancel()

	for _, asset := range w.assets {
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

	w.logger.Infof("interest estimate: session=%s month=%s remaining_hours=%d projected_usdt=%s realized_usdt=%s realized_mode=%s month_total_usdt=%s complete=%t missing_rate=%v missing_price=%v missing_history=%t sync_failed=%v unconfigured=%v",
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
		!est.RealizedOK || len(est.SyncFailed) > 0,
		est.SyncFailed,
		est.Unconfigured,
	)

	for _, p := range est.Rolling {
		w.logger.Infof("interest projection: session=%s hours=%d rolling_months=%d projected_usdt=%s complete=%t",
			w.session.Name, p.Hours, p.Hours/margininterest.RollingMonthHours, p.TotalUSDT.String(), w.rates != nil && p.Complete())
	}

	for _, asset := range estimateAssets(est) {
		p := est.Projection.Assets[asset]
		m := est.Assets[asset]
		hourlyRate := fixedpoint.Zero
		if rate, ok := rates[asset]; ok && rate != nil {
			hourlyRate = rate.HourlyRate
		}

		w.logger.Debugf("interest estimate asset: session=%s asset=%s principal=%s hourly_rate=%s hours=%d projected=%s projected_usdt=%s realized=%s realized_usdt=%s month_total=%s month_total_usdt=%s complete=%t missing_rate=%t missing_price=%t missing_history=%t",
			w.session.Name, asset,
			p.Principal.String(), hourlyRate.String(), p.Hours,
			m.Projected.String(), m.ProjectedUSDT.String(),
			m.Realized.String(), m.RealizedUSDT.String(),
			m.Total.String(), m.TotalUSDT.String(),
			m.Complete(), m.MissingRate, m.MissingPrice, m.MissingHistory,
		)
	}
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
	if est.RealizedOK {
		for asset, r := range est.Realized.Assets {
			realizedAmount[asset] = r.Interest.Float64()
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
}

func (w *interestEstimateWorker) publishRolling(est *interestEstimate) {
	var amount, usdt, total, complete []gaugeSample

	for _, p := range est.Rolling {
		hours := strconv.Itoa(p.Hours)
		rollingComplete := 0.0
		if w.rates != nil && p.Complete() {
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
