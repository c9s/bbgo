package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_klines, down_main_klines)
}

func up_main_klines(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `klines`\n(\n    `gid`           BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,\n    `exchange`      VARCHAR(10)             NOT NULL,\n    `start_time`    DATETIME(3)             NOT NULL,\n    `end_time`      DATETIME(3)             NOT NULL,\n    `interval`      VARCHAR(3)              NOT NULL,\n    `symbol`        VARCHAR(20)              NOT NULL,\n    `open`          DECIMAL(20, 8) UNSIGNED NOT NULL,\n    `high`          DECIMAL(20, 8) UNSIGNED NOT NULL,\n    `low`           DECIMAL(20, 8) UNSIGNED NOT NULL,\n    `close`         DECIMAL(20, 8) UNSIGNED NOT NULL DEFAULT 0.0,\n    `volume`        DECIMAL(20, 8) UNSIGNED NOT NULL DEFAULT 0.0,\n    `closed`        BOOL                    NOT NULL DEFAULT TRUE,\n    `last_trade_id` INT UNSIGNED            NOT NULL DEFAULT 0,\n    `num_trades`    INT UNSIGNED            NOT NULL DEFAULT 0,\n    PRIMARY KEY (`gid`)\n);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE INDEX `klines_end_time_symbol_interval` ON klines (`end_time`, `symbol`, `interval`);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE TABLE `okex_klines` LIKE `klines`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE TABLE `binance_klines` LIKE `klines`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE TABLE `max_klines` LIKE `klines`;")
	if err != nil {
		return err
	}
	return err
}

func down_main_klines(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP INDEX `klines_end_time_symbol_interval` ON `klines`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP TABLE `binance_klines`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP TABLE `okex_klines`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP TABLE `max_klines`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP TABLE `klines`;")
	if err != nil {
		return err
	}
	return err
}
