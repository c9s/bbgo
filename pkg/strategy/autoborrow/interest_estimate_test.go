package autoborrow

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/accounting/margininterest"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

type fakeRateService struct {
	calls  [][]string
	failOn map[int]error // 1-based call index
	rates  map[string]string
}

func (f *fakeRateService) QueryMarginFutureHourlyInterestRate(
	_ context.Context, assets []string,
) (types.MarginNextHourlyInterestRateMap, error) {
	f.calls = append(f.calls, append([]string(nil), assets...))
	if err := f.failOn[len(f.calls)]; err != nil {
		return nil, err
	}

	m := types.MarginNextHourlyInterestRateMap{}
	for _, asset := range assets {
		if r, ok := f.rates[asset]; ok {
			m[asset] = &types.MarginNextHourlyInterestRate{Asset: asset, HourlyRate: Number(r)}
		}
	}
	return m, nil
}

type syncCall struct {
	session, asset string
	start          time.Time
}

type fakeHistoryStore struct {
	syncs    []syncCall
	syncErr  map[string]error
	values   []syncCall
	valueErr error
	// valueTimeouts is how long each ValueInterests call was given
	valueTimeouts []time.Duration
	queries       []service.QueryMarginInterestValuationOptions
	rows          []types.MarginInterestValuation
	queryErr      error
}

func (f *fakeHistoryStore) SyncInterests(_ context.Context, _ types.Exchange, session, asset string, start time.Time) error {
	f.syncs = append(f.syncs, syncCall{session, asset, start})
	return f.syncErr[asset]
}

func (f *fakeHistoryStore) ValueInterests(ctx context.Context, _ types.Exchange, session string, since time.Time) error {
	f.values = append(f.values, syncCall{session: session, start: since})
	f.valueTimeouts = append(f.valueTimeouts, timeLeft(ctx))
	return f.valueErr
}

// timeLeft returns how long ctx has until its deadline, 0 without one
func timeLeft(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0
	}
	return time.Until(deadline)
}

// QueryValuations returns the rows in [Since, Until), like MarginInterestValuationService
func (f *fakeHistoryStore) QueryValuations(
	_ context.Context, opts service.QueryMarginInterestValuationOptions,
) ([]types.MarginInterestValuation, error) {
	f.queries = append(f.queries, opts)
	if f.queryErr != nil {
		return nil, f.queryErr
	}

	var rows []types.MarginInterestValuation
	for _, row := range f.rows {
		if t := row.Time.Time(); !t.Before(opts.Since) && t.Before(opts.Until) {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

var testNow = time.Date(2026, 11, 15, 10, 20, 0, 0, time.UTC)

var sessionSeq atomic.Int64

func testPrices(prices map[string]string) margininterest.PriceFunc {
	return func(asset string) (fixedpoint.Value, bool) {
		p, ok := prices[asset]
		if !ok {
			return fixedpoint.Zero, false
		}
		return Number(p), true
	}
}

// newTestInterestWorker builds a worker on a uniquely named session, because the metric vecs are global
func newTestInterestWorker(
	t *testing.T, account *types.Account, rates *fakeRateService, history *fakeHistoryStore, assets ...MarginAssetConfig,
) *interestEstimateWorker {
	t.Helper()

	return newTestInterestWorkerOf(t, newTestStrategy(t, account, newFakeMarginBorrowRepayService(nil), assets...), rates, history)
}

// newTestInterestWorkerOf builds the worker of s, for a test that changes s afterwards
func newTestInterestWorkerOf(t *testing.T, s *Strategy, rates *fakeRateService, history *fakeHistoryStore) *interestEstimateWorker {
	t.Helper()

	session := s.ExchangeSession
	session.Name = fmt.Sprintf("test-%d", sessionSeq.Add(1))
	session.ExchangeName = types.ExchangeBinance

	w := newInterestEstimateWorker(s, session, &InterestEstimateConfig{
		Interval:     types.Duration(5 * time.Minute),
		SyncInterval: types.Duration(time.Hour),
	})
	w.now = func() time.Time { return testNow }
	w.pricer = testPrices(map[string]string{"BTC": "60000", "ETH": "3000"})
	// the prices of a test are the ones of its pricer
	w.queryPrice = func(context.Context, string) error { return nil }
	if rates != nil {
		w.interestQueryService = rates
	}
	if history != nil {
		w.history = history
	}
	return w
}

// gathered returns the value of each series of the named metric, keyed by its labels
func gathered(t *testing.T, name string) []map[string]any {
	t.Helper()

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	var out []map[string]any
	for _, f := range families {
		if f.GetName() != name {
			continue
		}

		for _, m := range f.GetMetric() {
			series := map[string]any{}
			for _, l := range m.GetLabel() {
				series[l.GetName()] = l.GetValue()
			}
			if m.GetGauge() != nil {
				series["value"] = m.GetGauge().GetValue()
			} else {
				series["value"] = m.GetCounter().GetValue()
			}
			out = append(out, series)
		}
	}
	return out
}

func sessionGauge(t *testing.T, w *interestEstimateWorker, name string) float64 {
	t.Helper()
	for _, series := range gathered(t, name) {
		if series["session"] == w.session.Name {
			return series["value"].(float64)
		}
	}
	t.Fatalf("no %s series for session %s", name, w.session.Name)
	return 0
}

// assetSeries returns the per-asset series of the named metric that belong to the worker's session
func assetSeries(t *testing.T, w *interestEstimateWorker, name string) map[string]float64 {
	t.Helper()
	out := map[string]float64{}
	for _, series := range gathered(t, name) {
		if series["session"] == w.session.Name {
			out[series["asset"].(string)] = series["value"].(float64)
		}
	}
	return out
}

func errorCount(t *testing.T, w *interestEstimateWorker, kind string) float64 {
	t.Helper()
	for _, series := range gathered(t, "autoborrow_interest_estimate_errors_total") {
		if series["session"] == w.session.Name && series["kind"] == kind {
			return series["value"].(float64)
		}
	}
	return 0
}

// configured returns the autoborrow asset configs of the given assets
func configured(assets ...string) []MarginAssetConfig {
	out := make([]MarginAssetConfig, 0, len(assets))
	for _, asset := range assets {
		out = append(out, MarginAssetConfig{Asset: asset})
	}
	return out
}

func usdtRow(at time.Time, interest string) types.MarginInterestValuation {
	return valuedRow("USDT", interest, "1", at)
}

// valuedRow is a valued interest row; priceInUSDT "" leaves it unpriced
func valuedRow(asset, interest, priceInUSDT string, at time.Time) types.MarginInterestValuation {
	row := types.MarginInterestValuation{
		PaidAsset: asset, BorrowedAsset: asset, Interest: Number(interest), Time: types.Time(at),
	}
	borrowed := row.Interest
	row.BorrowedInterest = &borrowed
	if priceInUSDT != "" {
		price := Number(priceInUSDT)
		usdt := row.Interest.Mul(price)
		row.PriceInUSDT, row.InterestUSDT = &price, &usdt
	}
	return row
}

func TestInterestEstimate_ProjectionAndRealized(t *testing.T) {
	account := newTestAccount(Number(10),
		debtBalance("BTC", fixedpoint.Zero, Number(0.5)),
		debtBalance("USDT", fixedpoint.Zero, Number(1000)),
	)
	// ETH is configured but not borrowed: its rate is reported, its interest is 0
	rates := &fakeRateService{rates: map[string]string{"BTC": "0.00000417", "ETH": "0.000002", "USDT": "0.00000417"}}
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		usdtRow(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), "0.00417"),
		// exactly at the current hour: realized, not projected
		usdtRow(time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC), "0.00417"),
	}}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT", "ETH", "BTC")...)

	est := w.tick(context.Background())

	// 373 hours left: BTC 0.5 x 373 x 0.00000417 x 60000 = 46.6623, USDT 1000 x 373 x 0.00000417 = 1.55541.
	// Non-dnum keeps 8 decimals of BTC (0.00077770), so the USDT value can be price x 1e-8 = 6e-4 low.
	const usdtTol = 1e-3
	assert.Equal(t, 373, est.RemainingHours)
	assert.InDelta(t, 48.21771, est.Projection.TotalUSDT.Float64(), usdtTol)
	assert.InDelta(t, 0.00834, est.Realized.TotalUSDT.Float64(), 1e-9)
	assert.InDelta(t, 48.22605, est.MonthTotalUSDT.Float64(), usdtTol)
	assert.True(t, est.Complete)

	// every configured asset, borrowed or not
	assert.Equal(t, [][]string{{"BTC", "ETH", "USDT"}}, rates.calls)

	require.Len(t, history.queries, 1)
	assert.Equal(t, w.session.Name, history.queries[0].Session)
	assert.Equal(t, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), history.queries[0].Since)
	// realized ends after the current hour, where the projection starts
	assert.Equal(t, time.Date(2026, 11, 15, 11, 0, 0, 0, time.UTC), history.queries[0].Until)

	// without a sync start, syncs the configured assets from the month start,
	// plus BNB on Binance: interest paid with BNB deduction is returned only for BNB
	var synced []string
	for _, c := range history.syncs {
		assert.Equal(t, w.session.Name, c.session)
		assert.Equal(t, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), c.start)
		synced = append(synced, c.asset)
	}
	assert.Equal(t, []string{"BNB", "BTC", "ETH", "USDT"}, synced)

	assert.InDelta(t, 48.21771, sessionGauge(t, w, "autoborrow_interest_projected_total_in_usdt"), usdtTol)
	assert.InDelta(t, 48.22605, sessionGauge(t, w, "autoborrow_interest_month_total_in_usdt"), usdtTol)
	assert.Equal(t, 373.0, sessionGauge(t, w, "autoborrow_interest_remaining_hours"))
	assert.Equal(t, 1.0, sessionGauge(t, w, "autoborrow_interest_estimate_complete"))
	assert.InDelta(t, 46.6623, assetSeries(t, w, "autoborrow_interest_projected_in_usdt")["BTC"], usdtTol)
	assert.InDelta(t, 0.00834, assetSeries(t, w, "autoborrow_interest_realized_in_usdt")["USDT"], 1e-9)

	// per asset: USDT 1.55541 projected + 0.00834 realized; BTC projected only; ETH 0
	require.Len(t, est.Assets, 3)
	usdt := est.Assets["USDT"]
	assert.InDelta(t, 1.56375, usdt.Total.Float64(), 1e-9)
	assert.InDelta(t, 1.56375, usdt.TotalUSDT.Float64(), 1e-9)
	assert.True(t, usdt.Complete())
	assert.InDelta(t, 0.00077770, est.Assets["BTC"].Total.Float64(), 1e-8)
	eth := est.Assets["ETH"]
	assert.True(t, eth.Total.IsZero())
	assert.True(t, eth.Complete())
	assert.InDelta(t, 0.000002, assetSeries(t, w, "autoborrow_interest_hourly_rate")["ETH"], 1e-12)
	assert.Equal(t, 0.0, assetSeries(t, w, "autoborrow_interest_projected_amount")["ETH"])

	assert.InDelta(t, 0.00834, assetSeries(t, w, "autoborrow_interest_realized_amount")["USDT"], 1e-9)
	monthAmount := assetSeries(t, w, "autoborrow_interest_month_amount")
	assert.InDelta(t, 1.56375, monthAmount["USDT"], 1e-9)
	assert.InDelta(t, 0.00077770, monthAmount["BTC"], 1e-8)
	monthUSDT := assetSeries(t, w, "autoborrow_interest_month_in_usdt")
	assert.InDelta(t, 1.56375, monthUSDT["USDT"], 1e-9)
	assert.InDelta(t, 46.6623, monthUSDT["BTC"], usdtTol)
	assert.Equal(t, 0.0, monthAmount["ETH"])
	assert.Equal(t, map[string]float64{"BTC": 1, "ETH": 1, "USDT": 1}, assetSeries(t, w, "autoborrow_interest_asset_estimate_complete"))

	// the per-asset USDT adds up to the session month total
	assert.InDelta(t, est.MonthTotalUSDT.Float64(), monthUSDT["USDT"]+monthUSDT["BTC"]+monthUSDT["ETH"], 1e-9)
}

