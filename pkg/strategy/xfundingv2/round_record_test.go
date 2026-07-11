package xfundingv2

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// register the DB drivers that DB_DRIVER may select.
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// newTestInsertDB connects to the database configured via the DB_DRIVER and DB_DSN
// environment variables. The migrations are assumed to be already applied, so the
// test only exercises the insert service. When either variable is missing the test
// is skipped so that CI without a database stays green.
func newTestInsertDB(t *testing.T) *sqlx.DB {
	t.Helper()

	driver, ok := os.LookupEnv("DB_DRIVER")
	if !ok || driver == "" {
		t.Skip("DB_DRIVER is not set, skipping round insert service DB test")
	}

	dsn, ok := os.LookupEnv("DB_DSN")
	if !ok || dsn == "" {
		t.Skip("DB_DSN is not set, skipping round insert service DB test")
	}

	db, err := sqlx.Connect(driver, dsn)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

// cleanupRound removes the round record and its funding-fee records that were
// created during a test, keyed by the round's own id. Both sqlite3 and mysql use
// the `?` placeholder so the same statements work for either driver.
func cleanupRound(t *testing.T, db *sqlx.DB, roundID string) {
	t.Cleanup(func() {
		if _, err := db.Exec("DELETE FROM xfundingv2_funding_fees WHERE round_id = ?", roundID); err != nil {
			t.Logf("cleanup: failed to delete funding fees for round %s: %v", roundID, err)
		}
		if _, err := db.Exec("DELETE FROM xfundingv2_closed_rounds WHERE id = ?", roundID); err != nil {
			t.Logf("cleanup: failed to delete closed round %s: %v", roundID, err)
		}
	})
}

func TestRoundInsertService(t *testing.T) {
	db := newTestInsertDB(t)

	svc := NewRoundInsertService(context.Background(), db, "xfundingv2-test-instance")

	// use a fresh id per run so the assertions and cleanup never collide with
	// leftover rows from earlier runs or other data in a shared database.
	roundID := uuid.NewString()
	cleanupRound(t, db, roundID)

	readyTime := time.Now().Add(-23 * time.Hour)
	record := ClosedRoundRecord{
		ID:                   roundID,
		InstanceID:           svc.instanceID,
		SpotSymbol:           "BTCUSDT",
		FuturesSymbol:        "BTCUSDT",
		SpotExchange:         "binance",
		FuturesExchange:      "binance",
		Direction:            "short",
		CollateralAsset:      "BTC",
		Leverage:             fixedpoint.NewFromInt(3),
		TriggeredFundingRate: fixedpoint.NewFromFloat(0.0003),
		AnnualizedRate:       fixedpoint.NewFromFloat(0.32),
		FundingIncome:        fixedpoint.NewFromFloat(12.5),
		SpotPnL:              fixedpoint.NewFromFloat(-1.2),
		SpotNetPnL:           fixedpoint.NewFromFloat(-1.5),
		FuturesPnL:           fixedpoint.NewFromFloat(0.8),
		FuturesNetPnL:        fixedpoint.NewFromFloat(0.5),
		NetPnL:               fixedpoint.NewFromFloat(11.5),
		NumHoldingIntervals:  3,
		StartAt:              time.Now().Add(-24 * time.Hour),
		ReadyAt:              &readyTime,
		ClosingAt:            time.Now().Add(-2 * time.Hour),
		ClosedAt:             time.Now().Add(-time.Hour),
	}

	fees := []FundingFeeRecord{
		{RoundID: roundID, Asset: "USDT", Amount: fixedpoint.NewFromFloat(6.25), Txn: 1001, Time: time.Now().Add(-16 * time.Hour)},
		{RoundID: roundID, Asset: "USDT", Amount: fixedpoint.NewFromFloat(6.25), Txn: 1002, Time: time.Now().Add(-8 * time.Hour)},
	}

	err := svc.insertClosedRound(record, fees)
	require.NoError(t, err)

	var roundCount int
	require.NoError(t, db.Get(&roundCount, "SELECT COUNT(*) FROM xfundingv2_closed_rounds WHERE id = ?", roundID))
	assert.Equal(t, 1, roundCount)

	var feeCount int
	require.NoError(t, db.Get(&feeCount, "SELECT COUNT(*) FROM xfundingv2_funding_fees WHERE round_id = ?", roundID))
	assert.Equal(t, len(fees), feeCount)

	// the round row should carry the values we inserted
	var gotRoundID, gotSymbol, gotDirection string
	require.NoError(t, db.QueryRow(
		"SELECT id, spot_symbol, direction FROM xfundingv2_closed_rounds WHERE id = ?", roundID,
	).Scan(&gotRoundID, &gotSymbol, &gotDirection))
	assert.Equal(t, roundID, gotRoundID)
	assert.Equal(t, "BTCUSDT", gotSymbol)
	assert.Equal(t, "short", gotDirection)
}

func TestRoundInsertService_NoFundingFees(t *testing.T) {
	db := newTestInsertDB(t)

	svc := NewRoundInsertService(context.Background(), db, "xfundingv2-test-instance")

	roundID := uuid.NewString()
	cleanupRound(t, db, roundID)

	// a closed round always has start/closing/closed times; ready_time is left unset
	// to exercise the nullable-ready_time path (a round closed before it became ready).
	err := svc.insertClosedRound(ClosedRoundRecord{
		ID:              roundID,
		InstanceID:      svc.instanceID,
		SpotSymbol:      "ETHUSDT",
		FuturesSymbol:   "ETHUSDT",
		SpotExchange:    "binance",
		FuturesExchange: "binance",
		Direction:       "long",
		CollateralAsset: "USDT",
		StartAt:         time.Now().Add(-24 * time.Hour),
		ClosingAt:       time.Now().Add(-2 * time.Hour),
		ClosedAt:        time.Now().Add(-time.Hour),
	}, nil)
	require.NoError(t, err)

	var roundCount int
	require.NoError(t, db.Get(&roundCount, "SELECT COUNT(*) FROM xfundingv2_closed_rounds WHERE id = ?", roundID))
	assert.Equal(t, 1, roundCount)

	var feeCount int
	require.NoError(t, db.Get(&feeCount, "SELECT COUNT(*) FROM xfundingv2_funding_fees WHERE round_id = ?", roundID))
	assert.Equal(t, 0, feeCount)

	// ready_time must be persisted as NULL when the round never became ready
	var readyAt sql.NullTime
	require.NoError(t, db.QueryRow(
		"SELECT ready_at FROM xfundingv2_closed_rounds WHERE id = ?", roundID,
	).Scan(&readyAt))
	assert.False(t, readyAt.Valid, "ready_time should be NULL when unset")
}
