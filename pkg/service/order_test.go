package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_genOrderSQL(t *testing.T) {
	t.Run("accept empty options", func(t *testing.T) {
		o := QueryOrdersOptions{}
		assert.Equal(t, "SELECT orders.*, IFNULL(SUM(t.price * t.quantity)/SUM(t.quantity), orders.price) AS average_price FROM orders LEFT JOIN trades AS t ON (t.order_id = orders.order_id) GROUP BY orders.gid  ORDER BY orders.gid ASC LIMIT 500", genOrderSQL(o))
	})

	t.Run("different ordering ", func(t *testing.T) {
		o := QueryOrdersOptions{}
		assert.Equal(t, "SELECT orders.*, IFNULL(SUM(t.price * t.quantity)/SUM(t.quantity), orders.price) AS average_price FROM orders LEFT JOIN trades AS t ON (t.order_id = orders.order_id) GROUP BY orders.gid  ORDER BY orders.gid ASC LIMIT 500", genOrderSQL(o))
		o.Ordering = "ASC"
		assert.Equal(t, "SELECT orders.*, IFNULL(SUM(t.price * t.quantity)/SUM(t.quantity), orders.price) AS average_price FROM orders LEFT JOIN trades AS t ON (t.order_id = orders.order_id) GROUP BY orders.gid  ORDER BY orders.gid ASC LIMIT 500", genOrderSQL(o))
		o.Ordering = "DESC"
		assert.Equal(t, "SELECT orders.*, IFNULL(SUM(t.price * t.quantity)/SUM(t.quantity), orders.price) AS average_price FROM orders LEFT JOIN trades AS t ON (t.order_id = orders.order_id) GROUP BY orders.gid  ORDER BY orders.gid DESC LIMIT 500", genOrderSQL(o))
	})

}