func TestInterestEstimate_RateChunks(t *testing.T) {
	var balances []types.Balance
	var assets []string
	rateMap := map[string]string{}
	for i := 0; i < 25; i++ {
		asset := fmt.Sprintf("A%02d", i)
		balances = append(balances, debtBalance(asset, fixedpoint.Zero, Number(1)))
		assets = append(assets, asset)
		rateMap[asset] = "0.000001"
	}
	rates := &fakeRateService{rates: rateMap, failOn: map[int]error{2: errors.New("rate limit")}}
	w := newTestInterestWorker(t, newTestAccount(Number(10), balances...), rates, &fakeHistoryStore{}, configured(assets...)...)

	est := w.tick(context.Background())

	require.Len(t, rates.calls, 2)
	assert.Len(t, rates.calls[0], 20)
	assert.Equal(t, "A00", rates.calls[0][0])
	assert.Equal(t, []string{"A20", "A21", "A22", "A23", "A24"}, rates.calls[1])
	assert.Len(t, est.Projection.MissingRate, 5)
	assert.False(t, est.Complete)
	assert.Equal(t, 1.0, errorCount(t, w, "rate"))
}

func TestInterestEstimate_NoDatabase(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("USDT", fixedpoint.Zero, Number(1000)))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.00000417"}}
	w := newTestInterestWorker(t, account, rates, nil, configured("USDT")...)
	require.Nil(t, w.history)

	est := w.tick(context.Background())

	assert.False(t, est.RealizedOK)
	assert.False(t, est.Complete)
	assert.InDelta(t, 1.55541, sessionGauge(t, w, "autoborrow_interest_projected_total_in_usdt"), 1e-6)
	assert.Equal(t, 0.0, sessionGauge(t, w, "autoborrow_interest_estimate_complete"))
}

