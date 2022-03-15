package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upAddPositions, downAddPositions)

}

func upAddPositions(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "CREATE TABLE `positions`\n(\n    `gid`                  INTEGER PRIMARY KEY AUTOINCREMENT,\n    `strategy`             VARCHAR(32)    NOT NULL,\n    `strategy_instance_id` VARCHAR(64)    NOT NULL,\n    `symbol`               VARCHAR(20)    NOT NULL,\n    `quote_currency`       VARCHAR(10)    NOT NULL,\n    `base_currency`        VARCHAR(10)    NOT NULL,\n    -- average_cost is the position average cost\n    `average_cost`         DECIMAL(16, 8) NOT NULL,\n    `base`                 DECIMAL(16, 8) NOT NULL,\n    `quote`                DECIMAL(16, 8) NOT NULL,\n    `profit`               DECIMAL(16, 8) NULL,\n    -- trade related columns\n    `trade_id`             BIGINT         NOT NULL,\n    `side`                 VARCHAR(4)     NOT NULL, -- side of the trade\n    `exchange`             VARCHAR(12)    NOT NULL, -- exchange of the trade\n    `traded_at`            DATETIME(3)    NOT NULL\n);")
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
