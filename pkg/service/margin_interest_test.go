package service

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
func insertInterest(t *testing.T, s *MarginService, session, asset string, at time.Time, interest string) {
	t.Helper()
	err := insertMarginInterest(s.DB, types.MarginInterest{
		Exchange:  types.ExchangeBinance,
		Asset:     asset,
		Principal: Number(1000),
		Interest:  Number(interest),
		Session:   session,
		Time:      types.Time(at.UTC()),
	})
	require.NoError(t, err)
}

func txID(id uint64) *uint64 { return &id }

func TestMarginService_QueryInterests(t *testing.T) {
	ctx := context.Background()
	s := newTestMarginService(t)

	nov1 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	dec1 := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	plus8 := time.FixedZone("+8", 8*3600)

	for _, session := range []string{"a", "b", ""} {
		insertInterest(t, s, session, "USDT", nov1, "0.00417")
		insertInterest(t, s, session, "BTC", mid, "0.00000209")
		insertInterest(t, s, session, "USDT", dec1, "0.00417")
	}
	// local times on either side of the month boundary
	insertInterest(t, s, "a", "USDT", time.Date(2026, 10, 31, 20, 0, 0, 0, time.UTC).In(plus8), "0.001")
	insertInterest(t, s, "a", "USDT", time.Date(2026, 11, 30, 20, 0, 0, 0, time.UTC).In(plus8), "0.002")

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
	insertInterest(t, s, "a", "USDT", h10, "0.00417")
	// a prorated borrow-time charge inside the 10:00 hour
	insertInterest(t, s, "a", "USDT", h10.Add(25*time.Minute), "0.001")
	insertInterest(t, s, "a", "USDT", h10.Add(time.Hour), "0.00417")

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
	insertInterest(t, s, "a", "USDT", at, "0.00417")
	insertInterest(t, s, "", "USDT", at.Add(time.Hour), "0.00417")

	records, err := selectAndScanType(ctx, s.DB, SelectLastMarginInterests(types.ExchangeBinance, "a", "USDT", 100), types.MarginInterest{})
	require.NoError(t, err)
	assert.Len(t, records.([]types.MarginInterest), 1)

	records, err = selectAndScanType(ctx, s.DB, SelectLastMarginInterests(types.ExchangeBinance, "new", "USDT", 100), types.MarginInterest{})
	require.NoError(t, err)
	assert.Empty(t, records.([]types.MarginInterest))
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

func TestInterestSync_FetchesARowPublishedLate(t *testing.T) {
	ctx := context.Background()
	s := newTestMarginService(t)

	h10 := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	// the borrow-time row at 10:30 was synced before the 10:00 charge was published
	insertInterest(t, s, "a", "USDT", h10.Add(30*time.Minute), "0.0005")

	task := s.interestSyncTask(nil, types.ExchangeBinance, "a", "USDT")
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

func reflectValueOf(v interface{}) reflect.Value {
	return reflect.ValueOf(v)
}

// fakeMarginExchange is a cross-margin exchange that returns the same interests for every query
type fakeMarginExchange struct {
	types.Exchange
	interests    []types.MarginInterest
	liquidations int
}

func (f *fakeMarginExchange) Name() types.ExchangeName { return types.ExchangeBinance }
func (f *fakeMarginExchange) UseMargin()               {}
func (f *fakeMarginExchange) UseIsolatedMargin(string) {}
func (f *fakeMarginExchange) GetMarginSettings() types.MarginSettings {
	return types.MarginSettings{IsMargin: true}
}

func (f *fakeMarginExchange) QueryLoanHistory(context.Context, string, *time.Time, *time.Time) ([]types.MarginLoan, error) {
	return nil, nil
}

func (f *fakeMarginExchange) QueryRepayHistory(context.Context, string, *time.Time, *time.Time) ([]types.MarginRepay, error) {
	return nil, nil
}

func (f *fakeMarginExchange) QueryInterestHistory(context.Context, string, *time.Time, *time.Time) ([]types.MarginInterest, error) {
	return f.interests, nil
}

func (f *fakeMarginExchange) QueryLiquidationHistory(context.Context, *time.Time, *time.Time) ([]types.MarginLiquidation, error) {
	f.liquidations++
	return nil, nil
}

func TestMarginService_SyncStoresEveryChargeOfAnHour(t *testing.T) {
	ctx := context.Background()
	s := newTestMarginService(t)
	h10 := time.Now().UTC().Truncate(time.Hour).Add(-3 * time.Hour)

	// the shape Binance returns with BNB deduction on: several charges share the
	// hour, the paid asset and even the principal, only txId tells them apart
	charge := func(id uint64, at time.Time, rawAsset, typ, principal, interest string) types.MarginInterest {
		return types.MarginInterest{
			Exchange: types.ExchangeBinance, TxID: txID(id), Asset: "BNB", RawAsset: rawAsset, InterestType: typ,
			Principal: Number(principal), Interest: Number(interest), InterestRate: Number("0.0001"), Time: types.Time(at),
		}
	}
	ex := &fakeMarginExchange{interests: []types.MarginInterest{
		charge(1793934000000000001, h10, "USDT", "PERIODIC_CONVERTED", "1000", "0.00001"),
		charge(1793934000000000002, h10, "USDT", "ON_BORROW_CONVERTED", "1000", "0.00001"),
		charge(1793934000000000003, h10, "USDT", "PERIODIC_CONVERTED", "1000", "0.00001"),
		charge(1793934000000000004, h10, "BTC", "PERIODIC_CONVERTED", "0.5", "0.00002"),
		charge(1793934000000000005, h10.Add(time.Hour), "USDT", "PERIODIC_CONVERTED", "1000", "0.00001"),
	}}

	require.NoError(t, s.Sync(ctx, ex, "a", "BNB", h10.Add(-time.Hour)))
	assert.Positive(t, ex.liquidations, "the tasks after the interests still run")

	query := func(session string) []types.MarginInterest {
		rows, err := s.QueryInterests(ctx, QueryMarginInterestOptions{
			Exchange: types.ExchangeBinance, Session: session, Since: h10, Until: h10.Add(2 * time.Hour),
		})
		require.NoError(t, err)
		return rows
	}

	rows := query("a")
	require.Len(t, rows, 5)

	// stored as returned, plus the session
	got := map[uint64]types.MarginInterest{}
	for _, row := range rows {
		require.NotNil(t, row.TxID)
		got[*row.TxID] = row
	}
	row := got[1793934000000000002]
	assert.Equal(t, "a", row.Session)
	assert.Equal(t, "BNB", row.Asset)
	assert.Equal(t, "USDT", row.RawAsset)
	assert.Equal(t, "ON_BORROW_CONVERTED", row.InterestType)
	assert.Equal(t, "1000", row.Principal.String())
	assert.Equal(t, "0.00001", row.Interest.String())
	assert.Equal(t, h10, row.Time.Time().UTC())

	// syncing again stores nothing twice
	require.NoError(t, s.SyncInterests(ctx, ex, "a", "BNB", h10.Add(-time.Hour)))
	require.NoError(t, s.Sync(ctx, ex, "a", "BNB", h10.Add(-time.Hour)))
	assert.Len(t, query("a"), 5)

	// another session keeps its own copy of the same charges
	require.NoError(t, s.SyncInterests(ctx, ex, "b", "BNB", h10.Add(-time.Hour)))
	assert.Len(t, query("b"), 5)
	assert.Len(t, query("a"), 5)
}

func TestMarginInterests_UniqueTxIDPerSession(t *testing.T) {
	s := newTestMarginService(t)
	at := types.Time(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	insert := func(session string, id *uint64) error {
		return insertMarginInterest(s.DB, types.MarginInterest{
			Exchange: types.ExchangeBinance, Session: session, TxID: id, Asset: "USDT", Time: at,
		})
	}

	require.NoError(t, insert("a", txID(1)))
	assert.Error(t, insert("a", txID(1)), "the same charge is stored once per session")
	assert.NoError(t, insert("b", txID(1)))
	assert.NoError(t, insert("a", txID(2)))

	// rows without a tx id, e.g. synced before it was stored, never collide
	assert.NoError(t, insert("", nil))
	assert.NoError(t, insert("", nil))

	// a large txId is stored as is
	assert.NoError(t, insert("a", txID(1<<62+5)))
	assert.Error(t, insert("a", txID(1<<62+5)))
}

func TestMarginInterest_Key(t *testing.T) {
	at := types.Time(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	a := types.MarginInterest{TxID: txID(1), Asset: "BNB", Time: at}
	b := types.MarginInterest{TxID: txID(2), Asset: "BNB", Time: at}
	assert.NotEqual(t, a.Key(), b.Key(), "charges of the same hour and asset")

	// without a tx id, the asset, isolated symbol and time
	legacy := types.MarginInterest{Asset: "BNB", Time: at}
	assert.Equal(t, legacy.Key(), types.MarginInterest{Asset: "BNB", Time: at}.Key())
	assert.NotEqual(t, legacy.Key(), types.MarginInterest{Asset: "BNB", IsolatedSymbol: "BNBUSDT", Time: at}.Key())
	assert.NotEqual(t, legacy.Key(), a.Key())
}
