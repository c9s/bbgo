package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

func newTestMarginService(t *testing.T) *MarginService {
	db, err := prepareDB(t)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return &MarginService{DB: sqlx.NewDb(db.DB, "sqlite3")}
}

// insertInterest inserts the row the same way the sync insert hook does
func insertInterest(t *testing.T, s *MarginService, session, asset string, at time.Time, interest, price string) {
	t.Helper()
	err := insertType(s.DB, types.MarginInterest{
		Exchange:    types.ExchangeBinance,
		Asset:       asset,
		Principle:   Number(1000),
		Interest:    Number(interest),
		Session:     session,
		PriceInUSDT: Number(price),
		Time:        types.Time(at.UTC()),
	}, false)
	require.NoError(t, err)
}

func TestMarginService_QueryInterests(t *testing.T) {
	ctx := context.Background()
	s := newTestMarginService(t)

	nov1 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	dec1 := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	plus8 := time.FixedZone("+8", 8*3600)

	for _, session := range []string{"a", "b", ""} {
		insertInterest(t, s, session, "USDT", nov1, "0.00417", "1")
		insertInterest(t, s, session, "BTC", mid, "0.00000209", "60000")
		insertInterest(t, s, session, "USDT", dec1, "0.00417", "1")
	}
	// local times on either side of the month boundary
	insertInterest(t, s, "a", "USDT", time.Date(2026, 10, 31, 20, 0, 0, 0, time.UTC).In(plus8), "0.001", "1")
	insertInterest(t, s, "a", "USDT", time.Date(2026, 11, 30, 20, 0, 0, 0, time.UTC).In(plus8), "0.002", "1")

	rows, err := s.QueryInterests(ctx, QueryMarginInterestOptions{
		Exchange: types.ExchangeBinance,
		Session:  "a",
		Since:    nov1,
		Until:    dec1,
	})
	require.NoError(t, err)
	require.Len(t, rows, 3)

	assert.Equal(t, nov1, rows[0].Time.Time().UTC())
	assert.Equal(t, mid, rows[1].Time.Time().UTC())
	assert.Equal(t, time.Date(2026, 11, 30, 20, 0, 0, 0, time.UTC), rows[2].Time.Time().UTC())
	for _, row := range rows {
		assert.Equal(t, "a", row.Session)
	}
	assert.Equal(t, "BTC", rows[1].Asset)
	assert.InDelta(t, 60000.0, rows[1].PriceInUSDT.Float64(), 1e-9)
	assert.InDelta(t, 0.00000209, rows[1].Interest.Float64(), 1e-12)

	rows, err = s.QueryInterests(ctx, QueryMarginInterestOptions{
		Exchange: types.ExchangeBinance,
		Session:  "a",
		Assets:   []string{"BTC"},
		Since:    nov1,
		Until:    dec1,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "BTC", rows[0].Asset)
}

func TestMarginService_QueryInterests_HourBoundaries(t *testing.T) {
	ctx := context.Background()
	s := newTestMarginService(t)

	h10 := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	insertInterest(t, s, "a", "USDT", h10, "0.00417", "1")
	// a prorated borrow-time charge inside the 10:00 hour
	insertInterest(t, s, "a", "USDT", h10.Add(25*time.Minute), "0.001", "1")
	insertInterest(t, s, "a", "USDT", h10.Add(time.Hour), "0.00417", "1")

	query := func(since, until time.Time) ([]types.MarginInterest, error) {
		return s.QueryInterests(ctx, QueryMarginInterestOptions{
			Exchange: types.ExchangeBinance, Session: "a", Since: since, Until: until,
		})
	}

	// the 10:00 hour holds its hourly charge and the borrow-time charge
	rows, err := query(h10, h10.Add(time.Hour))
	require.NoError(t, err)
	assert.Len(t, rows, 2)

	// the hour boundary is the same instant in any zone
	rows, err = query(h10.In(time.FixedZone("+8", 8*3600)), h10.Add(2*time.Hour))
	require.NoError(t, err)
	assert.Len(t, rows, 3)

	_, err = query(h10.Add(time.Minute), h10.Add(time.Hour))
	assert.Error(t, err)
	_, err = query(h10, h10.Add(time.Hour+time.Second))
	assert.Error(t, err)
	// +05:30 midnight is not on a UTC hour boundary
	_, err = query(time.Date(2026, 11, 15, 0, 0, 0, 0, time.FixedZone("+0530", 5*3600+1800)), h10)
	assert.Error(t, err)
}

func TestSelectLastMarginInterests_Session(t *testing.T) {
	ctx := context.Background()
	s := newTestMarginService(t)

	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	insertInterest(t, s, "a", "USDT", at, "0.00417", "1")
	insertInterest(t, s, "", "USDT", at.Add(time.Hour), "0.00417", "1")

	records, err := selectAndScanType(ctx, s.DB, SelectLastMarginInterests(types.ExchangeBinance, "a", "USDT", 100), types.MarginInterest{})
	require.NoError(t, err)
	assert.Len(t, records.([]types.MarginInterest), 1)

	records, err = selectAndScanType(ctx, s.DB, SelectLastMarginInterests(types.ExchangeBinance, "new", "USDT", 100), types.MarginInterest{})
	require.NoError(t, err)
	assert.Empty(t, records.([]types.MarginInterest))
}

func TestInsertMarginTypes_WithSession(t *testing.T) {
	ctx := context.Background()
	s := newTestMarginService(t)
	at := types.Time(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))

	require.NoError(t, insertType(s.DB, types.MarginLoan{
		Exchange: types.ExchangeBinance, TransactionID: 1, Asset: "USDT", Principle: Number(10), Time: at, Session: "a",
	}, false))
	require.NoError(t, insertType(s.DB, types.MarginRepay{
		Exchange: types.ExchangeBinance, TransactionID: 2, Asset: "USDT", Principle: Number(10), Time: at, Session: "a",
	}, false))
	require.NoError(t, insertType(s.DB, types.MarginLiquidation{
		Exchange: types.ExchangeBinance, OrderID: 3, Symbol: "BTCUSDT", UpdatedTime: at, Session: "a",
	}, false))

	loans, err := selectAndScanType(ctx, s.DB, SelectLastMarginLoans(types.ExchangeBinance, "a", "USDT", 10), types.MarginLoan{})
	require.NoError(t, err)
	assert.Len(t, loans.([]types.MarginLoan), 1)

	repays, err := selectAndScanType(ctx, s.DB, SelectLastMarginRepays(types.ExchangeBinance, "a", "USDT", 10), types.MarginRepay{})
	require.NoError(t, err)
	assert.Len(t, repays.([]types.MarginRepay), 1)

	liqs, err := selectAndScanType(ctx, s.DB, SelectLastMarginLiquidations(types.ExchangeBinance, "a", 10), types.MarginLiquidation{})
	require.NoError(t, err)
	assert.Len(t, liqs.([]types.MarginLiquidation), 1)
	assert.Equal(t, "a", liqs.([]types.MarginLiquidation)[0].Session)
}

