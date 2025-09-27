package postgres

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_trades, down_main_trades)
}

func up_main_trades(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE trades\n(\n    gid            BIGSERIAL               NOT NULL,\n    id             BIGINT,\n    order_id       BIGINT                  NOT NULL,\n    exchange       VARCHAR(24)             NOT NULL DEFAULT '',\n    symbol         VARCHAR(32)             NOT NULL,\n    price          NUMERIC(16, 8)          NOT NULL CHECK (price >= 0),\n    quantity       NUMERIC(16, 8)          NOT NULL CHECK (quantity >= 0),\n    quote_quantity NUMERIC(16, 8)          NOT NULL CHECK (quote_quantity >= 0),\n    fee            NUMERIC(16, 8)          NOT NULL CHECK (fee >= 0),\n    fee_currency   VARCHAR(16)             NOT NULL,\n    is_buyer       BOOLEAN                 NOT NULL DEFAULT FALSE,\n    is_maker       BOOLEAN                 NOT NULL DEFAULT FALSE,\n    side           VARCHAR(4)              NOT NULL DEFAULT '',\n    traded_at      TIMESTAMP(3)            NOT NULL,\n    is_margin      BOOLEAN                 NOT NULL DEFAULT FALSE,\n    is_isolated    BOOLEAN                 NOT NULL DEFAULT FALSE,\n    strategy       VARCHAR(32)             NULL,\n    pnl            NUMERIC                 NULL,\n    PRIMARY KEY (gid),\n    UNIQUE (exchange, symbol, side, id)\n);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE INDEX trades_symbol ON trades (exchange, symbol);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE INDEX trades_symbol_fee_currency ON trades (exchange, symbol, fee_currency, traded_at);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE INDEX trades_traded_at_symbol ON trades (exchange, traded_at, symbol);")
	if err != nil {
		return err
	}
	return err
}

func down_main_trades(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS trades_symbol;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS trades_symbol_fee_currency;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS trades_traded_at_symbol;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS trades;")
	if err != nil {
		return err
	}
	return err
}
