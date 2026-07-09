package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20220307132917_add_positions.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20220307132917, "migrations/sqlite3/20220307132917_add_positions.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `positions`\n(\n    `gid`                  INTEGER PRIMARY KEY AUTOINCREMENT,\n    `strategy`             VARCHAR(32)    NOT NULL,\n    `strategy_instance_id` VARCHAR(64)    NOT NULL,\n    `symbol`               VARCHAR(20)    NOT NULL,\n    `quote_currency`       VARCHAR(10)    NOT NULL,\n    `base_currency`        VARCHAR(10)    NOT NULL,\n    -- average_cost is the position average cost\n    `average_cost`         DECIMAL(16, 8) NOT NULL,\n    `base`                 DECIMAL(16, 8) NOT NULL,\n    `quote`                DECIMAL(16, 8) NOT NULL,\n    `profit`               DECIMAL(16, 8) NULL,\n    -- trade related columns\n    `trade_id`             BIGINT         NOT NULL,\n    `side`                 VARCHAR(4)     NOT NULL, -- side of the trade\n    `exchange`             VARCHAR(12)    NOT NULL, -- exchange of the trade\n    `traded_at`            DATETIME(3)    NOT NULL\n);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `positions`;"},
		},
	)
}
