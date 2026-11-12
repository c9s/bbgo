package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20261112104111_margin_interests_add_session_and_tx.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20261112104111, "migrations/sqlite3/20261112104111_margin_interests_add_session_and_tx.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `margin_interests` ADD COLUMN `session` TEXT NOT NULL DEFAULT '';"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `margin_interests` ADD COLUMN `tx_id` BIGINT NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `margin_interests` ADD COLUMN `raw_asset` TEXT NOT NULL DEFAULT '';"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `margin_interests` ADD COLUMN `interest_type` TEXT NOT NULL DEFAULT '';"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `margin_interests` RENAME COLUMN `principle` TO `principal`;"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE UNIQUE INDEX `uidx_margin_interests_exchange_session_tx` ON `margin_interests` (`exchange`, `session`, `tx_id`);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX `idx_margin_interests_session_time` ON `margin_interests` (`exchange`, `session`, `asset`, `time`);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS `idx_margin_interests_session_time`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS `uidx_margin_interests_exchange_session_tx`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `margin_interests` RENAME COLUMN `principal` TO `principle`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `margin_interests` DROP COLUMN `interest_type`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `margin_interests` DROP COLUMN `raw_asset`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `margin_interests` DROP COLUMN `tx_id`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `margin_interests` DROP COLUMN `session`;"},
		},
	)
}
