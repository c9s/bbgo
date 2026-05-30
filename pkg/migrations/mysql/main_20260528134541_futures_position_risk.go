package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_futuresPositionRisk, down_main_futuresPositionRisk)
}

func up_main_futuresPositionRisk(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `futures_position_risks`\n(\n    `gid`                        BIGINT UNSIGNED    NOT NULL AUTO_INCREMENT,\n    `exchange`                   VARCHAR(24)        NOT NULL DEFAULT '',\n    `symbol`                     VARCHAR(32)        NOT NULL,\n    `position_side`              VARCHAR(10)        NOT NULL DEFAULT '',\n    `leverage`                   DECIMAL(16, 2)     NOT NULL DEFAULT 0,\n    `liquidation_price`          DECIMAL(16, 8)     NOT NULL DEFAULT 0,\n    `entry_price`                DECIMAL(16, 8)     NOT NULL DEFAULT 0,\n    `mark_price`                 DECIMAL(16, 8)     NOT NULL DEFAULT 0,\n    `break_even_price`           DECIMAL(16, 8)     NOT NULL DEFAULT 0,\n    `position_amount`            DECIMAL(16, 8)     NOT NULL DEFAULT 0,\n    `unrealized_pnl`             DECIMAL(16, 2)     NOT NULL DEFAULT 0,\n    `notional`                   DECIMAL(16, 2)     NOT NULL DEFAULT 0,\n    `initial_margin`             DECIMAL(16, 2)     NOT NULL DEFAULT 0,\n    `maint_margin`               DECIMAL(16, 2)     NOT NULL DEFAULT 0,\n    `position_initial_margin`    DECIMAL(16, 2)     NOT NULL DEFAULT 0,\n    `open_order_initial_margin`  DECIMAL(16, 2)     NOT NULL DEFAULT 0,\n    `adl`                        DECIMAL(16, 2)     NOT NULL DEFAULT 0,\n    `margin_asset`               VARCHAR(20)        NOT NULL DEFAULT '',\n    `updated_at`                  DATETIME(3)        NOT NULL,\n    PRIMARY KEY (`gid`),\n    KEY `idx_position_risks_exchange_symbol` (`exchange`, `symbol`),\n    UNIQUE KEY `futures_position_risks_exchange_symbol_side_time`  (`exchange`, `symbol`, `position_side`, `updated_at`)\n);")
	if err != nil {
		return err
	}
	return err
}

func down_main_futuresPositionRisk(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `futures_position_risks`;")
	if err != nil {
		return err
	}
	return err
}
