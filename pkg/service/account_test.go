package service

import (
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestAccountService(t *testing.T) {
	db, err := prepareDB(t)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()

	xdb := sqlx.NewDb(db.DB, "sqlite3")
	service := &AccountService{DB: xdb}

	t1 := time.Now()
	err = service.InsertAsset(t1, "binance", types.ExchangeBinance, "main", false, false, "", types.AssetMap{
		"BTC": types.Asset{
			Currency:   "BTC",
			Total:      fixedpoint.MustNewFromString("1.0"),
			InUSD:      fixedpoint.MustNewFromString("10.0"),
			InBTC:      fixedpoint.MustNewFromString("0.0001"),
			Time:       t1,
			Locked:     fixedpoint.MustNewFromString("0"),
			Available:  fixedpoint.MustNewFromString("1.0"),
			Borrowed:   fixedpoint.MustNewFromString("0"),
			NetAsset:   fixedpoint.MustNewFromString("1"),
			PriceInUSD: fixedpoint.MustNewFromString("44870"),
		},
	})
	assert.NoError(t, err)
}
