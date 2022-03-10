package service

import (
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestPositionService(t *testing.T) {
	db, err := prepareDB(t)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()

	xdb := sqlx.NewDb(db.DB, "sqlite3")
	service := &PositionService{DB: xdb}

	err = service.Insert(&types.Position{
		Symbol:        "BTCUSDT",
		BaseCurrency:  "BTC",
		QuoteCurrency: "USDT",
		AverageCost:   fixedpoint.NewFromFloat(44000),
		ChangedAt:     time.Now(),
	}, types.Trade{}, fixedpoint.NewFromFloat(10.9))
	assert.NoError(t, err)

}