func TestInterestEstimate_SyncCadenceAndFailure(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("USDT", fixedpoint.Zero, Number(1000)))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.00000417"}}
	history := &fakeHistoryStore{syncErr: map[string]error{"USDT": errors.New("timeout")}}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)

	est := w.tick(context.Background())
	assert.Len(t, history.syncs, 2) // BNB and USDT
	assert.False(t, est.Complete)
	assert.Equal(t, []string{"USDT"}, est.SyncFailed)
	assert.Equal(t, 1.0, errorCount(t, w, "sync"))
	assert.True(t, est.Assets["USDT"].MissingHistory)
	assert.Equal(t, 0.0, assetSeries(t, w, "autoborrow_interest_asset_estimate_complete")["USDT"])

	// within the sync interval, only the failed asset is synced again: it is
	// retried on the next tick, not an interval later
	now := testNow.Add(5 * time.Minute)
	w.now = func() time.Time { return now }
	est = w.tick(context.Background())
	assert.Equal(t, []string{"USDT"}, syncedAssets(history, 2))
	assert.Len(t, history.values, 2)
	assert.Len(t, history.queries, 2)
	assert.False(t, est.Complete)

	// the retry succeeds and clears the failure
	history.syncErr = nil
	now = testNow.Add(10 * time.Minute)
	est = w.tick(context.Background())
	assert.Equal(t, []string{"USDT"}, syncedAssets(history, 3))
	assert.Empty(t, est.SyncFailed)
	assert.True(t, est.Complete)

	// nothing is pending any more: within the sync interval, query only
	now = testNow.Add(15 * time.Minute)
	est = w.tick(context.Background())
	assert.Len(t, history.syncs, 4)
	assert.Len(t, history.queries, 4)

	// after the sync interval every asset is synced again
	now = testNow.Add(time.Hour)
	est = w.tick(context.Background())
	assert.Equal(t, []string{"BNB", "USDT"}, syncedAssets(history, 4))
	assert.Empty(t, est.SyncFailed)
	assert.True(t, est.Complete)
	assert.True(t, est.Assets["USDT"].Complete())
	assert.Equal(t, 1.0, assetSeries(t, w, "autoborrow_interest_asset_estimate_complete")["USDT"])
}

func TestInterestEstimate_SyncFailedAssetWithoutDebtIsListed(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("USDT", fixedpoint.Zero, Number(1000)))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.00000417"}}
	history := &fakeHistoryStore{syncErr: map[string]error{"ETH": errors.New("timeout")}}
	w := newTestInterestWorker(t, account, rates, history, configured("ETH", "USDT")...)

	est := w.tick(context.Background())
	eth, ok := est.Assets["ETH"]
	require.True(t, ok)
	assert.True(t, eth.MissingHistory)
	assert.True(t, eth.Total.IsZero())
	assert.True(t, est.Assets["USDT"].Complete())
	assert.Equal(t, 0.0, assetSeries(t, w, "autoborrow_interest_asset_estimate_complete")["ETH"])
}

func TestInterestEstimate_QueryError(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("USDT", fixedpoint.Zero, Number(1000)))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.00000417"}}
	history := &fakeHistoryStore{queryErr: errors.New("db down")}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)

	est := w.tick(context.Background())
	assert.False(t, est.RealizedOK)
	assert.False(t, est.Complete)
	assert.Equal(t, 1.0, errorCount(t, w, "query"))
	assert.Equal(t, 0.0, sessionGauge(t, w, "autoborrow_interest_realized_total_in_usdt"))
}

func TestInterestEstimate_StaleSeriesAreDeleted(t *testing.T) {
	account := newTestAccount(Number(10),
		debtBalance("BTC", fixedpoint.Zero, Number(0.5)),
		debtBalance("ETH", fixedpoint.Zero, Number(2)),
		debtBalance("USDT", fixedpoint.Zero, Number(1000)),
	)
	rates := &fakeRateService{rates: map[string]string{"BTC": "0.00000417", "ETH": "0.000002", "USDT": "0.00000417"}}
	w := newTestInterestWorker(t, account, rates, &fakeHistoryStore{}, configured("BTC", "ETH", "USDT")...)

	w.tick(context.Background())
	assert.Len(t, assetSeries(t, w, "autoborrow_interest_projected_in_usdt"), 3)
	assert.Len(t, assetSeries(t, w, "autoborrow_interest_hourly_rate"), 3)

	// BTC is repaid, ETH loses its price, USDT loses its rate
	account.UpdateBalances(types.BalanceMap{
		"BTC":  debtBalance("BTC", Number(1), fixedpoint.Zero),
		"ETH":  debtBalance("ETH", fixedpoint.Zero, Number(2)),
		"USDT": debtBalance("USDT", fixedpoint.Zero, Number(1000)),
	})
	w.pricer = testPrices(map[string]string{"BTC": "60000"})
	delete(rates.rates, "USDT")

	est := w.tick(context.Background())
	assert.Equal(t, []string{"USDT"}, est.Projection.MissingRate)
	assert.Equal(t, []string{"ETH"}, est.Projection.MissingPrice)

	// the repaid BTC stays at 0, ETH loses its USDT series, USDT loses its rate and projection
	assert.Equal(t, map[string]float64{"BTC": 0}, assetSeries(t, w, "autoborrow_interest_projected_in_usdt"))

	amounts := assetSeries(t, w, "autoborrow_interest_projected_amount")
	assert.ElementsMatch(t, []string{"BTC", "ETH"}, keys(amounts))
	assert.Equal(t, 0.0, amounts["BTC"])

	hourly := assetSeries(t, w, "autoborrow_interest_hourly_rate")
	assert.ElementsMatch(t, []string{"BTC", "ETH"}, keys(hourly))

	// per asset: BTC is 0 and complete, ETH has an amount but no USDT (no price),
	// USDT has no rate, so its 0 figures are published as incomplete
	assert.ElementsMatch(t, []string{"BTC", "ETH", "USDT"}, keys(assetSeries(t, w, "autoborrow_interest_month_amount")))
	assert.Equal(t, map[string]float64{"BTC": 0, "USDT": 0}, assetSeries(t, w, "autoborrow_interest_month_in_usdt"))
	assert.Equal(t, map[string]float64{"BTC": 1, "ETH": 0, "USDT": 0}, assetSeries(t, w, "autoborrow_interest_asset_estimate_complete"))
	assert.True(t, est.Assets["USDT"].MissingRate)
	assert.True(t, est.Assets["ETH"].AmountComplete())
}

func TestInterestEstimate_SessionsDoNotShareState(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("USDT", fixedpoint.Zero, Number(1000)))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.00000417"}}

	a := newTestInterestWorker(t, account, rates, &fakeHistoryStore{syncErr: map[string]error{"USDT": errors.New("x")}}, configured("USDT")...)
	b := newTestInterestWorker(t, account, rates, &fakeHistoryStore{}, configured("USDT")...)

	a.tick(context.Background())
	b.tick(context.Background())

	assert.NotEmpty(t, a.syncFailed)
	assert.Empty(t, b.syncFailed)
	assert.Equal(t, 0.0, sessionGauge(t, a, "autoborrow_interest_estimate_complete"))
	assert.Equal(t, 1.0, sessionGauge(t, b, "autoborrow_interest_estimate_complete"))
}

func TestInterestEstimateConfig_Validate(t *testing.T) {
	assert.NoError(t, (&InterestEstimateConfig{}).Validate())
	assert.NoError(t, (&InterestEstimateConfig{RealizedPriceMode: margininterest.PriceModeCurrent}).Validate())
	assert.Error(t, (&InterestEstimateConfig{RealizedPriceMode: "spot"}).Validate())

	assert.Error(t, (&InterestEstimateConfig{ProjectionMonths: []int{1, 0}}).Validate())
	assert.Error(t, (&InterestEstimateConfig{ProjectionMonths: []int{-3}}).Validate())

	assert.Equal(t, []int{720, 2160}, (&InterestEstimateConfig{}).projectionHours())
	assert.Equal(t, []int{720, 4320}, (&InterestEstimateConfig{ProjectionMonths: []int{6, 1, 1}}).projectionHours())
	assert.Empty(t, (&InterestEstimateConfig{ProjectionMonths: []int{}}).projectionHours())

	assert.Equal(t, time.Hour, (&InterestEstimateConfig{}).syncInterval())
	assert.Error(t, (&Strategy{InterestEstimate: &InterestEstimateConfig{RealizedPriceMode: "spot"}}).Validate())
	assert.NoError(t, (&Strategy{}).Validate())
}

