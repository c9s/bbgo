package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20210215203111_add_pnl_column.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20210215203111, "migrations/sqlite3/20210215203111_add_pnl_column.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `trades` ADD COLUMN `pnl` DECIMAL NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `trades` ADD COLUMN `strategy` TEXT;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `trades` RENAME COLUMN `pnl` TO `pnl_deleted`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `trades` RENAME COLUMN `strategy` TO `strategy_deleted`;"},
		},
	)
}
