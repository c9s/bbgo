package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20210307201830_add_deposits_table.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20210307201830, "migrations/sqlite3/20210307201830_add_deposits_table.sql", false,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `deposits`\n(\n    `gid`      INTEGER PRIMARY KEY AUTOINCREMENT,\n    `exchange` VARCHAR(24)    NOT NULL,\n    -- asset is the asset name (currency)\n    `asset`    VARCHAR(10)    NOT NULL,\n    `address`  VARCHAR(128)    NOT NULL DEFAULT '',\n    `amount`   DECIMAL(16, 8) NOT NULL,\n    `txn_id`   VARCHAR(256)    NOT NULL,\n    `time`     DATETIME(3)    NOT NULL\n);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE UNIQUE INDEX `deposits_txn_id` ON `deposits` (`exchange`, `txn_id`);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS `deposits_txn_id`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `deposits`;"},
		},
	)
}