func keys(m map[string]float64) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestInterestEstimate_SyncStartsAtSyncSince(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("USDT", fixedpoint.Zero, Number(1000)))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.00000417"}}
	monthStart := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	t.Run("sync since before the month start", func(t *testing.T) {
		history := &fakeHistoryStore{}
		w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)
		since := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
		w.syncSince = since

		w.tick(context.Background())

		// same start as the full margin history sync, so it never resumes past a gap
		require.NotEmpty(t, history.syncs)
		for _, c := range history.syncs {
			assert.Equal(t, since, c.start)
		}

		// the realized query still covers the month only
		require.Len(t, history.queries, 1)
		assert.Equal(t, monthStart, history.queries[0].Since)
	})

	t.Run("sync since inside the month", func(t *testing.T) {
		history := &fakeHistoryStore{}
		w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)
		w.syncSince = time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)

		w.tick(context.Background())

		require.NotEmpty(t, history.syncs)
		for _, c := range history.syncs {
			assert.Equal(t, monthStart, c.start)
		}
	})
}

// rollingSeries returns the series of the named metric of the worker's session, keyed by "asset/hours" or "hours"
func rollingSeries(t *testing.T, w *interestEstimateWorker, name string) map[string]float64 {
	t.Helper()
	out := map[string]float64{}
	for _, series := range gathered(t, name) {
		if series["session"] != w.session.Name {
			continue
		}
		key := series["hours"].(string)
		if asset, ok := series["asset"].(string); ok {
			key = asset + "/" + key
		}
		out[key] = series["value"].(float64)
	}
	return out
}

func TestInterestEstimate_RollingMonths(t *testing.T) {
	account := newTestAccount(Number(10),
		debtBalance("BTC", fixedpoint.Zero, Number(0.5)),
		debtBalance("USDT", fixedpoint.Zero, Number(1000)),
		debtBalance("PEPE", fixedpoint.Zero, Number(1e6)),
	)
	rates := &fakeRateService{rates: map[string]string{"BTC": "0.000001", "USDT": "0.000002", "PEPE": "0.000001"}}
	// realized rows do not count in a rolling projection
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{usdtRow(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), "5")}}
	w := newTestInterestWorker(t, account, rates, history, configured("BTC", "PEPE", "USDT")...)

	est := w.tick(context.Background())

	// the default is 1 and 3 rolling months, whatever the calendar month has left (373 hours)
	require.Len(t, est.Rolling, 2)
	one, three := est.Rolling[0], est.Rolling[1]
	assert.Equal(t, 720, one.Hours)
	assert.Equal(t, 2160, three.Hours)

	// 720h: BTC 0.5 x 720 x 0.000001 = 0.00036 (21.6 USDT), USDT 1000 x 720 x 0.000002 = 1.44, PEPE 720 (no price)
	assert.InDelta(t, 0.00036, one.Assets["BTC"].Interest.Float64(), 1e-8)
	assert.InDelta(t, 720, one.Assets["PEPE"].Interest.Float64(), 1e-6)
	assert.InDelta(t, 23.04, one.TotalUSDT.Float64(), 1e-3)
	assert.InDelta(t, 69.12, three.TotalUSDT.Float64(), 1e-3)
	assert.False(t, est.RollingComplete)

	totals := rollingSeries(t, w, "autoborrow_interest_rolling_projected_total_in_usdt")
	assert.InDelta(t, 23.04, totals["720"], 1e-3)
	assert.InDelta(t, 69.12, totals["2160"], 1e-3)
	assert.Equal(t, map[string]float64{"720": 0, "2160": 0}, rollingSeries(t, w, "autoborrow_interest_rolling_projection_complete"))

	amount := rollingSeries(t, w, "autoborrow_interest_rolling_projected_amount")
	assert.Len(t, amount, 6)
	assert.InDelta(t, 4.32, amount["USDT/2160"], 1e-7)
	assert.InDelta(t, 2160, amount["PEPE/2160"], 1e-6)

	// PEPE has no price, so it has no USDT series
	usdt := rollingSeries(t, w, "autoborrow_interest_rolling_projected_in_usdt")
	assert.Len(t, usdt, 4)
	assert.InDelta(t, 64.8, usdt["BTC/2160"], 1e-3)
	_, ok := usdt["PEPE/720"]
	assert.False(t, ok)

	// a repaid asset stays at 0, which needs no price
	account.UpdateBalances(types.BalanceMap{"PEPE": debtBalance("PEPE", Number(1), fixedpoint.Zero)})
	est = w.tick(context.Background())
	assert.True(t, est.RollingComplete)
	amount = rollingSeries(t, w, "autoborrow_interest_rolling_projected_amount")
	assert.Len(t, amount, 6)
	assert.Equal(t, 0.0, amount["PEPE/720"])
	assert.Equal(t, 0.0, rollingSeries(t, w, "autoborrow_interest_rolling_projected_in_usdt")["PEPE/720"])
	assert.Equal(t, map[string]float64{"720": 1, "2160": 1}, rollingSeries(t, w, "autoborrow_interest_rolling_projection_complete"))
}

func TestInterestEstimate_NoRollingMonths(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("USDT", fixedpoint.Zero, Number(1000)))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000002"}}
	w := newTestInterestWorker(t, account, rates, &fakeHistoryStore{}, configured("USDT")...)
	w.config.ProjectionMonths = []int{}

	est := w.tick(context.Background())

	assert.Empty(t, est.Rolling)
	assert.Empty(t, rollingSeries(t, w, "autoborrow_interest_rolling_projected_total_in_usdt"))
}

// syncedAssets returns the assets synced since the call index from
func syncedAssets(history *fakeHistoryStore, from int) []string {
	var out []string
	for _, c := range history.syncs[from:] {
		out = append(out, c.asset)
	}
	return out
}

func TestInterestEstimate_ConfiguredAssetsAreSyncedWithoutDebt(t *testing.T) {
	// nothing is borrowed now, but BTC was borrowed and repaid between two ticks
	account := newTestAccount(Number(10))
	rates := &fakeRateService{rates: map[string]string{"BTC": "0.000001", "USDT": "0.000002"}}
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		valuedRow("BTC", "0.00001", "60000", time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)),
	}}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT", "BTC")...)

	est := w.tick(context.Background())
	assert.Equal(t, []string{"BNB", "BTC", "USDT"}, syncedAssets(history, 0))
	assert.True(t, est.Projection.TotalUSDT.IsZero())
	assert.InDelta(t, 0.6, est.MonthTotalUSDT.Float64(), 1e-6)
	assert.True(t, est.Complete)
	assert.ElementsMatch(t, []string{"BTC", "USDT"}, keys(assetSeries(t, w, "autoborrow_interest_month_in_usdt")))

	// a month later, still no debt: every configured asset is still synced
	later := testNow.AddDate(0, 1, 0)
	w.now = func() time.Time { return later }
	n := len(history.syncs)
	w.tick(context.Background())
	assert.Equal(t, []string{"BNB", "BTC", "USDT"}, syncedAssets(history, n))
}

