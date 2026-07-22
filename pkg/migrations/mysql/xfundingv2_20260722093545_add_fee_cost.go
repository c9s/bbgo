package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from pkg/strategy/xfundingv2/migrations/mysql/20260722093545_add_fee_cost.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("xfundingv2", 20260722093545, "pkg/strategy/xfundingv2/migrations/mysql/20260722093545_add_fee_cost.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `xfundingv2_closed_rounds`\n    ADD COLUMN `fee_symbol`   VARCHAR(32)    NOT NULL DEFAULT '' AFTER `funding_income`,\n    ADD COLUMN `fee_avg_cost` DECIMAL(20, 8) NOT NULL DEFAULT 0  AFTER `fee_symbol`;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `xfundingv2_round_snapshots`\n    ADD COLUMN `fee_symbol`   VARCHAR(32)    NOT NULL DEFAULT '' AFTER `funding_income`,\n    ADD COLUMN `fee_avg_cost` DECIMAL(20, 8) NOT NULL DEFAULT 0  AFTER `fee_symbol`;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `xfundingv2_round_snapshots`\n    DROP COLUMN `fee_avg_cost`,\n    DROP COLUMN `fee_symbol`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `xfundingv2_closed_rounds`\n    DROP COLUMN `fee_avg_cost`,\n    DROP COLUMN `fee_symbol`;"},
		},
	)
}
