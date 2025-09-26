package postgres

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_addFtxKline, down_main_addFtxKline)
}

func up_main_addFtxKline(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS ftx_klines\n(\n    gid                    BIGSERIAL               NOT NULL PRIMARY KEY,\n    exchange               VARCHAR(10)             NOT NULL,\n    start_time             TIMESTAMP(3)            NOT NULL,\n    end_time               TIMESTAMP(3)            NOT NULL,\n    interval               VARCHAR(3)              NOT NULL,\n    symbol                 VARCHAR(20)             NOT NULL,\n    open                   NUMERIC(20,8)           NOT NULL CHECK (open >= 0),\n    high                   NUMERIC(20,8)           NOT NULL CHECK (high >= 0),\n    low                    NUMERIC(20,8)           NOT NULL CHECK (low >= 0),\n    close                  NUMERIC(20,8)           DEFAULT 0.00000000 NOT NULL CHECK (close >= 0),\n    volume                 NUMERIC(20,8)           DEFAULT 0.00000000 NOT NULL CHECK (volume >= 0),\n    closed                 BOOLEAN                 DEFAULT TRUE NOT NULL,\n    last_trade_id          INTEGER                 DEFAULT 0 NOT NULL CHECK (last_trade_id >= 0),\n    num_trades             INTEGER                 DEFAULT 0 NOT NULL CHECK (num_trades >= 0),\n    quote_volume           NUMERIC(32,4)           DEFAULT 0.0000 NOT NULL,\n    taker_buy_base_volume  NUMERIC(32,8)           NOT NULL,\n    taker_buy_quote_volume NUMERIC(32,4)           DEFAULT 0.0000 NOT NULL\n);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE INDEX klines_end_time_symbol_interval_ftx\n    ON ftx_klines (end_time, symbol, interval);")
	if err != nil {
		return err
	}
	return err
}

func down_main_addFtxKline(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS ftx_klines;")
	if err != nil {
		return err
	}
	return err
}