// The estimate covers the session, not the asset list: a debt opened outside
// autoborrow and the charges of an asset that is not configured both count
func TestInterestEstimate_AssetsOutsideTheConfigAreCounted(t *testing.T) {
	account := newTestAccount(Number(10),
		debtBalance("USDT", fixedpoint.Zero, Number(1000)),
		// borrowed outside the autoborrow assets
		debtBalance("DOGE", fixedpoint.Zero, Number(100)),
	)
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001", "DOGE": "0.000001"}}
	h := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		usdtRow(h, "0.001"),
		// charged earlier this month and repaid; ETH is not configured (any more)
		valuedRow("ETH", "0.001", "3000", h),
		bnbPaidRow("ETH", "0.0000003", "0.0000015", h.Add(time.Hour)),
	}}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)
	w.pricer = testPrices(map[string]string{"DOGE": "0.1", "ETH": "3000"})

	est := w.tick(context.Background())

	// the debt is projected: 373 hours x 0.000001 on 100 DOGE at 0.1, and on 1000 USDT
	assert.Equal(t, [][]string{{"DOGE", "USDT"}}, rates.calls)
	assert.ElementsMatch(t, []string{"DOGE", "ETH", "USDT"}, keysOfMonths(est.Assets))
	assert.InDelta(t, 0.00373, est.Projection.Assets["DOGE"].USDT.Float64(), 1e-8)
	assert.InDelta(t, 0.37673, est.Projection.TotalUSDT.Float64(), 1e-8)

	// the charges are realized: 0.001 USDT, 3 USDT of ETH and 0.0009 USDT paid in BNB for ETH
	assert.InDelta(t, 3.0019, est.Realized.TotalUSDT.Float64(), 1e-8)
	assert.InDelta(t, 3.0009, est.Assets["ETH"].RealizedUSDT.Float64(), 1e-8)
	assert.ElementsMatch(t, []string{"DOGE", "ETH", "USDT"}, keys(assetSeries(t, w, "autoborrow_interest_month_in_usdt")))

	// DOGE has debt, so it is synced. ETH only has rows that something else
	// stored: its history may be incomplete until this worker has synced it.
	assert.Equal(t, []string{"BNB", "DOGE", "USDT"}, syncedAssets(history, 0))
	assert.True(t, est.Assets["ETH"].MissingHistory)
	assert.True(t, est.Assets["USDT"].Complete())
	assert.True(t, est.Assets["DOGE"].Complete())
	assert.False(t, est.Complete)

	// which happens on the next tick, not a sync interval later
	now := testNow.Add(5 * time.Minute)
	w.now = func() time.Time { return now }
	n := len(history.syncs)
	est = w.tick(context.Background())
	assert.Equal(t, []string{"ETH"}, syncedAssets(history, n))
	assert.True(t, est.Complete)
	assert.Equal(t, 1.0, sessionGauge(t, w, "autoborrow_interest_estimate_complete"))
}

// An asset repaid during the month keeps its charges in the month total and is
// still synced, so its last charges arrive. The next month starts without it.
func TestInterestEstimate_RepaidAssetStaysUntilTheMonthEnds(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("DOGE", fixedpoint.Zero, Number(100)))
	rates := &fakeRateService{rates: map[string]string{"DOGE": "0.000001"}}
	history := &fakeHistoryStore{}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)
	w.pricer = testPrices(map[string]string{"DOGE": "0.1"})
	now := testNow
	w.now = func() time.Time { return now }

	est := w.tick(context.Background())
	assert.Equal(t, []string{"BNB", "DOGE", "USDT"}, syncedAssets(history, 0))
	assert.False(t, est.Projection.Assets["DOGE"].Interest.IsZero())

	// DOGE is repaid; its last hourly charge is published after that
	account.UpdateBalances(types.BalanceMap{"DOGE": debtBalance("DOGE", Number(1), fixedpoint.Zero)})
	history.rows = append(history.rows, valuedRow("DOGE", "0.0001", "0.1", testNow.Truncate(time.Hour)))
	now = testNow.Add(time.Hour)
	n := len(history.syncs)
	est = w.tick(context.Background())
	assert.Equal(t, []string{"BNB", "DOGE", "USDT"}, syncedAssets(history, n))
	assert.True(t, est.Projection.Assets["DOGE"].Interest.IsZero())
	assert.InDelta(t, 0.00001, est.Assets["DOGE"].RealizedUSDT.Float64(), 1e-9)
	assert.True(t, est.Complete)

	// the next month: no debt and no charge, DOGE is not part of the estimate
	now = testNow.AddDate(0, 1, 0)
	n = len(history.syncs)
	est = w.tick(context.Background())
	assert.Equal(t, []string{"BNB", "USDT"}, syncedAssets(history, n))
	assert.ElementsMatch(t, []string{"USDT"}, keysOfMonths(est.Assets))
}

// A failed sync of an asset that is no longer part of the estimate does not
// keep the estimate incomplete: nothing would ever sync it again
func TestInterestEstimate_SyncFailureOfADroppedAssetIsForgotten(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("DOGE", fixedpoint.Zero, Number(100)))
	rates := &fakeRateService{rates: map[string]string{"DOGE": "0.000001"}}
	history := &fakeHistoryStore{syncErr: map[string]error{"DOGE": errors.New("timeout")}}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)
	w.pricer = testPrices(map[string]string{"DOGE": "0.1"})
	now := testNow
	w.now = func() time.Time { return now }

	est := w.tick(context.Background())
	assert.Equal(t, []string{"DOGE"}, est.SyncFailed)
	assert.False(t, est.Complete)

	// repaid: it stays in the estimate, and incomplete, until the month ends
	account.UpdateBalances(types.BalanceMap{"DOGE": debtBalance("DOGE", Number(1), fixedpoint.Zero)})
	now = testNow.Add(5 * time.Minute)
	est = w.tick(context.Background())
	assert.Equal(t, []string{"DOGE"}, est.SyncFailed)
	assert.False(t, est.Complete)

	now = testNow.AddDate(0, 1, 0)
	est = w.tick(context.Background())
	assert.Empty(t, est.SyncFailed)
	assert.True(t, est.Complete)
}

// An asset with debt on the last tick of a month is charged at the first hour
// of the next one, also when it is repaid before the first tick of that month:
// it starts the month covered, so the next sync fetches that charge
func TestInterestEstimate_DebtAtTheMonthEndIsCoveredInTheNextMonth(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("DOGE", fixedpoint.Zero, Number(100)))
	rates := &fakeRateService{rates: map[string]string{"DOGE": "0.000001"}}
	history := &fakeHistoryStore{}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)
	w.pricer = testPrices(map[string]string{"DOGE": "0.1"})
	now := time.Date(2026, 11, 30, 23, 58, 0, 0, time.UTC)
	w.now = func() time.Time { return now }

	w.tick(context.Background())

	// repaid at 00:01, after the charge of 00:00
	account.UpdateBalances(types.BalanceMap{"DOGE": debtBalance("DOGE", Number(1), fixedpoint.Zero)})
	now = time.Date(2026, 12, 1, 0, 3, 0, 0, time.UTC)
	est := w.tick(context.Background())
	assert.ElementsMatch(t, []string{"DOGE", "USDT"}, keysOfMonths(est.Assets))

	// the next full sync
	history.rows = append(history.rows, valuedRow("DOGE", "0.0001", "0.1", time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)))
	now = time.Date(2026, 12, 1, 1, 0, 0, 0, time.UTC)
	n := len(history.syncs)
	est = w.tick(context.Background())
	assert.Equal(t, []string{"BNB", "DOGE", "USDT"}, syncedAssets(history, n))
	assert.InDelta(t, 0.00001, est.Assets["DOGE"].RealizedUSDT.Float64(), 1e-9)
	assert.True(t, est.Complete)
}

