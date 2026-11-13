package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20210118163847_fix_unique_index.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20210118163847, "migrations/sqlite3/20210118163847_fix_unique_index.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE UNIQUE INDEX `trade_unique_id` ON `trades` (`exchange`,`symbol`, `side`, `id`);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS `trade_unique_id`;"},
		},
	)
}
