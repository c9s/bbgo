package bbgo

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

func TestTradeService(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	_ = mock

	xdb := sqlx.NewDb(db, "mysql")
	service.NewTradeService(xdb)
	/*

		stmt := mock.ExpectQuery(`SELECT \* FROM trades WHERE symbol = \? ORDER BY gid DESC LIMIT 1`)
		stmt.WithArgs("BTCUSDT")
		stmt.WillReturnRows(sqlmock.NewRows([]string{"gid", "id", "exchange", "symbol", "price", "quantity"}))

		stmt2 := mock.ExpectQuery(`INSERT INTO trades (id, exchange, symbol, price, quantity, quote_quantity, side, is_buyer, is_maker, fee, fee_currency, traded_at)
	        	            				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		stmt2.WithArgs()
	*/
}

func TestEnvironment_Connect(t *testing.T) {
	mysqlURL := os.Getenv("MYSQL_URL")
	if len(mysqlURL) == 0 {
		t.Skip("require mysql url")
	}
	mysqlURL = fmt.Sprintf("%s?parseTime=true", mysqlURL)

	key, secret := os.Getenv("BINANCE_API_KEY"), os.Getenv("BINANCE_API_SECRET")
	if len(key) == 0 || len(secret) == 0 {
		t.Skip("require key and secret")
	}

	exchange := binance.New(key, secret)
	assert.NotNil(t, exchange)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	xdb, err := sqlx.Connect("mysql", mysqlURL)
	assert.NoError(t, err)

	environment := NewEnvironment(xdb)
	environment.AddExchange("binance", exchange).
		Subscribe(types.KLineChannel,"BTCUSDT", types.SubscribeOptions{})

	err = environment.Connect(ctx)
	assert.NoError(t, err)

	time.Sleep(5 * time.Second)
}

