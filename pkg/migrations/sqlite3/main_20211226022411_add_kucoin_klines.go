package sqlite3

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/sqlite3/20211226022411_add_kucoin_klines.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20211226022411, "migrations/sqlite3/20211226022411_add_kucoin_klines.sql", false,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `kucoin_klines`\n(\n    `gid`                    INTEGER PRIMARY KEY AUTOINCREMENT,\n    `exchange`               VARCHAR(10)    NOT NULL,\n    `start_time`             DATETIME(3)    NOT NULL,\n    `end_time`               DATETIME(3)    NOT NULL,\n    `interval`               VARCHAR(3)     NOT NULL,\n    `symbol`                 VARCHAR(7)     NOT NULL,\n    `open`                   DECIMAL(16, 8) NOT NULL,\n    `high`                   DECIMAL(16, 8) NOT NULL,\n    `low`                    DECIMAL(16, 8) NOT NULL,\n    `close`                  DECIMAL(16, 8) NOT NULL DEFAULT 0.0,\n    `volume`                 DECIMAL(16, 8) NOT NULL DEFAULT 0.0,\n    `closed`                 BOOLEAN        NOT NULL DEFAULT TRUE,\n    `last_trade_id`          INT            NOT NULL DEFAULT 0,\n    `num_trades`             INT            NOT NULL DEFAULT 0,\n    `quote_volume`           DECIMAL        NOT NULL DEFAULT 0.0,\n    `taker_buy_base_volume`  DECIMAL        NOT NULL DEFAULT 0.0,\n    `taker_buy_quote_volume` DECIMAL        NOT NULL DEFAULT 0.0\n);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE kucoin_klines;"},
		},
	)
}
