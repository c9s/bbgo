package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20210307201830_add_deposits_table.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20210307201830, "migrations/mysql/20210307201830_add_deposits_table.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `deposits`\n(\n    `gid`      BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,\n    `exchange` VARCHAR(24)     NOT NULL,\n    -- asset is the asset name (currency)\n    `asset`    VARCHAR(10)     NOT NULL,\n    `address`  VARCHAR(128)     NOT NULL DEFAULT '',\n    `amount`   DECIMAL(16, 8)  NOT NULL,\n    `txn_id`   VARCHAR(256)     NOT NULL,\n    `time`     DATETIME(3)     NOT NULL,\n    PRIMARY KEY (`gid`),\n    UNIQUE KEY `txn_id` (`exchange`, `txn_id`)\n);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `deposits`;"},
		},
	)
}
