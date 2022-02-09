package service

import (
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestDepositService(t *testing.T) {
	db, err := prepareDB(t)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()

	xdb := sqlx.NewDb(db.DB, "sqlite3")
	service := &DepositService{DB: xdb}

	err = service.Insert(types.Deposit{
		Exchange:      types.ExchangeMax,
		Time:          types.Time(time.Now()),
		Amount:        fixedpoint.NewFromFloat(0.001),
		Asset:         "BTC",
		Address:       "test",
		TransactionID: "02",
		Status:        types.DepositSuccess,
	})
	assert.NoError(t, err)

	deposits, err := service.Query(types.ExchangeMax)
	assert.NoError(t, err)
	assert.NotEmpty(t, deposits)
}