func TestMarginService_SessionNameTooLong(t *testing.T) {
	s := &MarginService{}
	long := strings.Repeat("s", MaxSessionNameLength+1)
	// skipped, so the startup sync of the other sessions goes on
	assert.NoError(t, s.Sync(context.Background(), nil, long, "USDT", time.Now()))
	assert.Error(t, s.SyncInterests(context.Background(), nil, long, "USDT", time.Now()))

	// the column counts characters: 64 multi-byte characters fit
	assert.NoError(t, ValidateMarginSessionName(strings.Repeat("帳", MaxSessionNameLength)))
	assert.Error(t, ValidateMarginSessionName(strings.Repeat("帳", MaxSessionNameLength+1)))
}

func TestLastRecordTime_ClampStartTime(t *testing.T) {
	old := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	monthStart := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	records := []types.MarginInterest{{Time: types.Time(old)}}

	sel := SyncTask{Time: func(obj interface{}) time.Time { return obj.(types.MarginInterest).Time.Time() }}
	assert.Equal(t, old, lastRecordTime(sel, reflectValueOf(records), monthStart))

	sel.ClampStartTime = true
	assert.Equal(t, monthStart, lastRecordTime(sel, reflectValueOf(records), monthStart))

	// a newer last record still wins
	recent := monthStart.Add(5 * time.Hour)
	records = []types.MarginInterest{{Time: types.Time(recent)}}
	assert.Equal(t, recent, lastRecordTime(sel, reflectValueOf(records), monthStart))
}

