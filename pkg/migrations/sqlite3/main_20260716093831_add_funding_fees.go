package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20260716093831_add_funding_fees.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20260716093831, "migrations/sqlite3/20260716093831_add_funding_fees.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `funding_fees`\n(\n    `gid`         INTEGER PRIMARY KEY AUTOINCREMENT,\n    `exchange`    TEXT        NOT NULL DEFAULT '',\n    `symbol`      TEXT        NOT NULL DEFAULT '',\n    `asset`       TEXT        NOT NULL DEFAULT '',\n    `amount`      REAL        NOT NULL DEFAULT 0,\n    `txn`         BIGINT      NOT NULL DEFAULT 0,\n    `time`        DATETIME(3) NOT NULL,\n    `inserted_at` DATETIME(3)    DEFAULT CURRENT_TIMESTAMP NOT NULL\n);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX `idx_funding_fees_exchange_symbol` ON `funding_fees` (`exchange`, `symbol`);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX `idx_funding_fees_time` ON `funding_fees` (`time`);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS `idx_funding_fees_time`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX IF EXISTS `idx_funding_fees_exchange_symbol`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `funding_fees`;"},
		},
	)
}
