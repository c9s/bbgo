package service

import (
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/datatype"
	"github.com/c9s/bbgo/pkg/types"
)

func TestWithdrawService(t *testing.T) {
	db, err := prepareDB(t)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()

	xdb := sqlx.NewDb(db.DB, "sqlite3")
	service := &WithdrawService{DB: xdb}

	err = service.Insert(types.Withdraw{
		Exchange:       types.ExchangeMax,
		Asset:          "BTC",
		Amount:         0.0001,
		Address:        "test",
		TransactionID:  "01",
		TransactionFee: 0.0001,
		Network:        "omni",
		ApplyTime:      datatype.Time(time.Now()),
	})
	assert.NoError(t, err)

	withdraws, err := service.Query(types.ExchangeMax)
	assert.NoError(t, err)
	assert.NotEmpty(t, withdraws)
	assert.Equal(t, types.ExchangeMax, withdraws[0].Exchange)
}