func TestLastRecordTime_ResumeOverlap(t *testing.T) {
	last := time.Date(2026, 11, 15, 10, 30, 0, 0, time.UTC)
	records := []types.MarginInterest{{Time: types.Time(last)}}
	sel := SyncTask{
		Time:          func(obj interface{}) time.Time { return obj.(types.MarginInterest).Time.Time() },
		ResumeOverlap: time.Hour,
	}
	assert.Equal(t, last.Add(-time.Hour), lastRecordTime(sel, reflectValueOf(records), time.Time{}))

	// no records: the start time, with no overlap
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, start, lastRecordTime(sel, reflectValueOf([]types.MarginInterest{}), start))
}

func TestLastRecordTime_ResumeOverlapStaysInTheLoadedRows(t *testing.T) {
	last := time.Date(2026, 11, 15, 10, 30, 0, 0, time.UTC)
	sel := SyncTask{
		Time:          func(obj interface{}) time.Time { return obj.(types.MarginInterest).Time.Time() },
		ResumeOverlap: time.Hour,
		SelectLimit:   3,
	}

	// a full Select: 3 rows within the last 20 minutes, older rows may not be loaded
	full := []types.MarginInterest{
		{Time: types.Time(last.Add(-20 * time.Minute))},
		{Time: types.Time(last.Add(-10 * time.Minute))},
		{Time: types.Time(last)},
	}
	assert.Equal(t, last.Add(-20*time.Minute), lastRecordTime(sel, reflectValueOf(full), time.Time{}))

	// fewer rows than the limit: every row is loaded, the full overlap applies
	assert.Equal(t, last.Add(-time.Hour), lastRecordTime(sel, reflectValueOf(full[1:]), time.Time{}))
}

// binanceName is an exchange that only has a name, enough to build a sync task
type binanceName struct{ types.Exchange }

func (binanceName) Name() types.ExchangeName { return types.ExchangeBinance }

func TestInterestSync_FetchesARowPublishedLate(t *testing.T) {
	ctx := context.Background()
	s := newTestMarginService(t)

	h10 := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	// the borrow-time row at 10:30 was synced before the 10:00 charge was published
	insertInterest(t, s, "a", "USDT", h10.Add(30*time.Minute), "0.0005", "1")

	task := s.interestSyncTask(ctx, binanceName{}, nil, "a", "USDT")
	require.Equal(t, time.Hour, task.ResumeOverlap)

	var since time.Time
	task.BatchQuery = func(_ context.Context, startTime, _ time.Time) (interface{}, chan error) {
		since = startTime
		c := make(chan types.MarginInterest, 3)
		errC := make(chan error, 1)
		for _, at := range []time.Time{h10, h10.Add(30 * time.Minute), h10.Add(time.Hour)} {
			c <- types.MarginInterest{Exchange: types.ExchangeBinance, Asset: "USDT", Interest: Number("0.001"), Time: types.Time(at)}
		}
		close(c)
		close(errC)
		return c, errC
	}
	// no kline pricing in this test
	task.Insert = func(obj interface{}) error {
		m := obj.(types.MarginInterest)
		m.Session = "a"
		return insertType(s.DB, m, false)
	}

	require.NoError(t, task.execute(ctx, s.DB, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), h10.Add(2*time.Hour)))
	assert.Equal(t, h10.Add(-30*time.Minute), since.UTC())

	// 10:00 is fetched, 10:30 is not stored twice, 11:00 is new
	rows, err := s.QueryInterests(ctx, QueryMarginInterestOptions{
		Exchange: types.ExchangeBinance, Session: "a", Since: h10, Until: h10.Add(2 * time.Hour),
	})
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, h10, rows[0].Time.Time().UTC())
	assert.InDelta(t, 0.0005, rows[1].Interest.Float64(), 1e-12)
}

