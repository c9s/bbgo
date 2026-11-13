package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20250721175655_orders_add_uuid.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20250721175655, "migrations/sqlite3/20250721175655_orders_add_uuid.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE trades ADD COLUMN order_uuid TEXT NOT NULL DEFAULT '';"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE trades DROP COLUMN order_uuid;"},
		},
	)
}