// An asset that left the estimate and comes back is new again: rows may be
// missing since its last sync, so it is synced on the tick it shows up, from
// the start
func TestInterestEstimate_ReturningAssetIsSyncedAgain(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("DOGE", fixedpoint.Zero, Number(100)))
	rates := &fakeRateService{rates: map[string]string{"DOGE": "0.000001"}}
	history := &fakeHistoryStore{}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)
	w.pricer = testPrices(map[string]string{"DOGE": "0.1"})
	now := testNow
	w.now = func() time.Time { return now }

	w.tick(context.Background())

	// repaid in November, not part of December
	account.UpdateBalances(types.BalanceMap{"DOGE": debtBalance("DOGE", Number(1), fixedpoint.Zero)})
	now = testNow.Add(5 * time.Minute)
	w.tick(context.Background())
	now = time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC)
	n := len(history.syncs)
	w.tick(context.Background())
	assert.Equal(t, []string{"BNB", "USDT"}, syncedAssets(history, n))

	// borrowed again between two full syncs
	account.UpdateBalances(types.BalanceMap{"DOGE": debtBalance("DOGE", fixedpoint.Zero, Number(100))})
	now = now.Add(5 * time.Minute)
	n = len(history.syncs)
	est := w.tick(context.Background())
	assert.Equal(t, []string{"DOGE"}, syncedAssets(history, n))
	// not from its November sync: from the month start
	assert.Equal(t, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), syncStarts(history, n)["DOGE"])
	assert.True(t, est.Complete)
}

// The autoborrow assets are read on every tick, like the borrow loop reads
// them: an asset added while bbgo runs is in the next estimate
func TestInterestEstimate_FollowsTheAssetListAtRuntime(t *testing.T) {
	account := newTestAccount(Number(10))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001", "ETH": "0.000001"}}
	history := &fakeHistoryStore{}
	s := newTestStrategy(t, account, newFakeMarginBorrowRepayService(nil), configured("USDT")...)
	w := newTestInterestWorkerOf(t, s, rates, history)
	now := testNow
	w.now = func() time.Time { return now }

	est := w.tick(context.Background())
	assert.ElementsMatch(t, []string{"USDT"}, keysOfMonths(est.Assets))

	s.Assets = append(s.Assets, configured("ETH")...)
	now = testNow.Add(5 * time.Minute)
	n := len(history.syncs)
	est = w.tick(context.Background())
	assert.ElementsMatch(t, []string{"ETH", "USDT"}, keysOfMonths(est.Assets))
	assert.Equal(t, []string{"ETH", "USDT"}, rates.calls[len(rates.calls)-1])
	// the new asset is synced at once
	assert.Equal(t, []string{"ETH"}, syncedAssets(history, n))
	assert.True(t, est.Complete)

	// an asset taken off the list stays until the month ends
	s.Assets = configured("ETH")
	now = testNow.Add(10 * time.Minute)
	est = w.tick(context.Background())
	assert.ElementsMatch(t, []string{"ETH", "USDT"}, keysOfMonths(est.Assets))

	// it is still synced, and a failed sync of it still makes the estimate incomplete
	history.syncErr = map[string]error{"USDT": errors.New("timeout")}
	now = testNow.Add(time.Hour)
	n = len(history.syncs)
	est = w.tick(context.Background())
	assert.Equal(t, []string{"BNB", "ETH", "USDT"}, syncedAssets(history, n))
	assert.Equal(t, []string{"USDT"}, est.SyncFailed)
	assert.False(t, est.Complete)
	history.syncErr = nil

	now = testNow.AddDate(0, 1, 0)
	est = w.tick(context.Background())
	assert.ElementsMatch(t, []string{"ETH"}, keysOfMonths(est.Assets))
}

// Without any autoborrow asset the estimate still covers what the session borrowed
func TestInterestEstimate_RunsWithoutConfiguredAssets(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("USDT", fixedpoint.Zero, Number(1000)))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001"}}
	w := newTestInterestWorker(t, account, rates, &fakeHistoryStore{})

	assert.Empty(t, w.disabledReason())
	est := w.tick(context.Background())
	assert.InDelta(t, 0.373, est.Projection.TotalUSDT.Float64(), 1e-8)
	assert.True(t, est.Complete)
}

func keysOfMonths(m map[string]margininterest.AssetMonth) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestInterestEstimate_TopOfHourChargeIsRealizedOnly(t *testing.T) {
	h10 := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	account := newTestAccount(Number(10), debtBalance("USDT", fixedpoint.Zero, Number(1000)))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001"}}
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		usdtRow(h10, "0.001"),
		// a charge the host has not reached yet, e.g. a clock behind Binance
		usdtRow(h10.Add(time.Hour), "0.001"),
	}}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)
	w.now = func() time.Time { return h10 }

	est := w.tick(context.Background())

	// 10:00 is realized; 11:00 .. Nov 30 23:00 (373 hours) is projected, and not realized as well
	assert.Equal(t, 373, est.RemainingHours)
	assert.Equal(t, h10.Add(time.Hour), history.queries[0].Until)
	assert.InDelta(t, 0.001, est.Realized.TotalUSDT.Float64(), 1e-12)
	assert.InDelta(t, 0.373, est.Projection.TotalUSDT.Float64(), 1e-9)
}

// syncStarts returns the sync start of each asset synced since the call index from
func syncStarts(history *fakeHistoryStore, from int) map[string]time.Time {
	out := map[string]time.Time{}
	for _, c := range history.syncs[from:] {
		out[c.asset] = c.start
	}
	return out
}

func TestInterestEstimate_SyncResumesFromTheLastSuccessfulSync(t *testing.T) {
	// nothing borrowed: no stored row to resume from
	account := newTestAccount(Number(10))
	history := &fakeHistoryStore{syncErr: map[string]error{"BTC": errors.New("timeout")}}
	w := newTestInterestWorker(t, account, &fakeRateService{}, history, configured("BTC", "USDT")...)
	since := time.Date(2025, 11, 15, 0, 0, 0, 0, time.UTC)
	w.syncSince = since

	w.tick(context.Background())
	assert.Equal(t, map[string]time.Time{"BNB": since, "BTC": since, "USDT": since}, syncStarts(history, 0))

	// USDT synced: it starts 1h before that sync instead of a year back;
	// BTC failed, so it scans from syncSince again
	later := testNow.Add(time.Hour)
	w.now = func() time.Time { return later }
	n := len(history.syncs)
	w.tick(context.Background())
	assert.Equal(t, map[string]time.Time{
		"BNB": testNow.Add(-time.Hour), "BTC": since, "USDT": testNow.Add(-time.Hour),
	}, syncStarts(history, n))

	// once BTC syncs, it resumes the same way
	delete(history.syncErr, "BTC")
	later = later.Add(time.Hour)
	w.tick(context.Background())
	later = later.Add(time.Hour)
	n = len(history.syncs)
	w.tick(context.Background())
	assert.Equal(t, later.Add(-2*time.Hour), syncStarts(history, n)["BTC"])
}

