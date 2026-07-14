package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from pkg/strategy/xfundingv2/migrations/mysql/20260713211916_add_unique_key_round_id.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("xfundingv2", 20260713211916, "pkg/strategy/xfundingv2/migrations/mysql/20260713211916_add_unique_key_round_id.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `xfundingv2_closed_rounds`\n    DROP KEY `idx_xfundingv2_closed_rounds_round_id`,\n    ADD UNIQUE KEY `uk_xfundingv2_closed_rounds_id` (`id`);"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `xfundingv2_funding_fees`\n    DROP KEY `idx_xfundingv2_funding_fees_round_id`,\n    ADD UNIQUE KEY `uk_xfundingv2_funding_fees_round_id_txn` (`round_id`, `txn`);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `xfundingv2_funding_fees`\n    DROP KEY `uk_xfundingv2_funding_fees_round_id_txn`,\n    ADD KEY `idx_xfundingv2_funding_fees_round_id` (`round_id`);"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `xfundingv2_closed_rounds`\n    DROP KEY `uk_xfundingv2_closed_rounds_id`,\n    ADD KEY `idx_xfundingv2_closed_rounds_round_id` (`id`);"},
		},
	)
}
