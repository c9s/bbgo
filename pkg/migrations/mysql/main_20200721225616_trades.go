package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_trades, down_main_trades)

}

func up_main_trades(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `trades`\n(\n    `gid`            BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,\n    `id`             BIGINT UNSIGNED,\n    `order_id`       BIGINT UNSIGNED         NOT NULL,\n    `exchange`       VARCHAR(24)             NOT NULL DEFAULT '',\n    `symbol`         VARCHAR(20)             NOT NULL,\n    `price`          DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `quantity`       DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `quote_quantity` DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `fee`            DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `fee_currency`   VARCHAR(10)             NOT NULL,\n    `is_buyer`       BOOLEAN                 NOT NULL DEFAULT FALSE,\n    `is_maker`       BOOLEAN                 NOT NULL DEFAULT FALSE,\n    `side`           VARCHAR(4)              NOT NULL DEFAULT '',\n    `traded_at`      DATETIME(3)             NOT NULL,\n    `is_margin`      BOOLEAN                 NOT NULL DEFAULT FALSE,\n    `is_isolated`    BOOLEAN                 NOT NULL DEFAULT FALSE,\n    `strategy`       VARCHAR(32)             NULL,\n    `pnl`            DECIMAL                 NULL,\n    PRIMARY KEY (`gid`),\n    UNIQUE KEY `id` (`exchange`, `symbol`, `side`, `id`)\n);")
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
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `trades`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP INDEX trades_symbol ON trades;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP INDEX trades_symbol_fee_currency ON trades;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP INDEX trades_traded_at_symbol ON trades;")
	if err != nil {
		return err
	}
	return err
}