// fakeKLineExchange serves hourly klines in [StartTime, EndTime], at most
// fakeKLinePageSize per call, like the Binance kline API
type fakeKLineExchange struct {
	types.Exchange
	calls int
	err   error
	// open returns the open price of the hour, ok = false when it has no kline
	open func(hour time.Time) (fixedpoint.Value, bool)
}

const fakeKLinePageSize = 500

func (f *fakeKLineExchange) QueryKLines(_ context.Context, _ string, _ types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}

	var ks []types.KLine
	for h := options.StartTime.Truncate(time.Hour); !h.After(*options.EndTime) && len(ks) < fakeKLinePageSize; h = h.Add(time.Hour) {
		if h.Before(*options.StartTime) {
			continue
		}
		if open, ok := f.open(h); ok {
			ks = append(ks, types.KLine{StartTime: types.Time(h), Open: open})
		}
	}
	return ks, nil
}

// opensFrom returns open = base + hours since start for every hour from start
func opensFrom(start time.Time, base int64) func(time.Time) (fixedpoint.Value, bool) {
	return func(h time.Time) (fixedpoint.Value, bool) {
		if h.Before(start) {
			return fixedpoint.Zero, false
		}
		return fixedpoint.NewFromInt(base + int64(h.Sub(start)/time.Hour)), true
	}
}

func noKLines(time.Time) (fixedpoint.Value, bool) { return fixedpoint.Zero, false }

