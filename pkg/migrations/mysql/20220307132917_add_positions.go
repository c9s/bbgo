package mysql

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upAddPositions, downAddPositions)

}

func upAddPositions(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "CREATE TABLE `positions`\n(\n    `gid`                  BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,\n    `strategy`             VARCHAR(32)             NOT NULL,\n    `strategy_instance_id` VARCHAR(64)             NOT NULL,\n    `symbol`               VARCHAR(20)              NOT NULL,\n    `quote_currency`       VARCHAR(10)             NOT NULL,\n    `base_currency`        VARCHAR(10)             NOT NULL,\n    -- average_cost is the position average cost\n    `average_cost`         DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `base`                 DECIMAL(16, 8)          NOT NULL,\n    `quote`                DECIMAL(16, 8)          NOT NULL,\n    `trade_id`             BIGINT UNSIGNED         NOT NULL,\n    `traded_at`            DATETIME(3)             NOT NULL,\n    PRIMARY KEY (`gid`),\n    UNIQUE KEY `trade_id` (`trade_id`)\n);")
	if err != nil {
		return err
	}

	return err
}

func downAddPositions(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `positions`;")
	if err != nil {
		return err
	}

	return err
}
