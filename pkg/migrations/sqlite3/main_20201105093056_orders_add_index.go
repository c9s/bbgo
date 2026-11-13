package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20201105093056_orders_add_index.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20201105093056, "migrations/sqlite3/20201105093056_orders_add_index.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX orders_symbol ON orders (exchange, symbol);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE UNIQUE INDEX orders_order_id ON orders (order_id, exchange);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS orders_symbol;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS orders_order_id;"},
		},
	)
}