func TestHourlyOpenPricer(t *testing.T) {
	ctx := context.Background()
	markets := types.MarketMap{
		"BTCUSDT": Market("BTCUSDT"),
		"USDTTWD": Market("USDTTWD"),
	}
	h10 := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	at := func(now time.Time) func() time.Time { return func() time.Time { return now } }

	priceAt := func(t *testing.T, p *hourlyOpenPricer, at time.Time) fixedpoint.Value {
		price, err := p.priceAt(ctx, at)
		assert.NoError(t, err)
		return price
	}

	t.Run("usdt is 1 without a query", func(t *testing.T) {
		ex := &fakeKLineExchange{}
		p := newHourlyOpenPricer(ex, markets, "USDT")
		assert.Equal(t, fixedpoint.One, priceAt(t, p, h10))
		assert.Zero(t, ex.calls)
	})

	t.Run("no market", func(t *testing.T) {
		ex := &fakeKLineExchange{}
		p := newHourlyOpenPricer(ex, markets, "DOGE")
		assert.True(t, priceAt(t, p, h10).IsZero())
		assert.Zero(t, ex.calls)
		assert.Equal(t, 1, p.misses)
	})

	t.Run("one batch prices every hour until now", func(t *testing.T) {
		ex := &fakeKLineExchange{open: opensFrom(h10, 60000)}
		p := newHourlyOpenPricer(ex, markets, "BTC")
		// 50 days: more than one page of klines
		p.now = at(h10.Add(50 * 24 * time.Hour))

		assert.InDelta(t, 60000, priceAt(t, p, h10).Float64(), 1e-6)
		assert.InDelta(t, 60000, priceAt(t, p, h10.Add(37*time.Minute)).Float64(), 1e-6)
		assert.InDelta(t, 60001, priceAt(t, p, h10.Add(time.Hour)).Float64(), 1e-6)
		assert.InDelta(t, 61200, priceAt(t, p, h10.Add(50*24*time.Hour)).Float64(), 1e-6)
		calls := ex.calls
		assert.Greater(t, calls, 2, "paged by the batch query")

		// every hour until now is cached
		priceAt(t, p, h10.Add(30*24*time.Hour))
		assert.Equal(t, calls, ex.calls)
	})

	t.Run("inverse pair", func(t *testing.T) {
		// USDTTWD open 32: 1 TWD = 1/32 USDT
		ex := &fakeKLineExchange{open: func(time.Time) (fixedpoint.Value, bool) { return fixedpoint.NewFromInt(32), true }}
		p := newHourlyOpenPricer(ex, markets, "TWD")
		p.now = at(h10.Add(3 * time.Hour))
		assert.InDelta(t, 1.0/32, priceAt(t, p, h10).Float64(), 1e-8)
	})

	t.Run("query error is returned after retries, not cached as 0", func(t *testing.T) {
		ex := &fakeKLineExchange{err: errors.New("boom")}
		p := newHourlyOpenPricer(ex, markets, "BTC")
		p.now = at(h10.Add(90 * 24 * time.Hour))
		_, err := p.priceAt(ctx, h10)
		assert.Error(t, err)
		assert.Equal(t, 1+interestPriceMaxRetries, ex.calls)

		// the next call queries again and succeeds
		ex.err = nil
		ex.open = opensFrom(h10, 60000)
		assert.InDelta(t, 60000, priceAt(t, p, h10).Float64(), 1e-6)
		assert.Zero(t, p.misses)
	})

	t.Run("recent kline not published yet is an error", func(t *testing.T) {
		ex := &fakeKLineExchange{open: noKLines}
		p := newHourlyOpenPricer(ex, markets, "BTC")
		p.now = at(h10.Add(5 * time.Second))
		_, err := p.priceAt(ctx, h10)
		assert.Error(t, err)
		assert.Zero(t, p.misses)
	})

	t.Run("a gap inside a fetched range is not fetched again", func(t *testing.T) {
		// klines from 12:00 on: 10:00 and 11:00 are before the listing
		ex := &fakeKLineExchange{open: opensFrom(h10.Add(2*time.Hour), 60000)}
		p := newHourlyOpenPricer(ex, markets, "BTC")
		p.now = at(h10.Add(10 * 24 * time.Hour))

		assert.True(t, priceAt(t, p, h10).IsZero())
		calls := ex.calls
		assert.True(t, priceAt(t, p, h10.Add(time.Hour)).IsZero())
		assert.True(t, priceAt(t, p, h10.Add(70*time.Minute)).IsZero())
		assert.InDelta(t, 60000, priceAt(t, p, h10.Add(2*time.Hour)).Float64(), 1e-6)
		assert.Equal(t, calls, ex.calls)
	})

	t.Run("a recent hour is fetched again once published", func(t *testing.T) {
		published := false
		ex := &fakeKLineExchange{open: func(h time.Time) (fixedpoint.Value, bool) {
			return fixedpoint.NewFromInt(60000), published || h.Before(h10)
		}}
		p := newHourlyOpenPricer(ex, markets, "BTC")
		now := h10.Add(30 * time.Minute)
		p.now = func() time.Time { return now }

		_, err := p.priceAt(ctx, h10)
		assert.Error(t, err, "not published yet")

		published = true
		now = now.Add(3 * time.Hour)
		assert.InDelta(t, 60000, priceAt(t, p, h10).Float64(), 1e-6)
	})

	t.Run("old hour without a kline is priced 0 once", func(t *testing.T) {
		ex := &fakeKLineExchange{open: noKLines}
		p := newHourlyOpenPricer(ex, markets, "BTC")
		p.now = at(h10.Add(90 * 24 * time.Hour))
		assert.True(t, priceAt(t, p, h10).IsZero())
		assert.True(t, priceAt(t, p, h10.Add(time.Minute)).IsZero())
		assert.Equal(t, 1, ex.calls)
		assert.Equal(t, 2, p.misses)
	})
}

func reflectValueOf(v interface{}) reflect.Value {
	return reflect.ValueOf(v)
}

// fakeMarginExchange is a cross-margin exchange whose 1h kline query fails
type fakeMarginExchange struct {
	types.Exchange
	interests    []types.MarginInterest
	liquidations int

	// interestAssets are the assets the interest history was queried for;
	// the batch query calls it from its own goroutine
	mu             sync.Mutex
	interestAssets []string
}

func (f *fakeMarginExchange) queriedInterestAssets() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.interestAssets...)
}

func (f *fakeMarginExchange) Name() types.ExchangeName { return types.ExchangeBinance }
func (f *fakeMarginExchange) UseMargin()               {}
func (f *fakeMarginExchange) UseIsolatedMargin(string) {}
func (f *fakeMarginExchange) GetMarginSettings() types.MarginSettings {
	return types.MarginSettings{IsMargin: true}
}

