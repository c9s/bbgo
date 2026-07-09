package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20220503144849_add_margin_info_to_nav.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20220503144849, "migrations/sqlite3/20220503144849_add_margin_info_to_nav.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `nav_history_details` ADD COLUMN `session` VARCHAR(50) NOT NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `nav_history_details` ADD COLUMN `borrowed` DECIMAL DEFAULT 0.00000000 NOT NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `nav_history_details` ADD COLUMN `net_asset` DECIMAL DEFAULT 0.00000000 NOT NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `nav_history_details` ADD COLUMN `price_in_usd` DECIMAL DEFAULT 0.00000000 NOT NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `nav_history_details` ADD COLUMN `is_margin` BOOL DEFAULT FALSE NOT NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `nav_history_details` ADD COLUMN `is_isolated` BOOL DEFAULT FALSE NOT NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `nav_history_details` ADD COLUMN `isolated_symbol` VARCHAR(30) DEFAULT '' NOT NULL;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "SELECT 1;"},
		},
	)
}
