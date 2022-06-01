package service

import (
	"context"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/testutil"
)

func TestMarginService(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "BINANCE")
	if !ok {
		t.SkipNow()
		return
	}

	ex := binance.New(key, secret)
	ex.MarginSettings.IsMargin = true
	ex.MarginSettings.IsIsolatedMargin = true
	ex.MarginSettings.IsolatedMarginSymbol = "DOTUSDT"

	logrus.SetLevel(logrus.ErrorLevel)
	db, err := prepareDB(t)

	assert.NoError(t, err)

	if err != nil {
		t.Fail()
		return
	}

	defer db.Close()

	ctx := context.Background()

	dbx := sqlx.NewDb(db.DB, "sqlite3")
	service := &MarginService{DB: dbx}

	logrus.SetLevel(logrus.DebugLevel)
	err = service.Sync(ctx, ex, "USDT", time.Date(2022, time.February, 1, 0, 0, 0, 0, time.UTC))
	assert.NoError(t, err)

	// sync second time to ensure that we can query records
	err = service.Sync(ctx, ex, "USDT", time.Date(2022, time.February, 1, 0, 0, 0, 0, time.UTC))
	assert.NoError(t, err)
}
