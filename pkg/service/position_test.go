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

	defer func() {
		err := db.Close()
		assert.NoError(t, err)
	}()

	xdb := sqlx.NewDb(db.DB, "sqlite3")
	service := &PositionService{DB: xdb}

	t.Run("minimal fields", func(t *testing.T) {
		err = service.Insert(&types.Position{
			Symbol:        "BTCUSDT",
			BaseCurrency:  "BTC",
			QuoteCurrency: "USDT",
			AverageCost:   fixedpoint.NewFromFloat(44000),
			ChangedAt:     time.Now(),
		}, types.Trade{
			Time: types.Time(time.Now()),
		}, fixedpoint.Zero)
		assert.NoError(t, err)
	})

	t.Run("full fields", func(t *testing.T) {
		err = service.Insert(&types.Position{
			Symbol:             "BTCUSDT",
			BaseCurrency:       "BTC",
			QuoteCurrency:      "USDT",
			AverageCost:        fixedpoint.NewFromFloat(44000),
			Base:               fixedpoint.NewFromFloat(0.1),
			Quote:              fixedpoint.NewFromFloat(-44000.0),
			ChangedAt:          time.Now(),
			Strategy:           "bollmaker",
			StrategyInstanceID: "bollmaker-BTCUSDT-1m",
		}, types.Trade{
			ID:       9,
			Exchange: types.ExchangeBinance,
			Side:     types.SideTypeSell,
			Time:     types.Time(time.Now()),
		}, fixedpoint.NewFromFloat(10.9))
		assert.NoError(t, err)
	})

}
