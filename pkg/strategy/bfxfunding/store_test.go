package bfxfunding

import (
	"context"
	"testing"
	"time"

	"github.com/c9s/rockhopper/v2"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prepareTestDB(t *testing.T) *sqlx.DB {
	ctx := context.Background()

	dialect, err := rockhopper.LoadDialect("sqlite3")
	require.NoError(t, err)

	db, err := rockhopper.Open("sqlite3", dialect, ":memory:", rockhopper.TableName)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, db.Touch(ctx))

	loader := &rockhopper.SqlMigrationLoader{}
	migrations, err := loader.Load("migrations/sqlite3")
	require.NoError(t, err)

	migrations = migrations.Sort().Connect()
	require.NotEmpty(t, migrations)
	require.NoError(t, rockhopper.Up(ctx, db, migrations.Head(), 0))

	return sqlx.NewDb(db.DB, "sqlite3")
}

func TestTradeStore(t *testing.T) {
	ctx := context.Background()
	store := NewTradeStore(prepareTestDB(t))
	require.NoError(t, store.Check(ctx))

	last, ids, err := store.LastTrade(ctx, "fUST")
	require.NoError(t, err)
	assert.True(t, last.IsZero())
	assert.Empty(t, ids)

	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	trades := []FundingTrade{
		{ID: 1, Time: t0, Amount: 100, Rate: 0.0001, Period: 2},
		{ID: 2, Time: t0.Add(time.Second), Amount: 200, Rate: 0.00012345, Period: 30},
		{ID: 3, Time: t0.Add(time.Second), Amount: 300, Rate: 0.0002, Period: 120},
	}

	require.NoError(t, store.Insert(ctx, "fUST", trades))

	// the duplicates are ignored
	require.NoError(t, store.Insert(ctx, "fUST", trades[1:]))
	require.NoError(t, store.Insert(ctx, "fBTC", trades[:1]))

	last, ids, err = store.LastTrade(ctx, "fUST")
	require.NoError(t, err)
	assert.True(t, last.Equal(t0.Add(time.Second)))
	assert.ElementsMatch(t, []int64{2, 3}, ids)

	all, err := store.Query(ctx, "fUST", t0)
	require.NoError(t, err)
	require.Len(t, all, 3)
	assert.Equal(t, int64(1), all[0].ID)
	assert.True(t, all[0].Time.Equal(t0))
	assert.Equal(t, 0.00012345, all[1].Rate)
	assert.Equal(t, 30, all[1].Period)
	assert.Equal(t, 300.0, all[2].Amount)

	recent, err := store.Query(ctx, "fUST", t0.Add(time.Second))
	require.NoError(t, err)
	assert.Len(t, recent, 2)

	// insert more than one batch
	var many []FundingTrade
	for i := range insertBatchSize*2 + 1 {
		many = append(many, FundingTrade{ID: int64(100 + i), Time: t0.Add(time.Duration(i) * time.Minute), Amount: 1, Rate: 0.0001, Period: 2})
	}

	require.NoError(t, store.Insert(ctx, "fETH", many))
	stored, err := store.Query(ctx, "fETH", t0)
	require.NoError(t, err)
	assert.Len(t, stored, len(many))
}
