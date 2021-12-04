package migrations

import (
	"database/sql"
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	rockhopper.AddMigration(upAddWithdrawsTable, downAddWithdrawsTable)
}

func upAddWithdrawsTable(ctx context.Context, tx *sql.Tx) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "CREATE TABLE `withdraws`\n(\n    `gid`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,\n    `exchange`         VARCHAR(24)     NOT NULL DEFAULT '',\n    -- asset is the asset name (currency)\n    `asset`            VARCHAR(10)     NOT NULL,\n    `address`          VARCHAR(128)     NOT NULL,\n    `network`          VARCHAR(32)     NOT NULL DEFAULT '',\n    `amount`           DECIMAL(16, 8)  NOT NULL,\n    `txn_id`           VARCHAR(256)    NOT NULL,\n    `txn_fee`          DECIMAL(16, 8)  NOT NULL DEFAULT 0,\n    `txn_fee_currency` VARCHAR(32)     NOT NULL DEFAULT '',\n    `time`             DATETIME(3)     NOT NULL,\n    PRIMARY KEY (`gid`),\n    UNIQUE KEY `txn_id` (`exchange`, `txn_id`)\n);")
	if err != nil {
		return err
	}

	return err
}

func downAddWithdrawsTable(ctx context.Context, tx *sql.Tx) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `withdraws`;")
	if err != nil {
		return err
	}

	return err
}
