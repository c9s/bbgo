package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20210301140656_add_withdraws_table.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20210301140656, "migrations/sqlite3/20210301140656_add_withdraws_table.sql", false,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `withdraws`\n(\n    `gid`              INTEGER PRIMARY KEY AUTOINCREMENT,\n    `exchange`         VARCHAR(24)    NOT NULL DEFAULT '',\n    -- asset is the asset name (currency)\n    `asset`            VARCHAR(10)    NOT NULL,\n    `address`          VARCHAR(128)    NOT NULL,\n    `network`          VARCHAR(32)    NOT NULL DEFAULT '',\n    `amount`           DECIMAL(16, 8) NOT NULL,\n    `txn_id`           VARCHAR(256)    NOT NULL,\n    `txn_fee`          DECIMAL(16, 8) NOT NULL DEFAULT 0,\n    `txn_fee_currency` VARCHAR(32)    NOT NULL DEFAULT '',\n    `time`             DATETIME(3)    NOT NULL\n);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE UNIQUE INDEX `withdraws_txn_id` ON `withdraws` (`exchange`, `txn_id`);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS `withdraws_txn_id`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `withdraws`;"},
		},
	)
}
