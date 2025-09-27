package service

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func Test_tradeService(t *testing.T) {
	db, err := prepareDB(t)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()

	xdb := sqlx.NewDb(db.DB, "sqlite3")
	service := &TradeService{DB: xdb}

	err = service.Insert(types.Trade{
		ID:            1,
		OrderID:       1,
		Exchange:      "binance",
		Price:         fixedpoint.NewFromInt(1000),
		Quantity:      fixedpoint.NewFromFloat(0.1),
		QuoteQuantity: fixedpoint.NewFromFloat(1000.0 * 0.1),
		Symbol:        "BTCUSDT",
		Side:          "BUY",
		IsBuyer:       true,
		Time:          types.Time(time.Now()),
	})
	assert.NoError(t, err)
}

func Test_queryTradingVolumeSQL(t *testing.T) {
	t.Run("group by different period with MySQL dialect", func(t *testing.T) {
		mysqlDialect := &MySQLDialect{}

		o := TradingVolumeQueryOptions{
			GroupByPeriod: "month",
		}
		assert.Equal(t, "SELECT YEAR(traded_at) AS year, MONTH(traded_at) AS month, SUM(quantity * price) AS quote_volume FROM trades WHERE traded_at > :start_time GROUP BY MONTH(traded_at), YEAR(traded_at) ORDER BY year ASC, month ASC", GenerateTradingVolumeQuerySQL(mysqlDialect, o))

		o.GroupByPeriod = "year"
		assert.Equal(t, "SELECT YEAR(traded_at) AS year, SUM(quantity * price) AS quote_volume FROM trades WHERE traded_at > :start_time GROUP BY YEAR(traded_at) ORDER BY year ASC", GenerateTradingVolumeQuerySQL(mysqlDialect, o))

		expectedDefaultSQL := "SELECT YEAR(traded_at) AS year, MONTH(traded_at) AS month, DAY(traded_at) AS day, SUM(quantity * price) AS quote_volume FROM trades WHERE traded_at > :start_time GROUP BY DAY(traded_at), MONTH(traded_at), YEAR(traded_at) ORDER BY year ASC, month ASC, day ASC"
		for _, s := range []string{"", "day"} {
			o.GroupByPeriod = s
			assert.Equal(t, expectedDefaultSQL, GenerateTradingVolumeQuerySQL(mysqlDialect, o))
		}
	})

	t.Run("group by different period with PostgreSQL dialect", func(t *testing.T) {
		pgDialect := &PostgreSQLDialect{}

		o := TradingVolumeQueryOptions{
			GroupByPeriod: "month",
		}
		assert.Equal(t, "SELECT EXTRACT(YEAR FROM traded_at) AS year, EXTRACT(MONTH FROM traded_at) AS month, SUM(quantity * price) AS quote_volume FROM trades WHERE traded_at > :start_time GROUP BY EXTRACT(MONTH FROM traded_at), EXTRACT(YEAR FROM traded_at) ORDER BY year ASC, month ASC", GenerateTradingVolumeQuerySQL(pgDialect, o))

		o.GroupByPeriod = "year"
		assert.Equal(t, "SELECT EXTRACT(YEAR FROM traded_at) AS year, SUM(quantity * price) AS quote_volume FROM trades WHERE traded_at > :start_time GROUP BY EXTRACT(YEAR FROM traded_at) ORDER BY year ASC", GenerateTradingVolumeQuerySQL(pgDialect, o))
	})

}

