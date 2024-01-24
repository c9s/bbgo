package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_klines, down_main_klines)
}

func up_main_klines(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `klines`\n(\n    `gid`           INTEGER PRIMARY KEY AUTOINCREMENT,\n    `exchange`      VARCHAR(10)    NOT NULL,\n    `start_time`    DATETIME(3) NOT NULL,\n    `end_time`      DATETIME(3) NOT NULL,\n    `interval`      VARCHAR(3)     NOT NULL,\n    `symbol`        VARCHAR(7)     NOT NULL,\n    `open`          DECIMAL(16, 8) NOT NULL,\n    `high`          DECIMAL(16, 8) NOT NULL,\n    `low`           DECIMAL(16, 8) NOT NULL,\n    `close`         DECIMAL(16, 8) NOT NULL DEFAULT 0.0,\n    `volume`        DECIMAL(16, 8) NOT NULL DEFAULT 0.0,\n    `closed`        BOOLEAN        NOT NULL DEFAULT TRUE,\n    `last_trade_id` INT            NOT NULL DEFAULT 0,\n    `num_trades`    INT            NOT NULL DEFAULT 0\n);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE TABLE `okex_klines`\n(\n    `gid`           INTEGER PRIMARY KEY AUTOINCREMENT,\n    `exchange`      VARCHAR(10)    NOT NULL,\n    `start_time`    DATETIME(3) NOT NULL,\n    `end_time`      DATETIME(3) NOT NULL,\n    `interval`      VARCHAR(3)     NOT NULL,\n    `symbol`        VARCHAR(7)     NOT NULL,\n    `open`          DECIMAL(16, 8) NOT NULL,\n    `high`          DECIMAL(16, 8) NOT NULL,\n    `low`           DECIMAL(16, 8) NOT NULL,\n    `close`         DECIMAL(16, 8) NOT NULL DEFAULT 0.0,\n    `volume`        DECIMAL(16, 8) NOT NULL DEFAULT 0.0,\n    `closed`        BOOLEAN        NOT NULL DEFAULT TRUE,\n    `last_trade_id` INT            NOT NULL DEFAULT 0,\n    `num_trades`    INT            NOT NULL DEFAULT 0\n);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE TABLE `binance_klines`\n(\n    `gid`           INTEGER PRIMARY KEY AUTOINCREMENT,\n    `exchange`      VARCHAR(10)    NOT NULL,\n    `start_time`    DATETIME(3) NOT NULL,\n    `end_time`      DATETIME(3) NOT NULL,\n    `interval`      VARCHAR(3)     NOT NULL,\n    `symbol`        VARCHAR(7)     NOT NULL,\n    `open`          DECIMAL(16, 8) NOT NULL,\n    `high`          DECIMAL(16, 8) NOT NULL,\n    `low`           DECIMAL(16, 8) NOT NULL,\n    `close`         DECIMAL(16, 8) NOT NULL DEFAULT 0.0,\n    `volume`        DECIMAL(16, 8) NOT NULL DEFAULT 0.0,\n    `closed`        BOOLEAN        NOT NULL DEFAULT TRUE,\n    `last_trade_id` INT            NOT NULL DEFAULT 0,\n    `num_trades`    INT            NOT NULL DEFAULT 0\n);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE TABLE `max_klines`\n(\n    `gid`           INTEGER PRIMARY KEY AUTOINCREMENT,\n    `exchange`      VARCHAR(10)    NOT NULL,\n    `start_time`    DATETIME(3) NOT NULL,\n    `end_time`      DATETIME(3) NOT NULL,\n    `interval`      VARCHAR(3)     NOT NULL,\n    `symbol`        VARCHAR(7)     NOT NULL,\n    `open`          DECIMAL(16, 8) NOT NULL,\n    `high`          DECIMAL(16, 8) NOT NULL,\n    `low`           DECIMAL(16, 8) NOT NULL,\n    `close`         DECIMAL(16, 8) NOT NULL DEFAULT 0.0,\n    `volume`        DECIMAL(16, 8) NOT NULL DEFAULT 0.0,\n    `closed`        BOOLEAN        NOT NULL DEFAULT TRUE,\n    `last_trade_id` INT            NOT NULL DEFAULT 0,\n    `num_trades`    INT            NOT NULL DEFAULT 0\n);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE INDEX `klines_end_time_symbol_interval` ON `klines` (`end_time`, `symbol`, `interval`);\nCREATE INDEX `binance_klines_end_time_symbol_interval` ON `binance_klines` (`end_time`, `symbol`, `interval`);\nCREATE INDEX `okex_klines_end_time_symbol_interval` ON `okex_klines` (`end_time`, `symbol`, `interval`);\nCREATE INDEX `max_klines_end_time_symbol_interval` ON `max_klines` (`end_time`, `symbol`, `interval`);")
	if err != nil {
		return err
	}
	return err
}

func down_main_klines(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS `klines_end_time_symbol_interval`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `binance_klines`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `okex_klines`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `max_klines`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `klines`;")
	if err != nil {
		return err
	}
	return err
}
