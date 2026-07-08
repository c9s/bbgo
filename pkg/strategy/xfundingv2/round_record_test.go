package xfundingv2

import (
	"context"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

const createRoundTableSQL = `
CREATE TABLE xfundingv2_closed_rounds (
	gid                    INTEGER PRIMARY KEY AUTOINCREMENT,
	id                     TEXT NOT NULL,
	strategy_instance_id   TEXT NOT NULL,
	spot_symbol            TEXT NOT NULL,
	futures_symbol         TEXT NOT NULL,
	spot_exchange          TEXT NOT NULL,
	futures_exchange       TEXT NOT NULL,
	direction              TEXT NOT NULL,
	collateral_asset       TEXT NOT NULL,
	leverage               DECIMAL,
	triggered_funding_rate DECIMAL,
	annualized_rate        DECIMAL,
	funding_income         DECIMAL,
	spot_pnl               DECIMAL,
	spot_net_pnl           DECIMAL,
	futures_pnl            DECIMAL,
	futures_net_pnl        DECIMAL,
	net_pnl                DECIMAL,
	num_holding_intervals  INTEGER,
	start_time             DATETIME,
	ready_time             DATETIME,
	closing_time           DATETIME,
	closed_time            DATETIME,
	inserted_at            DEFAULT CURRENT_TIMESTAMP NOT NULL
);`

const createFundingFeeTableSQL = `
CREATE TABLE xfundingv2_funding_fees (
	gid       INTEGER PRIMARY KEY AUTOINCREMENT,
	round_id  TEXT NOT NULL,
	asset     TEXT NOT NULL,
	amount    DECIMAL,
	txn       INTEGER,
	time      DATETIME
);`

func TestRoundInsertService(t *testing.T) {
	db, err := sqlx.Connect("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(createRoundTableSQL)
	require.NoError(t, err)
	_, err = db.Exec(createFundingFeeTableSQL)
	require.NoError(t, err)

	svc := NewRoundInsertService(context.Background(), db, "xfundingv2-test-instance")

	const roundID = "11111111-2222-3333-4444-555555555555"
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
		StartTime:            time.Now().Add(-24 * time.Hour),
		ReadyTime:            time.Now().Add(-23 * time.Hour),
		ClosingTime:          time.Now().Add(-2 * time.Hour),
		ClosedTime:           time.Now().Add(-time.Hour),
	}

	fees := []FundingFeeRecord{
		{RoundID: roundID, Asset: "USDT", Amount: fixedpoint.NewFromFloat(6.25), Txn: 1001, Time: time.Now().Add(-16 * time.Hour)},
		{RoundID: roundID, Asset: "USDT", Amount: fixedpoint.NewFromFloat(6.25), Txn: 1002, Time: time.Now().Add(-8 * time.Hour)},
	}

	err = svc.insert(record, fees)
	require.NoError(t, err)

	var roundCount int
	require.NoError(t, db.Get(&roundCount, "SELECT COUNT(*) FROM xfundingv2_closed_rounds"))
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
	db, err := sqlx.Connect("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(createRoundTableSQL)
	require.NoError(t, err)
	_, err = db.Exec(createFundingFeeTableSQL)
	require.NoError(t, err)

	svc := NewRoundInsertService(context.Background(), db, "xfundingv2-test-instance")

	err = svc.insert(ClosedRoundRecord{
		ID:              "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		InstanceID:      svc.instanceID,
		SpotSymbol:      "ETHUSDT",
		FuturesSymbol:   "ETHUSDT",
		SpotExchange:    "binance",
		FuturesExchange: "binance",
		Direction:       "long",
		CollateralAsset: "USDT",
	}, nil)
	require.NoError(t, err)

	var feeCount int
	require.NoError(t, db.Get(&feeCount, "SELECT COUNT(*) FROM xfundingv2_funding_fees"))
	assert.Equal(t, 0, feeCount)
}
