package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from pkg/strategy/xfundingv2/migrations/sqlite3/20260713211916_add_unique_key_round_id.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("xfundingv2", 20260713211916, "pkg/strategy/xfundingv2/migrations/sqlite3/20260713211916_add_unique_key_round_id.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "DROP INDEX IF EXISTS `idx_xfundingv2_closed_rounds_round_id`;"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE UNIQUE INDEX `uk_xfundingv2_closed_rounds_id` ON `xfundingv2_closed_rounds` (`id`);"},
			{Direction: rockhopper.DirectionUp, SQL: "DROP INDEX IF EXISTS `idx_xfundingv2_funding_fees_round_id`;"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE UNIQUE INDEX `uk_xfundingv2_funding_fees_round_id_txn` ON `xfundingv2_funding_fees` (`round_id`, `txn`);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS `uk_xfundingv2_funding_fees_round_id_txn`;"},
			{Direction: rockhopper.DirectionDown, SQL: "CREATE INDEX `idx_xfundingv2_funding_fees_round_id` ON `xfundingv2_funding_fees` (`round_id`);"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS `uk_xfundingv2_closed_rounds_id`;"},
			{Direction: rockhopper.DirectionDown, SQL: "CREATE INDEX `idx_xfundingv2_closed_rounds_round_id` ON `xfundingv2_closed_rounds` (`id`);"},
		},
	)
}
