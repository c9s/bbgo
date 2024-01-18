package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_orders, down_main_orders)

}

func up_main_orders(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `orders`\n(\n    `gid`               INTEGER PRIMARY KEY AUTOINCREMENT,\n    `exchange`          VARCHAR             NOT NULL DEFAULT '',\n    -- order_id is the order id returned from the exchange\n    `order_id`          INTEGER          NOT NULL,\n    `client_order_id`   VARCHAR          NOT NULL DEFAULT '',\n    `order_type`        VARCHAR          NOT NULL,\n    `symbol`            VARCHAR          NOT NULL,\n    `status`            VARCHAR          NOT NULL,\n    `time_in_force`     VARCHAR          NOT NULL,\n    `price`             DECIMAL(16, 8)  NOT NULL,\n    `stop_price`        DECIMAL(16, 8)  NOT NULL,\n    `quantity`          DECIMAL(16, 8)  NOT NULL,\n    `executed_quantity` DECIMAL(16, 8)  NOT NULL DEFAULT 0.0,\n    `side`              VARCHAR         NOT NULL DEFAULT '',\n    `is_working`        BOOLEAN         NOT NULL DEFAULT FALSE,\n    `created_at`        DATETIME(3)     NOT NULL,\n    `updated_at`        DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP\n);")
	if err != nil {
		return err
	}
	return err
}

func down_main_orders(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `orders`;")
	if err != nil {
		return err
	}
	return err
}
