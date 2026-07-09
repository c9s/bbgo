package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20220503144849_add_margin_info_to_nav.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20220503144849, "migrations/mysql/20220503144849_add_margin_info_to_nav.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `nav_history_details`\n    ADD COLUMN `session`      VARCHAR(30)                                NOT NULL,\n    ADD COLUMN `is_margin`    BOOLEAN                                    NOT NULL DEFAULT FALSE,\n    ADD COLUMN `is_isolated`  BOOLEAN                                    NOT NULL DEFAULT FALSE,\n    ADD COLUMN `isolated_symbol`  VARCHAR(30)                            NOT NULL DEFAULT '',\n    ADD COLUMN `net_asset`    DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,\n    ADD COLUMN `borrowed`     DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL,\n    ADD COLUMN `price_in_usd` DECIMAL(32, 8) UNSIGNED DEFAULT 0.00000000 NOT NULL\n;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `nav_history_details`\n    DROP COLUMN `session`,\n    DROP COLUMN `net_asset`,\n    DROP COLUMN `borrowed`,\n    DROP COLUMN `price_in_usd`,\n    DROP COLUMN `is_margin`,\n    DROP COLUMN `is_isolated`,\n    DROP COLUMN `isolated_symbol`\n;"},
		},
	)
}
