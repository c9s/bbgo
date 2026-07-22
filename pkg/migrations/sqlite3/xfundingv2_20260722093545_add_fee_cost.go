package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from pkg/strategy/xfundingv2/migrations/sqlite3/20260722093545_add_fee_cost.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("xfundingv2", 20260722093545, "pkg/strategy/xfundingv2/migrations/sqlite3/20260722093545_add_fee_cost.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `xfundingv2_closed_rounds` ADD COLUMN `fee_symbol` TEXT NOT NULL DEFAULT '';"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `xfundingv2_closed_rounds` ADD COLUMN `fee_avg_cost` REAL NOT NULL DEFAULT 0;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `xfundingv2_round_snapshots` ADD COLUMN `fee_symbol` TEXT NOT NULL DEFAULT '';"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `xfundingv2_round_snapshots` ADD COLUMN `fee_avg_cost` REAL NOT NULL DEFAULT 0;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `xfundingv2_round_snapshots` DROP COLUMN `fee_avg_cost`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `xfundingv2_round_snapshots` DROP COLUMN `fee_symbol`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `xfundingv2_closed_rounds` DROP COLUMN `fee_avg_cost`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `xfundingv2_closed_rounds` DROP COLUMN `fee_symbol`;"},
		},
	)
}