func (f *fakeMarginExchange) QueryMarkets(context.Context) (types.MarketMap, error) {
	return types.MarketMap{"BTCUSDT": Market("BTCUSDT")}, nil
}

func (f *fakeMarginExchange) QueryKLines(context.Context, string, types.Interval, types.KLineQueryOptions) ([]types.KLine, error) {
	return nil, errors.New("kline api is down")
}

func (f *fakeMarginExchange) QueryLoanHistory(context.Context, string, *time.Time, *time.Time) ([]types.MarginLoan, error) {
	return nil, nil
}

func (f *fakeMarginExchange) QueryRepayHistory(context.Context, string, *time.Time, *time.Time) ([]types.MarginRepay, error) {
	return nil, nil
}

func (f *fakeMarginExchange) QueryInterestHistory(_ context.Context, asset string, _, _ *time.Time) ([]types.MarginInterest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.interestAssets = append(f.interestAssets, asset)
	return f.interests, nil
}

func (f *fakeMarginExchange) QueryLiquidationHistory(context.Context, *time.Time, *time.Time) ([]types.MarginLiquidation, error) {
	f.liquidations++
	return nil, nil
}

func TestMarginService_SyncGoesOnWhenAnInterestIsNotPriced(t *testing.T) {
	t.Setenv("USE_MARKETS_CACHE_IN_MEMORY", "true")
	s := newTestMarginService(t)
	charged := time.Now().UTC().Truncate(time.Hour).Add(-3 * time.Hour)
	ex := &fakeMarginExchange{interests: []types.MarginInterest{
		{Exchange: types.ExchangeBinance, Asset: "BTC", Principle: Number(1), Interest: Number("0.00001"), Time: types.Time(charged)},
	}}

	// the startup sync does not fail, and the tasks after the interests still run
	require.NoError(t, s.Sync(context.Background(), ex, "a", "BTC", charged.Add(-time.Hour)))
	assert.Positive(t, ex.liquidations)

	// the row is not stored with price 0, so the next sync retries it
	rows, err := s.QueryInterests(context.Background(), QueryMarginInterestOptions{
		Exchange: types.ExchangeBinance, Session: "a", Since: charged.Add(-time.Hour), Until: charged.Add(time.Hour),
	})
	require.NoError(t, err)
	assert.Empty(t, rows)

	// the worker sync reports it, so the estimate is incomplete
	err = s.SyncInterests(context.Background(), ex, "a", "BTC", charged.Add(-time.Hour))
	assert.ErrorIs(t, err, errInterestNotPriced)
}

func TestInterestSyncTask_IDKeepsBNBPaidRowsApart(t *testing.T) {
	s := newTestMarginService(t)
	ex := &fakeMarginExchange{}
	task := s.interestSyncTask(context.Background(), ex, ex, "a", "BNB")

	// one charge hour has a BNB-paid row per borrowed asset
	at := types.Time(time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC))
	btc := types.MarginInterest{Asset: "BNB", RawAsset: "BTC", Time: at}
	eth := types.MarginInterest{Asset: "BNB", RawAsset: "ETH", Time: at}
	assert.NotEqual(t, task.ID(btc), task.ID(eth))
	assert.Equal(t, task.ID(btc), task.ID(btc))

	// a fetched row without RawAsset matches its stored copy, where it is filled
	fetched := types.MarginInterest{Asset: "USDT", Time: at}
	assert.Equal(t, task.ID(fetched), task.ID(normalizeInterest(fetched, "a")))

	// the batch query dedups with the same key
	assert.Equal(t, btc.Key(), task.ID(btc))
}

func TestNormalizeInterest(t *testing.T) {
	at := time.Date(2026, 11, 15, 18, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))

	m := normalizeInterest(types.MarginInterest{Asset: "USDT", Time: types.Time(at)}, "a")
	assert.Equal(t, "a", m.Session)
	assert.Equal(t, "USDT", m.RawAsset)
	assert.Equal(t, time.UTC, m.Time.Time().Location())

	m = normalizeInterest(types.MarginInterest{Asset: "BNB", RawAsset: "BTC", Time: types.Time(at)}, "a")
	assert.Equal(t, "BTC", m.RawAsset)
}

