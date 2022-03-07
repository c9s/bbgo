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
		Symbol:        "BTCUSDT",
		BaseCurrency:  "BTC",
		QuoteCurrency: "USDT",
		AverageCost:   fixedpoint.NewFromFloat(44000),
		Profit:        fixedpoint.NewFromFloat(1.01),
		NetProfit:     fixedpoint.NewFromFloat(0.98),
		TradeID:       99,
		Side:          types.SideTypeSell,
		Price:         fixedpoint.NewFromFloat(44300),
		Quantity:      fixedpoint.NewFromFloat(0.001),
		QuoteQuantity: fixedpoint.NewFromFloat(44.0),
		Exchange:      types.ExchangeMax,
		TradedAt:      time.Now(),
	})
	assert.NoError(t, err)
}
