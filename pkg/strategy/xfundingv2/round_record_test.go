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

// cleanupSnapshot removes the active round snapshot row created during a test,
// keyed by the round's own id.
func cleanupSnapshot(t *testing.T, db *sqlx.DB, roundID string) {
	t.Cleanup(func() {
		if _, err := db.Exec("DELETE FROM xfundingv2_round_snapshots WHERE id = ?", roundID); err != nil {
			t.Logf("cleanup: failed to delete round snapshot %s: %v", roundID, err)
		}
	})
}

func TestRoundInsertService_ClosedRound(t *testing.T) {
	db := newTestInsertDB(t)

	svc := NewRoundInsertService(context.Background(), db, "xfundingv2-test-instance")

	t.Run("insert closed round with funding fees", func(t *testing.T) {
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

		fees := []FundingFee{
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
	})

	t.Run("no funding fees", func(t *testing.T) {
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
	})
}

func TestRoundInsertService_ActiveRoundSnapshot(t *testing.T) {
	db := newTestInsertDB(t)

	svc := NewRoundInsertService(context.Background(), db, "xfundingv2-test-instance")

	roundID := uuid.NewString()
	// the active-round path writes to both the snapshot table and the shared
	// funding-fee table, and the transition assertion below also writes a closed
	// round, so register both cleanups.
	cleanupSnapshot(t, db, roundID)
	cleanupRound(t, db, roundID)

	record := ActiveRoundRecord{
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
		FundingIncome:        fixedpoint.NewFromFloat(4.2),
		SpotPosition:         fixedpoint.NewFromFloat(1.0),
		FuturesPosition:      fixedpoint.NewFromFloat(-1.0),
		SpotAverageCost:      fixedpoint.NewFromFloat(40000),
		FuturesAverageCost:   fixedpoint.NewFromFloat(40100),
		SpotPnL:              fixedpoint.NewFromFloat(-0.5),
		SpotNetPnL:           fixedpoint.NewFromFloat(-0.8),
		FuturesPnL:           fixedpoint.NewFromFloat(0.3),
		FuturesNetPnL:        fixedpoint.NewFromFloat(0.1),
		NetPnL:               fixedpoint.NewFromFloat(3.5),
		UnrealizedSpotPnL:    fixedpoint.NewFromFloat(1000),
		TotalSpotNetPnL:      fixedpoint.NewFromFloat(999.2),
		UnrealizedFuturesPnL: fixedpoint.NewFromFloat(100),
		TotalFuturesNetPnL:   fixedpoint.NewFromFloat(100.1),
		TotalNetPnL:          fixedpoint.NewFromFloat(1103.5),
	}

	fees := []FundingFee{
		{RoundID: roundID, Asset: "USDT", Amount: fixedpoint.NewFromFloat(2.1), Txn: 2001, Time: time.Now().Add(-16 * time.Hour)},
		{RoundID: roundID, Asset: "USDT", Amount: fixedpoint.NewFromFloat(2.1), Txn: 2002, Time: time.Now().Add(-8 * time.Hour)},
	}

	// first insert creates the snapshot row and the funding-fee rows
	require.NoError(t, svc.insertActiveRound(record, fees))

	var count int
	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM xfundingv2_round_snapshots WHERE id = ?", roundID))
	assert.Equal(t, 1, count)

	var feeCount int
	require.NoError(t, db.Get(&feeCount, "SELECT COUNT(*) FROM xfundingv2_funding_fees WHERE round_id = ?", roundID))
	assert.Equal(t, len(fees), feeCount, "each funding transaction should be a distinct row")

	// a second snapshot for the same round should create a new record
	record.TotalNetPnL = fixedpoint.NewFromFloat(2000.75)
	record.NetPnL = fixedpoint.NewFromFloat(4.0)
	// the funding fee amount is revised on re-sync; the upsert must update the row
	fees[0].Amount = fixedpoint.NewFromFloat(3.3)
	require.NoError(t, svc.insertActiveRound(record, fees))

	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM xfundingv2_round_snapshots WHERE id = ?", roundID))
	assert.Equal(t, 2, count, "snapshotting the same round must create a new row")

	require.NoError(t, db.Get(&feeCount, "SELECT COUNT(*) FROM xfundingv2_funding_fees WHERE round_id = ?", roundID))
	assert.Equal(t, len(fees), feeCount, "re-snapshotting must upsert funding fees, not duplicate them")

	// the lastest row should carry the latest values
	var gotTotalNetPnL, gotNetPnL fixedpoint.Value
	require.NoError(t, db.QueryRow(
		"SELECT total_net_pnl, net_pnl FROM xfundingv2_round_snapshots WHERE id = ? ORDER BY gid DESC LIMIT 1", roundID,
	).Scan(&gotTotalNetPnL, &gotNetPnL))
	assert.Equal(t, 0, gotTotalNetPnL.Compare(fixedpoint.NewFromFloat(2000.75)),
		"total_net_pnl should be updated, got: %s", gotTotalNetPnL)
	assert.Equal(t, 0, gotNetPnL.Compare(fixedpoint.NewFromFloat(4.0)),
		"net_pnl should be updated, got: %s", gotNetPnL)

	// the revised funding fee amount should be reflected in the upserted row
	var gotFeeAmount fixedpoint.Value
	require.NoError(t, db.QueryRow(
		"SELECT amount FROM xfundingv2_funding_fees WHERE round_id = ? AND txn = ?", roundID, int64(2001),
	).Scan(&gotFeeAmount))
	assert.Equal(t, 0, gotFeeAmount.Compare(fixedpoint.NewFromFloat(3.3)),
		"funding fee amount should be updated, got: %s", gotFeeAmount)

	// when the round is later closed, insertClosedRound persists the same funding
	// fees again; the upsert must not collide with the rows written while active.
	require.NoError(t, svc.insertClosedRound(ClosedRoundRecord{
		ID:              roundID,
		InstanceID:      svc.instanceID,
		SpotSymbol:      "BTCUSDT",
		FuturesSymbol:   "BTCUSDT",
		SpotExchange:    "binance",
		FuturesExchange: "binance",
		Direction:       "short",
		CollateralAsset: "BTC",
		StartAt:         time.Now().Add(-24 * time.Hour),
		ClosingAt:       time.Now().Add(-2 * time.Hour),
		ClosedAt:        time.Now().Add(-time.Hour),
	}, fees))

	require.NoError(t, db.Get(&feeCount, "SELECT COUNT(*) FROM xfundingv2_funding_fees WHERE round_id = ?", roundID))
	assert.Equal(t, len(fees), feeCount, "closing the round must upsert the same funding fees, not duplicate them")
}
