package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
		assert.Equal(t, "SELECT * FROM trades WHERE exchange = :exchange AND symbol = :symbol AND gid < :gid ORDER BY gid DESC LIMIT 500", queryTradesSQL(QueryTradesOptions{
			Exchange: "max",
			Symbol:   "btc",
			LastGID:  123,
			Ordering: "DESC",
			Limit:	  500,
		}))
	})
}