func TestMarginService_InterestRawAssetAndTypeRoundTrip(t *testing.T) {
	s := newTestMarginService(t)
	at := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	require.NoError(t, insertType(s.DB, types.MarginInterest{
		Exchange:     types.ExchangeBinance,
		Asset:        "BNB",
		RawAsset:     "BTC",
		InterestType: "PERIODIC_CONVERTED",
		Principle:    Number("0.5"),
		Interest:     Number("0.000003"),
		InterestRate: Number("0.00024"),
		Session:      "a",
		PriceInUSDT:  Number(600),
		Time:         types.Time(at),
	}, false))

	rows, err := s.QueryInterests(context.Background(), QueryMarginInterestOptions{
		Exchange: types.ExchangeBinance, Session: "a", Since: at, Until: at.Add(time.Hour),
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "BTC", rows[0].RawAsset)
	assert.Equal(t, "PERIODIC_CONVERTED", rows[0].InterestType)
	assert.Equal(t, "BTC", rows[0].BorrowedAsset())
}

func TestSyncMarginHistory_SyncsTheFeeAssetInterests(t *testing.T) {
	t.Setenv("USE_MARKETS_CACHE_IN_MEMORY", "true")
	charged := time.Now().UTC().Truncate(time.Hour).Add(-3 * time.Hour)

	t.Run("fee asset not listed", func(t *testing.T) {
		syncer := &SyncService{MarginService: newTestMarginService(t)}
		// a row the fake cannot price: the fee asset sync warns, it does not fail
		ex := &fakeMarginExchange{interests: []types.MarginInterest{
			{Exchange: types.ExchangeBinance, Asset: "BNB", RawAsset: "BTC", InterestType: types.MarginInterestTypePeriodicConverted,
				Principle: Number(1), Interest: Number("0.0001"), Time: types.Time(charged)},
		}}

		require.NoError(t, syncer.SyncMarginHistory(context.Background(), ex, "a", charged.Add(-time.Hour), "BTC"))
		assert.Contains(t, ex.queriedInterestAssets(), "BTC")
		assert.Contains(t, ex.queriedInterestAssets(), "BNB")
	})

	t.Run("fee asset listed", func(t *testing.T) {
		syncer := &SyncService{MarginService: newTestMarginService(t)}
		ex := &fakeMarginExchange{}

		require.NoError(t, syncer.SyncMarginHistory(context.Background(), ex, "a", charged.Add(-time.Hour), "BNB"))
		liquidations := ex.liquidations
		queried := ex.queriedInterestAssets()
		assert.NotEmpty(t, queried)
		for _, asset := range queried {
			assert.Equal(t, "BNB", asset)
		}

		// BNB went through Sync once, not again through the interest-only sync:
		// a plain Sync of another session makes as many queries
		require.NoError(t, syncer.MarginService.Sync(context.Background(), ex, "b", "BNB", charged.Add(-time.Hour)))
		assert.Len(t, ex.queriedInterestAssets(), 2*len(queried))
		assert.Equal(t, 2*liquidations, ex.liquidations)
	})

	t.Run("no fee asset", func(t *testing.T) {
		syncer := &SyncService{MarginService: newTestMarginService(t)}
		ex := &maxMarginExchange{fakeMarginExchange: &fakeMarginExchange{}}

		require.NoError(t, syncer.SyncMarginHistory(context.Background(), ex, "a", charged.Add(-time.Hour), "BTC"))
		queried := ex.queriedInterestAssets()
		assert.NotEmpty(t, queried)
		for _, asset := range queried {
			assert.Equal(t, "BTC", asset)
		}
	})
}

// maxMarginExchange is fakeMarginExchange named MAX, which has no fee asset
type maxMarginExchange struct{ *fakeMarginExchange }

func (maxMarginExchange) Name() types.ExchangeName { return types.ExchangeMax }

func TestMarginInterestFeeAsset(t *testing.T) {
	assert.Equal(t, "BNB", MarginInterestFeeAsset(types.ExchangeBinance))
	assert.Equal(t, "", MarginInterestFeeAsset(types.ExchangeMax))
}
