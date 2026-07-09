package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20210223080622_add_rewards_table.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20210223080622, "migrations/mysql/20210223080622_add_rewards_table.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `rewards`\n(\n    `gid`         BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,\n    -- for exchange\n    `exchange`    VARCHAR(24)             NOT NULL DEFAULT '',\n    -- reward record id\n    `uuid`        VARCHAR(32)             NOT NULL,\n    `reward_type` VARCHAR(24)             NOT NULL DEFAULT '',\n    -- currency symbol, BTC, MAX, USDT ... etc\n    `currency`    VARCHAR(5)              NOT NULL,\n    -- the quantity of the rewards\n    `quantity`    DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `state`       VARCHAR(5)              NOT NULL,\n    `created_at`  DATETIME                NOT NULL,\n    `spent`       BOOLEAN                 NOT NULL DEFAULT FALSE,\n    `note`        TEXT                    NULL,\n    PRIMARY KEY (`gid`),\n    UNIQUE KEY `uuid` (`exchange`, `uuid`)\n);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `rewards`;"},
		},
	)
}
