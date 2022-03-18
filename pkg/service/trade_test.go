package service

import (
	"context"
	"testing"
	"time"

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

	ctx := context.Background()

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

	err = service.Mark(ctx, 1, "grid")
	assert.NoError(t, err)

	tradeRecord, err := service.Load(ctx, 1)
	assert.NoError(t, err)
	assert.NotNil(t, tradeRecord)
	assert.True(t, tradeRecord.StrategyID.Valid)
	assert.Equal(t, "grid", tradeRecord.StrategyID.String)

	err = service.UpdatePnL(ctx, 1, 10.0)
	assert.NoError(t, err)

	tradeRecord, err = service.Load(ctx, 1)
	assert.NoError(t, err)
	assert.NotNil(t, tradeRecord)
	assert.True(t, tradeRecord.PnL.Valid)
	assert.Equal(t, 10.0, tradeRecord.PnL.Float64)
}

func Test_queryTradingVolumeSQL(t *testing.T) {
	t.Run("group by different period", func(t *testing.T) {
		o := TradingVolumeQueryOptions{
			GroupByPeriod: "month",
		}
		assert.Equal(t, "SELECT YEAR(traded_at) AS year, MONTH(traded_at) AS month, SUM(quantity * price) AS quote_volume FROM trades WHERE traded_at > :start_time GROUP BY MONTH(traded_at), YEAR(traded_at) ORDER BY year ASC, month ASC", generateMysqlTradingVolumeQuerySQL(o))

		o.GroupByPeriod = "year"
		assert.Equal(t, "SELECT YEAR(traded_at) AS year, SUM(quantity * price) AS quote_volume FROM trades WHERE traded_at > :start_time GROUP BY YEAR(traded_at) ORDER BY year ASC", generateMysqlTradingVolumeQuerySQL(o))

		expectedDefaultSQL := "SELECT YEAR(traded_at) AS year, MONTH(traded_at) AS month, DAY(traded_at) AS day, SUM(quantity * price) AS quote_volume FROM trades WHERE traded_at > :start_time GROUP BY DAY(traded_at), MONTH(traded_at), YEAR(traded_at) ORDER BY year ASC, month ASC, day ASC"
		for _, s := range []string{"", "day"} {
			o.GroupByPeriod = s
			assert.Equal(t, expectedDefaultSQL, generateMysqlTradingVolumeQuerySQL(o))
		}
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
