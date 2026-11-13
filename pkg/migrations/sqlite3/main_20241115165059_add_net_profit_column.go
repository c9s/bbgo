package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20241115165059_add_net_profit_column.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20241115165059, "migrations/sqlite3/20241115165059_add_net_profit_column.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `positions`\n    ADD COLUMN `net_profit` DECIMAL DEFAULT 0.00000000 NOT NULL\n;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `positions`\nDROP COLUMN `net_profit`\n;"},
		},
	)
}
