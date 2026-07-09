package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20220531015005_margin_liquidations.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20220531015005, "migrations/sqlite3/20220531015005_margin_liquidations.sql", false,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `margin_liquidations`\n(\n    `gid`               INTEGER PRIMARY KEY AUTOINCREMENT,\n    `exchange`          VARCHAR(24)    NOT NULL DEFAULT '',\n    `symbol`            VARCHAR(24)    NOT NULL DEFAULT '',\n    `order_id`          INTEGER        NOT NULL,\n    `is_isolated`       BOOL           NOT NULL DEFAULT false,\n    `average_price`     DECIMAL(16, 8) NOT NULL,\n    `price`             DECIMAL(16, 8) NOT NULL,\n    `quantity`          DECIMAL(16, 8) NOT NULL,\n    `executed_quantity` DECIMAL(16, 8) NOT NULL,\n    `side`              VARCHAR(5)     NOT NULL DEFAULT '',\n    `time_in_force`     VARCHAR(5)     NOT NULL DEFAULT '',\n    `time`      DATETIME(3)    NOT NULL\n);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `margin_liquidations`;"},
		},
	)
}
