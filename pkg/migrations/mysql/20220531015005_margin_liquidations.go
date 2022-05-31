package mysql

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upMarginLiquidations, downMarginLiquidations)

}

func upMarginLiquidations(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "CREATE TABLE `margin_liquidations`\n(\n    `gid`               BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,\n    `exchange`          VARCHAR(24)             NOT NULL DEFAULT '',\n    `symbol`            VARCHAR(24)             NOT NULL DEFAULT '',\n    `order_id`          BIGINT UNSIGNED         NOT NULL,\n    `is_isolated`       BOOL                    NOT NULL DEFAULT false,\n    `average_price`     DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `price`             DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `quantity`          DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `executed_quantity` DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `side`              VARCHAR(5)              NOT NULL DEFAULT '',\n    `time_in_force`     VARCHAR(5)              NOT NULL DEFAULT '',\n    `time`              DATETIME(3)             NOT NULL,\n    PRIMARY KEY (`gid`),\n    UNIQUE KEY (`order_id`, `exchange`)\n);")
	if err != nil {
		return err
	}

	return err
}

func downMarginLiquidations(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `margin_liquidations`;")
	if err != nil {
		return err
	}

	return err
}
