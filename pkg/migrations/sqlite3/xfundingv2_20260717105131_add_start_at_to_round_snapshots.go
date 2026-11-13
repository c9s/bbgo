package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from pkg/strategy/xfundingv2/migrations/sqlite3/20260717105131_add_start_at_to_round_snapshots.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("xfundingv2", 20260717105131, "pkg/strategy/xfundingv2/migrations/sqlite3/20260717105131_add_start_at_to_round_snapshots.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `xfundingv2_round_snapshots` ADD COLUMN `started_at` DATETIME(3) NULL;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `xfundingv2_round_snapshots` DROP COLUMN `started_at`;"},
		},
	)
}
