package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_xfundingv2ClosedRounds, down_main_xfundingv2ClosedRounds)
}

func up_main_xfundingv2ClosedRounds(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `xfundingv2_closed_rounds`\n(\n    `gid`                     BIGINT UNSIGNED    NOT NULL AUTO_INCREMENT,\n    `id`                      VARCHAR(36)        NOT NULL DEFAULT '',\n    `strategy_instance_id`    VARCHAR(64)        NOT NULL,\n    `spot_symbol`             VARCHAR(32)        NOT NULL,\n    `futures_symbol`          VARCHAR(32)        NOT NULL,\n    `spot_exchange`           VARCHAR(24)        NOT NULL DEFAULT '',\n    `futures_exchange`        VARCHAR(24)        NOT NULL DEFAULT '',\n    `direction`               VARCHAR(10)        NOT NULL DEFAULT '',\n    `collateral_asset`        VARCHAR(20)        NOT NULL DEFAULT '',\n    `leverage`                DECIMAL(16, 2)     NOT NULL DEFAULT 0,\n    `triggered_funding_rate`  DECIMAL(20, 8)     NOT NULL DEFAULT 0,\n    `annualized_rate`         DECIMAL(20, 8)     NOT NULL DEFAULT 0,\n    `funding_income`          DECIMAL(20, 8)     NOT NULL DEFAULT 0,\n    `spot_pnl`                DECIMAL(20, 8)     NOT NULL DEFAULT 0,\n    `spot_net_pnl`            DECIMAL(20, 8)     NOT NULL DEFAULT 0,\n    `futures_pnl`             DECIMAL(20, 8)     NOT NULL DEFAULT 0,\n    `futures_net_pnl`         DECIMAL(20, 8)     NOT NULL DEFAULT 0,\n    `net_pnl`                 DECIMAL(20, 8)     NOT NULL DEFAULT 0,\n    `num_holding_intervals`   INT                NOT NULL DEFAULT 0,\n    `start_time`              DATETIME(3)        NOT NULL,\n    `ready_time`              DATETIME(3)        NOT NULL,\n    `closing_time`            DATETIME(3)        NOT NULL,\n    `closed_time`             DATETIME(3)        NOT NULL,\n    `inserted_at`             DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,\n    PRIMARY KEY (`gid`),\n    KEY `idx_xfundingv2_closed_rounds_instance` (`strategy_instance_id`),\n    KEY `idx_xfundingv2_closed_rounds_round_id` (`id`)\n);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE TABLE `xfundingv2_funding_fees`\n(\n    `gid`        BIGINT UNSIGNED    NOT NULL AUTO_INCREMENT,\n    `round_id`   VARCHAR(36)        NOT NULL DEFAULT '',\n    `asset`      VARCHAR(20)        NOT NULL DEFAULT '',\n    `amount`     DECIMAL(20, 8)     NOT NULL DEFAULT 0,\n    `txn`        BIGINT             NOT NULL DEFAULT 0,\n    `time`       DATETIME(3)        NOT NULL,\n    PRIMARY KEY (`gid`),\n    KEY `idx_xfundingv2_funding_fees_round_id` (`round_id`)\n);")
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
