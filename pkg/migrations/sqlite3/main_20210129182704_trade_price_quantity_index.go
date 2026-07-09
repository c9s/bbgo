package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20210129182704_trade_price_quantity_index.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20210129182704, "migrations/sqlite3/20210129182704_trade_price_quantity_index.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX trades_price_quantity ON trades (order_id,price,quantity);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX trades_price_quantity;"},
		},
	)
}
