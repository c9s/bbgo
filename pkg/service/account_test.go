package service

import (
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/asset"
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
	err = service.InsertAsset(t1, "binance", types.ExchangeBinance, "main", false, false, "", asset.Map{
		"BTC": asset.Asset{
			Currency:      "BTC",
			Total:         fixedpoint.MustNewFromString("1.0"),
			NetAssetInUSD: fixedpoint.MustNewFromString("10.0"),
			NetAssetInBTC: fixedpoint.MustNewFromString("0.0001"),
			Time:          t1,
			Locked:        fixedpoint.MustNewFromString("0"),
			Available:     fixedpoint.MustNewFromString("1.0"),
			Borrowed:      fixedpoint.MustNewFromString("0"),
			NetAsset:      fixedpoint.MustNewFromString("1"),
			PriceInUSD:    fixedpoint.MustNewFromString("44870"),
		},
	})
	assert.NoError(t, err)
}
