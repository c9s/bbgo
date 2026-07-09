package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20220504184155_fix_net_asset_column.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20220504184155, "migrations/mysql/20220504184155_fix_net_asset_column.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `nav_history_details`\n    MODIFY COLUMN `net_asset` DECIMAL(32, 8) DEFAULT 0.00000000 NOT NULL,\n    CHANGE COLUMN `balance_in_usd` `net_asset_in_usd` DECIMAL(32, 2) DEFAULT 0.00000000 NOT NULL,\n    CHANGE COLUMN `balance_in_btc` `net_asset_in_btc` DECIMAL(32, 20) DEFAULT 0.00000000 NOT NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `nav_history_details`\n    ADD COLUMN `interest` DECIMAL(32, 20) UNSIGNED DEFAULT 0.00000000 NOT NULL;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `nav_history_details`\n    DROP COLUMN `interest`;"},
		},
	)
}
