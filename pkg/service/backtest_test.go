package service

import (
	"context"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/types"
)

func TestBacktestService(t *testing.T) {
	db, err := prepareDB(t)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()

	ctx := context.Background()
	dbx := sqlx.NewDb(db.DB, "sqlite3")

	ex, err := exchange.NewPublic(types.ExchangeBinance)
	assert.NoError(t, err)

	service := &BacktestService{DB: dbx}

	symbol := "BTCUSDT"
	now := time.Now()
	startTime1 := now.AddDate(0, 0, -6).Truncate(time.Hour)
	endTime1 := now.AddDate(0, 0, -5).Truncate(time.Hour)

	startTime2 := now.AddDate(0, 0, -4).Truncate(time.Hour)
	endTime2 := now.AddDate(0, 0, -3).Truncate(time.Hour)

	// kline query is exclusive
	err = service.SyncKLineByInterval(ctx, ex, symbol, types.Interval1h, startTime1.Add(-time.Second), endTime1.Add(time.Second))
	assert.NoError(t, err)

	err = service.SyncKLineByInterval(ctx, ex, symbol, types.Interval1h, startTime2.Add(-time.Second), endTime2.Add(time.Second))
	assert.NoError(t, err)

	t1, t2, err := service.QueryExistingDataRange(ctx, ex, symbol, types.Interval1h)
	if assert.NoError(t, err) {
		assert.Equal(t, startTime1, t1.Time(), "start time point should match")
		assert.Equal(t, endTime2, t2.Time(), "end time point should match")
	}

	timeRanges, err := service.FindMissingTimeRanges(ctx, ex, symbol, types.Interval1h, startTime1, endTime2)
	if assert.NoError(t, err) {
		assert.NotEmpty(t, timeRanges)
		assert.Len(t, timeRanges, 1, "should find one missing time range")
		t.Logf("found timeRanges: %+v", timeRanges)
	}
}
