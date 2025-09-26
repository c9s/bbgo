package postgres

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_klines, down_main_klines)
}

func up_main_klines(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE klines\n(\n    gid           BIGSERIAL               NOT NULL,\n    exchange      VARCHAR(10)             NOT NULL,\n    start_time    TIMESTAMP(3)            NOT NULL,\n    end_time      TIMESTAMP(3)            NOT NULL,\n    interval      VARCHAR(3)              NOT NULL,\n    symbol        VARCHAR(20)             NOT NULL,\n    open          NUMERIC(20, 8)          NOT NULL CHECK (open >= 0),\n    high          NUMERIC(20, 8)          NOT NULL CHECK (high >= 0),\n    low           NUMERIC(20, 8)          NOT NULL CHECK (low >= 0),\n    close         NUMERIC(20, 8)          NOT NULL DEFAULT 0.0 CHECK (close >= 0),\n    volume        NUMERIC(20, 8)          NOT NULL DEFAULT 0.0 CHECK (volume >= 0),\n    closed        BOOLEAN                 NOT NULL DEFAULT TRUE,\n    last_trade_id INTEGER                 NOT NULL DEFAULT 0 CHECK (last_trade_id >= 0),\n    num_trades    INTEGER                 NOT NULL DEFAULT 0 CHECK (num_trades >= 0),\n    PRIMARY KEY (gid)\n);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE INDEX klines_end_time_symbol_interval ON klines (end_time, symbol, interval);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE TABLE okex_klines\n(\n    gid           BIGSERIAL               NOT NULL,\n    exchange      VARCHAR(10)             NOT NULL,\n    start_time    TIMESTAMP(3)            NOT NULL,\n    end_time      TIMESTAMP(3)            NOT NULL,\n    interval      VARCHAR(3)              NOT NULL,\n    symbol        VARCHAR(20)             NOT NULL,\n    open          NUMERIC(20, 8)          NOT NULL CHECK (open >= 0),\n    high          NUMERIC(20, 8)          NOT NULL CHECK (high >= 0),\n    low           NUMERIC(20, 8)          NOT NULL CHECK (low >= 0),\n    close         NUMERIC(20, 8)          NOT NULL DEFAULT 0.0 CHECK (close >= 0),\n    volume        NUMERIC(20, 8)          NOT NULL DEFAULT 0.0 CHECK (volume >= 0),\n    closed        BOOLEAN                 NOT NULL DEFAULT TRUE,\n    last_trade_id INTEGER                 NOT NULL DEFAULT 0 CHECK (last_trade_id >= 0),\n    num_trades    INTEGER                 NOT NULL DEFAULT 0 CHECK (num_trades >= 0),\n    PRIMARY KEY (gid)\n);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE TABLE binance_klines\n(\n    gid           BIGSERIAL               NOT NULL,\n    exchange      VARCHAR(10)             NOT NULL,\n    start_time    TIMESTAMP(3)            NOT NULL,\n    end_time      TIMESTAMP(3)            NOT NULL,\n    interval      VARCHAR(3)              NOT NULL,\n    symbol        VARCHAR(20)             NOT NULL,\n    open          NUMERIC(20, 8)          NOT NULL CHECK (open >= 0),\n    high          NUMERIC(20, 8)          NOT NULL CHECK (high >= 0),\n    low           NUMERIC(20, 8)          NOT NULL CHECK (low >= 0),\n    close         NUMERIC(20, 8)          NOT NULL DEFAULT 0.0 CHECK (close >= 0),\n    volume        NUMERIC(20, 8)          NOT NULL DEFAULT 0.0 CHECK (volume >= 0),\n    closed        BOOLEAN                 NOT NULL DEFAULT TRUE,\n    last_trade_id INTEGER                 NOT NULL DEFAULT 0 CHECK (last_trade_id >= 0),\n    num_trades    INTEGER                 NOT NULL DEFAULT 0 CHECK (num_trades >= 0),\n    PRIMARY KEY (gid)\n);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE TABLE max_klines\n(\n    gid           BIGSERIAL               NOT NULL,\n    exchange      VARCHAR(10)             NOT NULL,\n    start_time    TIMESTAMP(3)            NOT NULL,\n    end_time      TIMESTAMP(3)            NOT NULL,\n    interval      VARCHAR(3)              NOT NULL,\n    symbol        VARCHAR(20)             NOT NULL,\n    open          NUMERIC(20, 8)          NOT NULL CHECK (open >= 0),\n    high          NUMERIC(20, 8)          NOT NULL CHECK (high >= 0),\n    low           NUMERIC(20, 8)          NOT NULL CHECK (low >= 0),\n    close         NUMERIC(20, 8)          NOT NULL DEFAULT 0.0 CHECK (close >= 0),\n    volume        NUMERIC(20, 8)          NOT NULL DEFAULT 0.0 CHECK (volume >= 0),\n    closed        BOOLEAN                 NOT NULL DEFAULT TRUE,\n    last_trade_id INTEGER                 NOT NULL DEFAULT 0 CHECK (last_trade_id >= 0),\n    num_trades    INTEGER                 NOT NULL DEFAULT 0 CHECK (num_trades >= 0),\n    PRIMARY KEY (gid)\n);")
	if err != nil {
		return err
	}
	return err
}

func down_main_klines(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS klines_end_time_symbol_interval;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS binance_klines;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS okex_klines;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS max_klines;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS klines;")
	if err != nil {
		return err
	}
	return err
}
