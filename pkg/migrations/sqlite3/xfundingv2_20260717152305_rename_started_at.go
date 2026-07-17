package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from pkg/strategy/xfundingv2/migrations/sqlite3/20260717152305_rename_started_at.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("xfundingv2", 20260717152305, "pkg/strategy/xfundingv2/migrations/sqlite3/20260717152305_rename_started_at.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `xfundingv2_closed_rounds` RENAME COLUMN `start_at` TO `started_at`;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `xfundingv2_closed_rounds` RENAME COLUMN `started_at` TO `start_at`;"},
		},
	)
}