func TestInterestEstimate_RollingCompleteInTheLastHourOfTheMonth(t *testing.T) {
	// PEPE is borrowed and has a rate, but no price
	account := newTestAccount(Number(10), debtBalance("PEPE", fixedpoint.Zero, Number(1e6)))
	rates := &fakeRateService{rates: map[string]string{"PEPE": "0.000001"}}
	w := newTestInterestWorker(t, account, rates, &fakeHistoryStore{}, configured("PEPE")...)
	lastHour := time.Date(2026, 11, 30, 23, 30, 0, 0, time.UTC)
	w.now = func() time.Time { return lastHour }

	est := w.tick(context.Background())

	// the calendar month has no hour left, so its zero interest needs no price
	assert.Equal(t, 0, est.RemainingHours)
	assert.True(t, est.Projection.Complete())

	// the rolling windows still project PEPE, which has no price
	assert.False(t, est.RollingComplete)
	assert.Equal(t, map[string]float64{"720": 0, "2160": 0}, rollingSeries(t, w, "autoborrow_interest_rolling_projection_complete"))
}

func TestInterestEstimate_ValuesTheRealizedRangeOnEveryTick(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("USDT", fixedpoint.Zero, Number(1000)))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001"}}
	history := &fakeHistoryStore{}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)
	w.syncSince = time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	w.tick(context.Background())

	// the raw rows sync from the full sync start, the valuation only covers the month
	require.Len(t, history.values, 1)
	assert.Equal(t, w.session.Name, history.values[0].session)
	assert.Equal(t, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), history.values[0].start)
	assert.Equal(t, w.syncSince, history.syncs[0].start)

	// and again on the next tick, although no sync is due: a row whose price was
	// not available is not left unpriced until the next sync
	n := len(history.syncs)
	w.tick(context.Background())
	assert.Len(t, history.values, 2)
	assert.Len(t, history.syncs, n)

	// each pass has its own time, whatever the sync before it used
	for _, left := range history.valueTimeouts {
		assert.Greater(t, left, valuationTimeout-5*time.Second)
		assert.LessOrEqual(t, left, valuationTimeout)
	}
}

func TestInterestEstimate_ValuationErrorIsCounted(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("USDT", fixedpoint.Zero, Number(1000)))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001"}}
	history := &fakeHistoryStore{
		valueErr: errors.New("kline api down"),
		rows:     []types.MarginInterestValuation{usdtRow(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), "0.001")},
	}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)

	est := w.tick(context.Background())

	// the rows valued before still count, but the ones just synced may have no
	// value yet and are not read: the estimate cannot say it is complete
	assert.Equal(t, 1.0, errorCount(t, w, "value"))
	assert.InDelta(t, 0.001, est.Realized.TotalUSDT.Float64(), 1e-12)
	assert.True(t, est.MissingHistory)
	assert.True(t, est.Assets["USDT"].MissingHistory)
	assert.False(t, est.Complete)

	// tried again on the next tick, not a sync interval later
	history.valueErr = nil
	est = w.tick(context.Background())
	assert.False(t, est.MissingHistory)
	assert.True(t, est.Complete)
}

func TestInterestEstimate_UnpricedRowIsIncomplete(t *testing.T) {
	account := newTestAccount(Number(10))
	rates := &fakeRateService{rates: map[string]string{"BTC": "0.000001"}}
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		valuedRow("BTC", "0.00001", "60000", time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)),
		// its kline is not synced yet
		valuedRow("BTC", "0.00001", "", time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC)),
	}}
	w := newTestInterestWorker(t, account, rates, history, configured("BTC")...)

	est := w.tick(context.Background())

	assert.Equal(t, []string{"BTC"}, est.Realized.MissingPrice)
	assert.InDelta(t, 0.00002, est.Realized.Assets["BTC"].Interest.Float64(), 1e-12)
	assert.InDelta(t, 0.6, est.Realized.TotalUSDT.Float64(), 1e-6)
	assert.False(t, est.Complete)
}

// bnbPaidRow is a valued hourly charge on borrowed paid in BNB at 600 USDT;
// nominal is principal x daily rate / 24 in borrowed units
func bnbPaidRow(borrowed, nominal, bnbInterest string, at time.Time) types.MarginInterestValuation {
	row := valuedRow("BNB", bnbInterest, "600", at)
	n := Number(nominal)
	row.BorrowedAsset, row.Converted, row.BorrowedInterest = borrowed, true, &n
	return row
}

func TestInterestEstimate_BNBPaidInterestCountsTowardTheBorrowedAsset(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("USDT", fixedpoint.Zero, Number(1000)))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001"}}
	h := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{
		usdtRow(h, "0.001"),
		bnbPaidRow("USDT", "0.001", "0.0000016", h.Add(time.Hour)),
	}}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)

	est := w.tick(context.Background())

	usdt := est.Realized.Assets["USDT"]
	assert.Equal(t, 2, usdt.Rows)
	assert.InDelta(t, 0.002, usdt.Interest.Float64(), 1e-12)
	assert.InDelta(t, 0.001, usdt.Charged["USDT"].Float64(), 1e-12)
	assert.InDelta(t, 0.0000016, usdt.Charged["BNB"].Float64(), 1e-12)
	// valued at the BNB charged: 0.0000016 x 600 = 0.00096
	assert.InDelta(t, 0.00196, est.Realized.TotalUSDT.Float64(), 1e-8)
	assert.NotContains(t, est.Realized.Assets, "BNB")
	assert.True(t, est.Complete)

	assert.InDelta(t, 0.002, assetSeries(t, w, "autoborrow_interest_realized_amount")["USDT"], 1e-12)
	charged := map[string]float64{}
	for _, s := range gathered(t, "autoborrow_interest_realized_charged_amount") {
		if s["session"] == w.session.Name {
			charged[s["charged_asset"].(string)] = s["value"].(float64)
		}
	}
	assert.InDelta(t, 0.001, charged["USDT"], 1e-12)
	assert.InDelta(t, 0.0000016, charged["BNB"], 1e-12)
}

func TestInterestEstimate_FailedBNBSyncMarksEveryAssetIncomplete(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("USDT", fixedpoint.Zero, Number(1000)))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001"}}
	history := &fakeHistoryStore{syncErr: map[string]error{"BNB": errors.New("timeout")}}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)

	est := w.tick(context.Background())

	assert.Equal(t, []string{"BNB"}, est.SyncFailed)
	assert.False(t, est.Complete)
	assert.True(t, est.Assets["USDT"].MissingHistory)
	assert.NotContains(t, est.Assets, "BNB")
}

func TestInterestEstimate_UnattributedRowIsIncomplete(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("USDT", fixedpoint.Zero, Number(1000)))
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001"}}
	row := bnbPaidRow("", "0", "0.0000016", time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC))
	row.BorrowedInterest = nil
	history := &fakeHistoryStore{rows: []types.MarginInterestValuation{row}}
	w := newTestInterestWorker(t, account, rates, history, configured("USDT")...)

	est := w.tick(context.Background())

	assert.Equal(t, 1, est.Unattributed)
	assert.Empty(t, est.Realized.Assets)
	assert.False(t, est.Complete)
	assert.Equal(t, 1.0, errorCount(t, w, "unattributed"))
}

