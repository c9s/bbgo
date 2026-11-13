package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20220531013327_margin_repays.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20220531013327, "migrations/sqlite3/20220531013327_margin_repays.sql", false,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `margin_repays`\n(\n    `gid`             INTEGER PRIMARY KEY AUTOINCREMENT,\n    `transaction_id`  INTEGER        NOT NULL,\n    `exchange`        VARCHAR(24)    NOT NULL DEFAULT '',\n    `asset`           VARCHAR(24)    NOT NULL DEFAULT '',\n    `isolated_symbol` VARCHAR(24)    NOT NULL DEFAULT '',\n    -- quantity is the quantity of the trade that makes profit\n    `principle`       DECIMAL(16, 8) NOT NULL,\n    `time`            DATETIME(3)    NOT NULL\n);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `margin_repays`;"},
		},
	)
}
