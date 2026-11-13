package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20200721225616_trades.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20200721225616, "migrations/sqlite3/20200721225616_trades.sql", false,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `trades`\n(\n    `gid`            INTEGER PRIMARY KEY AUTOINCREMENT,\n    `id`             INTEGER,\n    `exchange`       TEXT           NOT NULL DEFAULT '',\n    `symbol`         TEXT           NOT NULL,\n    `price`          DECIMAL(16, 8) NOT NULL,\n    `quantity`       DECIMAL(16, 8) NOT NULL,\n    `quote_quantity` DECIMAL(16, 8) NOT NULL,\n    `fee`            DECIMAL(16, 8) NOT NULL,\n    `fee_currency`   VARCHAR(4)     NOT NULL,\n    `is_buyer`       BOOLEAN        NOT NULL DEFAULT FALSE,\n    `is_maker`       BOOLEAN        NOT NULL DEFAULT FALSE,\n    `side`           VARCHAR(4)     NOT NULL DEFAULT '',\n    `traded_at`      DATETIME(3)    NOT NULL\n);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `trades`;"},
		},
	)
}
