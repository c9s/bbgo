package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_addPositions, down_main_addPositions)

}

func up_main_addPositions(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `positions`\n(\n    `gid`                  BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,\n    `strategy`             VARCHAR(32)             NOT NULL,\n    `strategy_instance_id` VARCHAR(64)             NOT NULL,\n    `symbol`               VARCHAR(20)             NOT NULL,\n    `quote_currency`       VARCHAR(10)             NOT NULL,\n    `base_currency`        VARCHAR(10)             NOT NULL,\n    -- average_cost is the position average cost\n    `average_cost`         DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `base`                 DECIMAL(16, 8)          NOT NULL,\n    `quote`                DECIMAL(16, 8)          NOT NULL,\n    `profit`               DECIMAL(16, 8)          NULL,\n    -- trade related columns\n    `trade_id`             BIGINT UNSIGNED         NOT NULL, -- the trade id in the exchange\n    `side`                 VARCHAR(4)              NOT NULL, -- side of the trade\n    `exchange`             VARCHAR(12)             NOT NULL, -- exchange of the trade\n    `traded_at`            DATETIME(3)             NOT NULL, -- millisecond timestamp\n    PRIMARY KEY (`gid`),\n    UNIQUE KEY `trade_id` (`trade_id`, `side`, `exchange`)\n);")
	if err != nil {
		return err
	}
	return err
}

func down_main_addPositions(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `positions`;")
	if err != nil {
		return err
	}
	return err
}
