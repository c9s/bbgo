package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20200721225616_trades.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20200721225616, "migrations/mysql/20200721225616_trades.sql", false,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `trades`\n(\n    `gid`            BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,\n    `id`             BIGINT UNSIGNED,\n    `order_id`       BIGINT UNSIGNED         NOT NULL,\n    `exchange`       VARCHAR(24)             NOT NULL DEFAULT '',\n    `symbol`         VARCHAR(32)             NOT NULL,\n    `price`          DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `quantity`       DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `quote_quantity` DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `fee`            DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `fee_currency`   VARCHAR(16)             NOT NULL,\n    `is_buyer`       BOOLEAN                 NOT NULL DEFAULT FALSE,\n    `is_maker`       BOOLEAN                 NOT NULL DEFAULT FALSE,\n    `side`           VARCHAR(4)              NOT NULL DEFAULT '',\n    `traded_at`      DATETIME(3)             NOT NULL,\n    `is_margin`      BOOLEAN                 NOT NULL DEFAULT FALSE,\n    `is_isolated`    BOOLEAN                 NOT NULL DEFAULT FALSE,\n    `strategy`       VARCHAR(32)             NULL,\n    `pnl`            DECIMAL                 NULL,\n    PRIMARY KEY (`gid`),\n    UNIQUE KEY `id` (`exchange`, `symbol`, `side`, `id`)\n);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX trades_symbol ON trades (exchange, symbol);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX trades_symbol_fee_currency ON trades (exchange, symbol, fee_currency, traded_at);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX trades_traded_at_symbol ON trades (exchange, traded_at, symbol);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `trades`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX trades_symbol ON trades;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX trades_symbol_fee_currency ON trades;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX trades_traded_at_symbol ON trades;"},
		},
	)
}
