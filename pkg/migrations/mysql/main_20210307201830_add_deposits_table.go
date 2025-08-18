package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_addDepositsTable, down_main_addDepositsTable)
}

func up_main_addDepositsTable(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `deposits`\n(\n    `gid`      BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,\n    `exchange` VARCHAR(24)     NOT NULL,\n    -- asset is the asset name (currency)\n    `asset`    VARCHAR(10)     NOT NULL,\n    `address`  VARCHAR(128)     NOT NULL DEFAULT '',\n    `amount`   DECIMAL(16, 8)  NOT NULL,\n    `txn_id`   VARCHAR(256)     NOT NULL,\n    `time`     DATETIME(3)     NOT NULL,\n    PRIMARY KEY (`gid`),\n    UNIQUE KEY `txn_id` (`exchange`, `txn_id`)\n);")
	if err != nil {
		return err
	}
	return err
}

func down_main_addDepositsTable(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `deposits`;")
	if err != nil {
		return err
	}
	return err
}
