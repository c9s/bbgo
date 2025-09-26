package postgres

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_addProfitTable, down_main_addProfitTable)
}

func up_main_addProfitTable(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE profits\n(\n    gid                  BIGSERIAL               NOT NULL,\n    strategy             VARCHAR(32)             NOT NULL,\n    strategy_instance_id VARCHAR(64)             NOT NULL,\n    symbol               VARCHAR(32)             NOT NULL,\n    -- average_cost is the position average cost\n    average_cost         NUMERIC(16, 8)          NOT NULL CHECK (average_cost >= 0),\n    -- profit is the pnl (profit and loss)\n    profit               NUMERIC(16, 8)          NOT NULL,\n    -- net_profit is the pnl (profit and loss)\n    net_profit           NUMERIC(16, 8)          NOT NULL,\n    -- profit_margin is the pnl (profit and loss)\n    profit_margin        NUMERIC(16, 8)          NOT NULL,\n    -- net_profit_margin is the pnl (profit and loss)\n    net_profit_margin    NUMERIC(16, 8)          NOT NULL,\n    quote_currency       VARCHAR(10)             NOT NULL,\n    base_currency        VARCHAR(16)             NOT NULL,\n    -- -------------------------------------------------------\n    -- embedded trade data --\n    -- -------------------------------------------------------\n    exchange             VARCHAR(24)             NOT NULL DEFAULT '',\n    is_futures           BOOLEAN                 NOT NULL DEFAULT FALSE,\n    is_margin            BOOLEAN                 NOT NULL DEFAULT FALSE,\n    is_isolated          BOOLEAN                 NOT NULL DEFAULT FALSE,\n    trade_id             BIGINT                  NOT NULL,\n    -- side is the side of the trade that makes profit\n    side                 VARCHAR(4)              NOT NULL DEFAULT '',\n    is_buyer             BOOLEAN                 NOT NULL DEFAULT FALSE,\n    is_maker             BOOLEAN                 NOT NULL DEFAULT FALSE,\n    -- price is the price of the trade that makes profit\n    price                NUMERIC(16, 8)          NOT NULL CHECK (price >= 0),\n    -- quantity is the quantity of the trade that makes profit\n    quantity             NUMERIC(16, 8)          NOT NULL CHECK (quantity >= 0),\n    -- quote_quantity is the quote quantity of the trade that makes profit\n    quote_quantity       NUMERIC(16, 8)          NOT NULL CHECK (quote_quantity >= 0),\n    traded_at            TIMESTAMP(3)            NOT NULL,\n    -- fee\n    fee_in_usd           NUMERIC(16, 8),\n    fee                  NUMERIC(16, 8)          NOT NULL,\n    fee_currency         VARCHAR(16)             NOT NULL,\n    PRIMARY KEY (gid),\n    UNIQUE (trade_id)\n);")
	if err != nil {
		return err
	}
	return err
}

func down_main_addProfitTable(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS profits;")
	if err != nil {
		return err
	}
	return err
}
