package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20261112104111_margin_interests_add_session_and_tx.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20261112104111, "migrations/mysql/20261112104111_margin_interests_add_session_and_tx.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `margin_interests`\n    ADD COLUMN `session`       VARCHAR(64)     NOT NULL DEFAULT '',\n    ADD COLUMN `tx_id`         BIGINT UNSIGNED NULL,\n    ADD COLUMN `raw_asset`     VARCHAR(24)     NOT NULL DEFAULT '',\n    ADD COLUMN `interest_type` VARCHAR(32)     NOT NULL DEFAULT '',\n    CHANGE `principle` `principal` DECIMAL(32, 8) UNSIGNED NOT NULL,\n    MODIFY `interest` DECIMAL(32, 16) UNSIGNED NOT NULL,\n    ADD UNIQUE KEY `uidx_margin_interests_exchange_session_tx` (`exchange`, `session`, `tx_id`);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX `idx_margin_interests_session_time` ON `margin_interests` (`exchange`, `session`, `asset`, `time`);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX `idx_margin_interests_session_time` ON `margin_interests`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `margin_interests`\n    DROP INDEX `uidx_margin_interests_exchange_session_tx`,\n    CHANGE `principal` `principle` DECIMAL(32, 8) UNSIGNED NOT NULL,\n    DROP COLUMN `interest_type`,\n    DROP COLUMN `raw_asset`,\n    DROP COLUMN `tx_id`,\n    DROP COLUMN `session`;"},
		},
	)
}
