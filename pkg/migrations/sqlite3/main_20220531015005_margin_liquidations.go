package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_marginLiquidations, down_main_marginLiquidations)
}

func up_main_marginLiquidations(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `margin_liquidations`\n(\n    `gid`               INTEGER PRIMARY KEY AUTOINCREMENT,\n    `exchange`          VARCHAR(24)    NOT NULL DEFAULT '',\n    `symbol`            VARCHAR(24)    NOT NULL DEFAULT '',\n    `order_id`          INTEGER        NOT NULL,\n    `is_isolated`       BOOL           NOT NULL DEFAULT false,\n    `average_price`     DECIMAL(16, 8) NOT NULL,\n    `price`             DECIMAL(16, 8) NOT NULL,\n    `quantity`          DECIMAL(16, 8) NOT NULL,\n    `executed_quantity` DECIMAL(16, 8) NOT NULL,\n    `side`              VARCHAR(5)     NOT NULL DEFAULT '',\n    `time_in_force`     VARCHAR(5)     NOT NULL DEFAULT '',\n    `time`      DATETIME(3)    NOT NULL\n);")
	if err != nil {
		return err
	}
	return err
}

func down_main_marginLiquidations(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `margin_liquidations`;")
	if err != nil {
		return err
	}
	return err
}
