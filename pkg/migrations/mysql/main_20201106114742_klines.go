package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20201106114742_klines.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20201106114742, "migrations/mysql/20201106114742_klines.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `klines`\n(\n    `gid`           BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,\n    `exchange`      VARCHAR(10)             NOT NULL,\n    `start_time`    DATETIME(3)             NOT NULL,\n    `end_time`      DATETIME(3)             NOT NULL,\n    `interval`      VARCHAR(3)              NOT NULL,\n    `symbol`        VARCHAR(20)              NOT NULL,\n    `open`          DECIMAL(20, 8) UNSIGNED NOT NULL,\n    `high`          DECIMAL(20, 8) UNSIGNED NOT NULL,\n    `low`           DECIMAL(20, 8) UNSIGNED NOT NULL,\n    `close`         DECIMAL(20, 8) UNSIGNED NOT NULL DEFAULT 0.0,\n    `volume`        DECIMAL(20, 8) UNSIGNED NOT NULL DEFAULT 0.0,\n    `closed`        BOOL                    NOT NULL DEFAULT TRUE,\n    `last_trade_id` INT UNSIGNED            NOT NULL DEFAULT 0,\n    `num_trades`    INT UNSIGNED            NOT NULL DEFAULT 0,\n    PRIMARY KEY (`gid`)\n);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX `klines_end_time_symbol_interval` ON klines (`end_time`, `symbol`, `interval`);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `okex_klines` LIKE `klines`;"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `binance_klines` LIKE `klines`;"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `max_klines` LIKE `klines`;"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX `klines_end_time_symbol_interval` ON `klines`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE `binance_klines`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE `okex_klines`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE `max_klines`;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE `klines`;"},
		},
	)
}
