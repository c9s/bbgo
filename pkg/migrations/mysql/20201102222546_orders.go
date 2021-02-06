package mysql

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	rockhopper.AddMigration(upOrders, downOrders)
}

func upOrders(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "CREATE TABLE `orders`\n(\n    `gid`               BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,\n    `exchange`          VARCHAR(24)             NOT NULL DEFAULT '',\n    -- order_id is the order id returned from the exchange\n    `order_id`          BIGINT UNSIGNED         NOT NULL,\n    `client_order_id`   VARCHAR(42)             NOT NULL DEFAULT '',\n    `order_type`        VARCHAR(16)             NOT NULL,\n    `symbol`            VARCHAR(8)              NOT NULL,\n    `status`            VARCHAR(12)             NOT NULL,\n    `time_in_force`     VARCHAR(4)              NOT NULL,\n    `price`             DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `stop_price`        DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `quantity`          DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `executed_quantity` DECIMAL(16, 8) UNSIGNED NOT NULL DEFAULT 0.0,\n    `side`              VARCHAR(4)              NOT NULL DEFAULT '',\n    `is_working`        BOOL                    NOT NULL DEFAULT FALSE,\n    `created_at`        DATETIME(3)             NOT NULL,\n    `updated_at`        DATETIME(3)             NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),\n    PRIMARY KEY (`gid`)\n);")
	if err != nil {
		return err
	}

	return err
}

func downOrders(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "DROP TABLE `orders`;")
	if err != nil {
		return err
	}

	return err
}