// The session only has the prices of the markets a strategy subscribed to at
// startup, so the worker queries the price of what it estimates itself.
func TestInterestEstimate_QueriesThePricesItNeeds(t *testing.T) {
	// queried lists the assets whose price a worker of newWorker queried, in order
	var queried []string
	newWorker := func(mode margininterest.PriceMode) *interestEstimateWorker {
		queried = nil
		prices := map[string]fixedpoint.Value{}
		account := newTestAccount(Number(10),
			debtBalance("USDT", fixedpoint.Zero, Number(1000)),
			// borrowed outside the autoborrow assets: nothing subscribed to its market
			debtBalance("DOGE", fixedpoint.Zero, Number(100)),
			debtBalance("XYZ", fixedpoint.Zero, Number(1)),
		)
		rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001", "DOGE": "0.000001", "XYZ": "0.000001"}}
		w := newTestInterestWorker(t, account, rates, &fakeHistoryStore{}, configured("USDT", "ETH")...)
		w.config.RealizedPriceMode = mode
		w.pricer = func(asset string) (fixedpoint.Value, bool) {
			if asset == "USDT" {
				return fixedpoint.One, true
			}
			price, ok := prices[asset]
			return price, ok
		}

		w.queryPrice = func(_ context.Context, asset string) error {
			queried = append(queried, asset)
			if asset == "XYZ" {
				return errors.New("no ticker")
			}
			prices[asset] = Number(0.1)
			return nil
		}
		return w
	}

	t.Run("of every debt", func(t *testing.T) {
		w := newWorker(margininterest.PriceModeCharged)
		est := w.tick(context.Background())

		// not USDT, and not ETH: it has no debt, and what it was charged is valued when charged
		assert.Equal(t, []string{"DOGE", "XYZ"}, queried)
		// 373 hours x 0.000001 on 100 DOGE at the price just queried
		assert.InDelta(t, 0.00373, est.Projection.Assets["DOGE"].USDT.Float64(), 1e-8)
		// a price that cannot be queried is a missing price, not an error of the tick
		assert.Equal(t, []string{"XYZ"}, est.Projection.MissingPrice)
		assert.Equal(t, []string{"XYZ"}, est.StalePrice)
		assert.False(t, est.Complete)

		// and again on the next tick: a price is not kept from the first query
		w.tick(context.Background())
		assert.Equal(t, []string{"DOGE", "XYZ", "DOGE", "XYZ"}, queried)
	})

	t.Run("of every asset when realized is valued at the current price", func(t *testing.T) {
		w := newWorker(margininterest.PriceModeCurrent)
		w.tick(context.Background())

		// BNB too: interest paid in BNB is valued at its current price
		assert.Equal(t, []string{"BNB", "DOGE", "ETH", "XYZ"}, queried)
	})
}

// The price is queried through the price solver of the session, which is what
// the estimate reads its prices from
func TestInterestEstimate_QueriedPriceReachesTheSession(t *testing.T) {
	account := newTestAccount(Number(10), debtBalance("ETH", fixedpoint.Zero, Number(10)))
	s := newTestStrategy(t, account, newFakeMarginBorrowRepayService(nil))
	session := s.ExchangeSession
	session.Name = fmt.Sprintf("test-%d", sessionSeq.Add(1))
	session.ExchangeName = types.ExchangeBinance
	session.SetMarkets(types.MarketMap{"ETHUSDT": Market("ETHUSDT"), "BTCUSDT": Market("BTCUSDT")})

	ticker := Ticker("ETHUSDT")
	exchange := session.Exchange.(*mocks.MockExchange)
	exchange.EXPECT().QueryTicker(gomock.Any(), "ETHUSDT").Return(&ticker, nil).Times(1)
	exchange.EXPECT().QueryTicker(gomock.Any(), "BTCUSDT").Return(&types.Ticker{}, nil).Times(1)

	w := newInterestEstimateWorker(s, session, &InterestEstimateConfig{Interval: types.Duration(5 * time.Minute)})

	_, ok := w.pricer("ETH")
	assert.False(t, ok)

	require.NoError(t, w.queryPrice(context.Background(), "ETH"))
	price, ok := w.pricer("ETH")
	require.True(t, ok)
	assert.Equal(t, ticker.GetValidPrice().String(), price.String())

	// the worker has to know when a price was not refreshed: a ticker without a
	// price and an asset without a USDT market are errors, not silence
	assert.ErrorContains(t, w.queryPrice(context.Background(), "BTC"), "no price")
	_, ok = w.pricer("BTC")
	assert.False(t, ok)
	assert.ErrorContains(t, w.queryPrice(context.Background(), "XYZ"), "no USDT market")
}

// A price that could not be queried on this tick is an old one. The figures
// still use it, which is better than nothing, but the estimate and the budgets
// on it do not pass for complete.
func TestInterestEstimate_StalePriceIsIncomplete(t *testing.T) {
	account := newTestAccount(Number(10),
		debtBalance("USDT", fixedpoint.Zero, Number(1000)),
		debtBalance("DOGE", fixedpoint.Zero, Number(100)),
	)
	rates := &fakeRateService{rates: map[string]string{"USDT": "0.000001", "DOGE": "0.000001"}}
	w := newTestInterestWorker(t, account, rates, &fakeHistoryStore{}, configured("USDT")...)
	w.pricer = testPrices(map[string]string{"DOGE": "0.1"})
	w.config.Budget = &InterestBudgetConfig{
		MonthlyUSDT: Number(100),
		Assets:      map[string]fixedpoint.Value{"DOGE": Number(10), "USDT": Number(10)},
	}

	var queryErr error
	var timeouts []time.Duration
	w.queryPrice = func(ctx context.Context, _ string) error {
		timeouts = append(timeouts, timeLeft(ctx))
		return queryErr
	}

	est := w.tick(context.Background())
	assert.Empty(t, est.StalePrice)
	assert.True(t, est.Complete)
	assert.Equal(t, margininterest.BudgetStatusOK, est.Budget.Status())

	queryErr = errors.New("ticker api down")
	est = w.tick(context.Background())

	// still projected, at the price the session has: 373 hours x 0.000001 x 100 DOGE x 0.1
	assert.InDelta(t, 0.00373, est.Projection.Assets["DOGE"].USDT.Float64(), 1e-8)
	assert.Empty(t, est.Projection.MissingPrice)
	assert.Equal(t, []string{"DOGE"}, est.StalePrice)
	assert.Equal(t, []string{"DOGE"}, est.StaleDebtPrice)
	assert.True(t, est.Assets["DOGE"].StalePrice)
	assert.False(t, est.Assets["DOGE"].Complete())
	assert.True(t, est.Assets["USDT"].Complete())
	assert.False(t, est.Complete)
	assert.False(t, est.RollingComplete)
	assert.Equal(t, 0.0, sessionGauge(t, w, "autoborrow_interest_estimate_complete"))
	assert.Equal(t, 1.0, errorCount(t, w, "price_query"))

	// within the budgets, but not known to be: only where the stale price is used
	assert.Equal(t, margininterest.BudgetStatusUnknown, est.Budget.Status())
	assert.Equal(t, margininterest.BudgetStatusUnknown, est.AssetBudgets["DOGE"].Status())
	assert.Equal(t, margininterest.BudgetStatusOK, est.AssetBudgets["USDT"].Status())

	// queried again on the next tick
	queryErr = nil
	est = w.tick(context.Background())
	assert.Empty(t, est.StalePrice)
	assert.True(t, est.Complete)

	// the queries of a tick share one bounded time
	for _, left := range timeouts {
		assert.Greater(t, left, priceQueryTimeout-5*time.Second)
		assert.LessOrEqual(t, left, priceQueryTimeout)
	}
}
