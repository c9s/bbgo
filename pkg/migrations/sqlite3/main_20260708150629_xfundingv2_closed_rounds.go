package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_xfundingv2ClosedRounds, down_main_xfundingv2ClosedRounds)
}

func up_main_xfundingv2ClosedRounds(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `xfundingv2_closed_rounds`\n(\n    `gid`                     INTEGER PRIMARY KEY AUTOINCREMENT,\n    `id`                      TEXT       NOT NULL DEFAULT '',\n    `strategy_instance_id`    TEXT       NOT NULL,\n    `spot_symbol`             TEXT       NOT NULL,\n    `futures_symbol`          TEXT       NOT NULL,\n    `spot_exchange`           TEXT       NOT NULL DEFAULT '',\n    `futures_exchange`        TEXT       NOT NULL DEFAULT '',\n    `direction`               TEXT       NOT NULL DEFAULT '',\n    `collateral_asset`        TEXT       NOT NULL DEFAULT '',\n    `leverage`                REAL       NOT NULL DEFAULT 0,\n    `triggered_funding_rate`  REAL       NOT NULL DEFAULT 0,\n    `annualized_rate`         REAL       NOT NULL DEFAULT 0,\n    `funding_income`          REAL       NOT NULL DEFAULT 0,\n    `spot_pnl`                REAL       NOT NULL DEFAULT 0,\n    `spot_net_pnl`            REAL       NOT NULL DEFAULT 0,\n    `futures_pnl`             REAL       NOT NULL DEFAULT 0,\n    `futures_net_pnl`         REAL       NOT NULL DEFAULT 0,\n    `net_pnl`                 REAL       NOT NULL DEFAULT 0,\n    `num_holding_intervals`   INTEGER    NOT NULL DEFAULT 0,\n    `start_time`              DATETIME   NOT NULL,\n    `ready_time`              DATETIME   NOT NULL,\n    `closing_time`            DATETIME   NOT NULL,\n    `closed_time`             DATETIME   NOT NULL,\n    `inserted_at`             DATETIME   DEFAULT CURRENT_TIMESTAMP NOT NULL\n);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE INDEX `idx_xfundingv2_closed_rounds_instance` ON `xfundingv2_closed_rounds` (`strategy_instance_id`);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE INDEX `idx_xfundingv2_closed_rounds_round_id` ON `xfundingv2_closed_rounds` (`id`);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE TABLE `xfundingv2_funding_fees`\n(\n    `gid`        INTEGER PRIMARY KEY AUTOINCREMENT,\n    `round_id`   TEXT       NOT NULL DEFAULT '',\n    `asset`      TEXT       NOT NULL DEFAULT '',\n    `amount`     REAL       NOT NULL DEFAULT 0,\n    `txn`        INTEGER    NOT NULL DEFAULT 0,\n    `time`       DATETIME   NOT NULL\n);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE INDEX `idx_xfundingv2_funding_fees_round_id` ON `xfundingv2_funding_fees` (`round_id`);")
	if err != nil {
		return err
	}
	return err
}

func down_main_xfundingv2ClosedRounds(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `xfundingv2_funding_fees`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `xfundingv2_closed_rounds`;")
	if err != nil {
		return err
	}
	return err
}
