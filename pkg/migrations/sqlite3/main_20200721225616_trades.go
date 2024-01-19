package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_trades, down_main_trades)

}

func up_main_trades(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `trades`\n(\n    `gid`            INTEGER PRIMARY KEY AUTOINCREMENT,\n    `id`             INTEGER,\n    `exchange`       TEXT           NOT NULL DEFAULT '',\n    `symbol`         TEXT           NOT NULL,\n    `price`          DECIMAL(16, 8) NOT NULL,\n    `quantity`       DECIMAL(16, 8) NOT NULL,\n    `quote_quantity` DECIMAL(16, 8) NOT NULL,\n    `fee`            DECIMAL(16, 8) NOT NULL,\n    `fee_currency`   VARCHAR(4)     NOT NULL,\n    `is_buyer`       BOOLEAN        NOT NULL DEFAULT FALSE,\n    `is_maker`       BOOLEAN        NOT NULL DEFAULT FALSE,\n    `side`           VARCHAR(4)     NOT NULL DEFAULT '',\n    `traded_at`      DATETIME(3)    NOT NULL\n);")
	if err != nil {
		return err
	}
	return err
}

func down_main_trades(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `trades`;")
	if err != nil {
		return err
	}
	return err
}
