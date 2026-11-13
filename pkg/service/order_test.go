package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_genOrderSQL(t *testing.T) {
	t.Run("accept empty options", func(t *testing.T) {
		o := QueryOrdersOptions{}
		assert.Equal(t, "SELECT orders.*, IFNULL(SUM(t.price * t.quantity)/SUM(t.quantity), orders.price) AS average_price FROM orders LEFT JOIN trades AS t ON (t.order_id = orders.order_id) GROUP BY orders.gid  ORDER BY orders.gid ASC LIMIT 500", genOrderSQL("sqlite", o))
	})

	t.Run("different ordering ", func(t *testing.T) {
		o := QueryOrdersOptions{}
		assert.Equal(t, "SELECT orders.*, IFNULL(SUM(t.price * t.quantity)/SUM(t.quantity), orders.price) AS average_price FROM orders LEFT JOIN trades AS t ON (t.order_id = orders.order_id) GROUP BY orders.gid  ORDER BY orders.gid ASC LIMIT 500", genOrderSQL("sqlite", o))
		o.Ordering = "ASC"
		assert.Equal(t, "SELECT orders.*, IFNULL(SUM(t.price * t.quantity)/SUM(t.quantity), orders.price) AS average_price FROM orders LEFT JOIN trades AS t ON (t.order_id = orders.order_id) GROUP BY orders.gid  ORDER BY orders.gid ASC LIMIT 500", genOrderSQL("sqlite", o))
		o.Ordering = "DESC"
		assert.Equal(t, "SELECT orders.*, IFNULL(SUM(t.price * t.quantity)/SUM(t.quantity), orders.price) AS average_price FROM orders LEFT JOIN trades AS t ON (t.order_id = orders.order_id) GROUP BY orders.gid  ORDER BY orders.gid DESC LIMIT 500", genOrderSQL("sqlite", o))
	})

	t.Run("with since and until", func(t *testing.T) {
		since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		until := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
		o := QueryOrdersOptions{
			Exchange: "binance",
			Symbol:   "BTCUSDT",
			Since:    &since,
			Until:    &until,
		}
		sql := genOrderSQL("sqlite", o)
		assert.Contains(t, sql, "orders.created_at >= :since")
		assert.Contains(t, sql, "orders.created_at < :until")
		assert.Contains(t, sql, "orders.exchange = :exchange")
		assert.Contains(t, sql, "orders.symbol = :symbol")
	})

	t.Run("with only since", func(t *testing.T) {
		since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		o := QueryOrdersOptions{
			Since: &since,
		}
		sql := genOrderSQL("sqlite", o)
		assert.Contains(t, sql, "orders.created_at >= :since")
		assert.NotContains(t, sql, ":until")
	})

	t.Run("custom limit", func(t *testing.T) {
		o := QueryOrdersOptions{Limit: 100}
		sql := genOrderSQL("sqlite", o)
		assert.Contains(t, sql, "LIMIT 100")
	})

}
