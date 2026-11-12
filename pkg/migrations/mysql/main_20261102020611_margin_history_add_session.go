package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20261102020611_margin_history_add_session.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20261102020611, "migrations/mysql/20261102020611_margin_history_add_session.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `margin_loans`\n    ADD COLUMN `session` VARCHAR(64) NOT NULL DEFAULT '',\n    MODIFY `principle` DECIMAL(32, 8) UNSIGNED NOT NULL,\n    DROP INDEX `transaction_id`,\n    ADD UNIQUE KEY `transaction_id_session` (`transaction_id`, `session`);"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `margin_repays`\n    ADD COLUMN `session` VARCHAR(64) NOT NULL DEFAULT '',\n    MODIFY `principle` DECIMAL(32, 8) UNSIGNED NOT NULL,\n    DROP INDEX `transaction_id`,\n    ADD UNIQUE KEY `transaction_id_session` (`transaction_id`, `session`);"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `margin_liquidations`\n    ADD COLUMN `session` VARCHAR(64) NOT NULL DEFAULT '',\n    DROP INDEX `order_id`,\n    ADD UNIQUE KEY `order_id_exchange_session` (`order_id`, `exchange`, `session`);"},
			{Direction: rockhopper.DirectionUp, SQL: "ALTER TABLE `margin_interests`\n    ADD COLUMN `session` VARCHAR(64) NOT NULL DEFAULT '',\n    ADD COLUMN `price_in_usdt` DECIMAL(32, 16) UNSIGNED NOT NULL DEFAULT 0,\n    MODIFY `principle` DECIMAL(32, 8) UNSIGNED NOT NULL,\n    MODIFY `interest` DECIMAL(32, 16) UNSIGNED NOT NULL;"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX `idx_margin_interests_session_time` ON `margin_interests` (`exchange`, `session`, `asset`, `time`);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX `idx_margin_interests_session_time` ON `margin_interests`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DELETE a FROM `margin_interests` a\n    JOIN `margin_interests` b\n        ON a.`exchange` = b.`exchange`\n        AND a.`asset` = b.`asset`\n        AND a.`isolated_symbol` = b.`isolated_symbol`\n        AND a.`time` = b.`time`\n        AND a.`principle` = b.`principle`\n        AND a.`interest` = b.`interest`\n        AND a.`interest_rate` = b.`interest_rate`\n        AND b.`session` < a.`session`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `margin_interests` DROP COLUMN `price_in_usdt`, DROP COLUMN `session`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DELETE a FROM `margin_liquidations` a\n    JOIN `margin_liquidations` b\n        ON a.`order_id` = b.`order_id`\n        AND a.`exchange` = b.`exchange`\n        AND b.`session` < a.`session`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `margin_liquidations`\n    DROP INDEX `order_id_exchange_session`,\n    ADD UNIQUE KEY `order_id` (`order_id`, `exchange`),\n    DROP COLUMN `session`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DELETE a FROM `margin_repays` a\n    JOIN `margin_repays` b\n        ON a.`transaction_id` = b.`transaction_id`\n        AND b.`session` < a.`session`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `margin_repays`\n    DROP INDEX `transaction_id_session`,\n    ADD UNIQUE KEY `transaction_id` (`transaction_id`),\n    DROP COLUMN `session`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DELETE a FROM `margin_loans` a\n    JOIN `margin_loans` b\n        ON a.`transaction_id` = b.`transaction_id`\n        AND b.`session` < a.`session`;"},
			{Direction: rockhopper.DirectionDown, SQL: "ALTER TABLE `margin_loans`\n    DROP INDEX `transaction_id_session`,\n    ADD UNIQUE KEY `transaction_id` (`transaction_id`),\n    DROP COLUMN `session`;"},
		},
	)
}
