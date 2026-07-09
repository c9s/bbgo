package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20260114145908_fix_positions_unique_key_trade_id.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20260114145908, "migrations/sqlite3/20260114145908_fix_positions_unique_key_trade_id.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE UNIQUE INDEX `positions_trade_id` ON `positions` (`trade_id`, `side`, `symbol`, `exchange`);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS `positions_trade_id`;"},
		},
	)
}
