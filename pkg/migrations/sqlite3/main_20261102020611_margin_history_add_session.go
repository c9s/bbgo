package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20261102020611_margin_history_add_session.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20261102020611, "migrations/sqlite3/20261102020611_margin_history_add_session.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `margin_loans` ADD COLUMN `session` VARCHAR(64) NOT NULL DEFAULT '';"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `margin_repays` ADD COLUMN `session` VARCHAR(64) NOT NULL DEFAULT '';"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `margin_liquidations` ADD COLUMN `session` VARCHAR(64) NOT NULL DEFAULT '';"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `margin_interests` ADD COLUMN `session` VARCHAR(64) NOT NULL DEFAULT '';"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `margin_interests` ADD COLUMN `price_in_usdt` DECIMAL(32, 16) NOT NULL DEFAULT 0;"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX `idx_margin_interests_session_time` ON `margin_interests` (`exchange`, `session`, `asset`, `time`);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS `idx_margin_interests_session_time`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `margin_interests` DROP COLUMN `price_in_usdt`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `margin_interests` DROP COLUMN `session`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `margin_liquidations` DROP COLUMN `session`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `margin_repays` DROP COLUMN `session`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `margin_loans` DROP COLUMN `session`;"},
		},
	)
}
