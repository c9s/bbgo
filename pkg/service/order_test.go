package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_genOrderSQL(t *testing.T) {
    t.Run("accept empty options", func(t *testing.T) {
        o := QueryOrdersOptions{}
        assert.Equal(t, "SELECT orders.*, IFNULL(avg_trades.avg_price, orders.price) AS average_price FROM orders LEFT JOIN (SELECT order_id, exchange, SUM(price * quantity)/NULLIF(SUM(quantity), 0) AS avg_price FROM trades GROUP BY exchange, order_id) AS avg_trades ON (avg_trades.order_id = orders.order_id AND avg_trades.exchange = orders.exchange) ORDER BY orders.gid ASC LIMIT 500", genOrderSQL("sqlite", o))
    })

    t.Run("different ordering ", func(t *testing.T) {
        o := QueryOrdersOptions{}
        assert.Equal(t, "SELECT orders.*, IFNULL(avg_trades.avg_price, orders.price) AS average_price FROM orders LEFT JOIN (SELECT order_id, exchange, SUM(price * quantity)/NULLIF(SUM(quantity), 0) AS avg_price FROM trades GROUP BY exchange, order_id) AS avg_trades ON (avg_trades.order_id = orders.order_id AND avg_trades.exchange = orders.exchange) ORDER BY orders.gid ASC LIMIT 500", genOrderSQL("sqlite", o))
        o.Ordering = "ASC"
        assert.Equal(t, "SELECT orders.*, IFNULL(avg_trades.avg_price, orders.price) AS average_price FROM orders LEFT JOIN (SELECT order_id, exchange, SUM(price * quantity)/NULLIF(SUM(quantity), 0) AS avg_price FROM trades GROUP BY exchange, order_id) AS avg_trades ON (avg_trades.order_id = orders.order_id AND avg_trades.exchange = orders.exchange) ORDER BY orders.gid ASC LIMIT 500", genOrderSQL("sqlite", o))
        o.Ordering = "DESC"
        assert.Equal(t, "SELECT orders.*, IFNULL(avg_trades.avg_price, orders.price) AS average_price FROM orders LEFT JOIN (SELECT order_id, exchange, SUM(price * quantity)/NULLIF(SUM(quantity), 0) AS avg_price FROM trades GROUP BY exchange, order_id) AS avg_trades ON (avg_trades.order_id = orders.order_id AND avg_trades.exchange = orders.exchange) ORDER BY orders.gid DESC LIMIT 500", genOrderSQL("sqlite", o))
    })

}