func Test_queryTradesSQL(t *testing.T) {
	t.Run("generate order by clause by Ordering option", func(t *testing.T) {
		assert.Equal(t, "SELECT * FROM trades ORDER BY gid ASC LIMIT 500", queryTradesSQL(QueryTradesOptions{Limit: 500}))
		assert.Equal(t, "SELECT * FROM trades ORDER BY gid ASC LIMIT 500", queryTradesSQL(QueryTradesOptions{Ordering: "ASC", Limit: 500}))
		assert.Equal(t, "SELECT * FROM trades ORDER BY gid DESC LIMIT 500", queryTradesSQL(QueryTradesOptions{Ordering: "DESC", Limit: 500}))
	})

	t.Run("filter by exchange name", func(t *testing.T) {
		assert.Equal(t, "SELECT * FROM trades WHERE exchange = :exchange ORDER BY gid ASC LIMIT 500", queryTradesSQL(QueryTradesOptions{Exchange: "max", Limit: 500}))
	})

	t.Run("filter by symbol", func(t *testing.T) {
		assert.Equal(t, "SELECT * FROM trades WHERE symbol = :symbol ORDER BY gid ASC LIMIT 500", queryTradesSQL(QueryTradesOptions{Symbol: "eth", Limit: 500}))
	})

	t.Run("GID ordering", func(t *testing.T) {
		assert.Equal(t, "SELECT * FROM trades WHERE gid > :gid ORDER BY gid ASC LIMIT 500", queryTradesSQL(QueryTradesOptions{LastGID: 1, Limit: 500}))
		assert.Equal(t, "SELECT * FROM trades WHERE gid > :gid ORDER BY gid ASC LIMIT 500", queryTradesSQL(QueryTradesOptions{LastGID: 1, Ordering: "ASC", Limit: 500}))
		assert.Equal(t, "SELECT * FROM trades WHERE gid < :gid ORDER BY gid DESC LIMIT 500", queryTradesSQL(QueryTradesOptions{LastGID: 1, Ordering: "DESC", Limit: 500}))
	})

	t.Run("convert all options", func(t *testing.T) {
		assert.Equal(t, "SELECT * FROM trades WHERE gid < :gid AND symbol = :symbol AND exchange = :exchange ORDER BY gid DESC LIMIT 500", queryTradesSQL(QueryTradesOptions{
			Exchange: "max",
			Symbol:   "btc",
			LastGID:  123,
			Ordering: "DESC",
			Limit:    500,
		}))
	})
}

func TestTradeService_Query(t *testing.T) {
	db, mock, err := sqlmock.New()
	if !assert.NoError(t, err) {
		return
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "mysql")
	defer sqlxDB.Close()

	s := NewTradeService(sqlxDB)

	_, err = s.Query(QueryTradesOptions{Ordering: "test_ordering"})
	assert.Error(t, err)
	assert.Equal(t, "invalid ordering: test_ordering", err.Error())

	_, err = s.Query(QueryTradesOptions{OrderByColumn: "invalid_column"})
	assert.Error(t, err)
	assert.Equal(t, "invalid order by column: invalid_column", err.Error())

	mock.ExpectQuery("SELECT .* FROM trades WHERE gid > \\? ORDER BY gid ASC").WithArgs(1234).WillReturnError(sql.ErrNoRows)
	_, err = s.Query(QueryTradesOptions{LastGID: 1234, Ordering: "ASC", OrderByColumn: "gid"})
	assert.Equal(t, sql.ErrNoRows, err)

	mock.ExpectQuery("SELECT .* FROM trades ORDER BY gid DESC").WillReturnError(sql.ErrNoRows)
	_, err = s.Query(QueryTradesOptions{Ordering: "DESC", OrderByColumn: "gid"})
	assert.Equal(t, sql.ErrNoRows, err)

	mock.ExpectQuery("SELECT .* FROM trades ORDER BY traded_at ASC").WillReturnError(sql.ErrNoRows)
	_, err = s.Query(QueryTradesOptions{Ordering: "ASC", OrderByColumn: "traded_at"})
	assert.Equal(t, sql.ErrNoRows, err)
}

func Test_genTradeSelectColumns(t *testing.T) {
	assert.Equal(t, []string{"*"}, genTradeSelectColumns("sqlite3"))
	mysqlDialect := &MySQLDialect{}
	expectedMySQLColumns := []string{"gid", "id", "order_id", dialectUuidSelector(mysqlDialect, "trades", "order_uuid"), "exchange", "price", "quantity", "quote_quantity", "symbol", "side", "is_buyer", "is_maker", "traded_at", "fee", "fee_currency", "is_margin", "is_futures", "is_isolated", "strategy", "pnl", "inserted_at"}
	assert.Equal(t, expectedMySQLColumns, genTradeSelectColumns("mysql"))
}
