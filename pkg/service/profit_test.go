package service

import (
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestProfitService(t *testing.T) {
	db, err := prepareDB(t)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()

	xdb := sqlx.NewDb(db.DB, "sqlite3")
	service := &ProfitService{DB: xdb}

	err = service.Insert(types.Profit{
		Symbol: "BTCUSDT",
		BaseCurrency:  "BTC",
		QuoteCurrency: "USDT",
		Profit:        fixedpoint.NewFromFloat(1.01),
		NetProfit:     fixedpoint.NewFromFloat(0.98),
		AverageCost:   fixedpoint.NewFromFloat(44000),
		TradeID:       99,
		Price:         fixedpoint.NewFromFloat(44300),
		Quantity:      fixedpoint.NewFromFloat(0.001),
		TradeAmount:   fixedpoint.NewFromFloat(44.0),
		Exchange:      types.ExchangeMax,
		Time:          time.Now(),
	})
	assert.NoError(t, err)
}
