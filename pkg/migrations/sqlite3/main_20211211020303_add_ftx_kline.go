package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_addFtxKline, down_main_addFtxKline)
}

func up_main_addFtxKline(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `ftx_klines`\n(\n    `gid`           INTEGER PRIMARY KEY AUTOINCREMENT,\n    `exchange`      VARCHAR(10)    NOT NULL,\n    `start_time`    DATETIME(3)    NOT NULL,\n    `end_time`      DATETIME(3)    NOT NULL,\n    `interval`      VARCHAR(3)     NOT NULL,\n    `symbol`        VARCHAR(7)     NOT NULL,\n    `open`          DECIMAL(16, 8) NOT NULL,\n    `high`          DECIMAL(16, 8) NOT NULL,\n    `low`           DECIMAL(16, 8) NOT NULL,\n    `close`         DECIMAL(16, 8) NOT NULL DEFAULT 0.0,\n    `volume`        DECIMAL(16, 8) NOT NULL DEFAULT 0.0,\n    `closed`        BOOLEAN        NOT NULL DEFAULT TRUE,\n    `last_trade_id` INT            NOT NULL DEFAULT 0,\n    `num_trades`    INT            NOT NULL DEFAULT 0,\n    `quote_volume` DECIMAL NOT NULL DEFAULT 0.0,\n    `taker_buy_base_volume` DECIMAL NOT NULL DEFAULT 0.0,\n    `taker_buy_quote_volume` DECIMAL NOT NULL DEFAULT 0.0\n);")
	if err != nil {
		return err
	}
	return err
}

func down_main_addFtxKline(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE ftx_klines;")
	if err != nil {
		return err
	}
	return err
}
